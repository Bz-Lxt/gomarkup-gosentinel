package engine

import (
	"sync"
	"sync/atomic"
	"time"

	"gosentinel/internal/circuit"
	"gosentinel/internal/clock"
	"gosentinel/internal/limiter"
	"gosentinel/internal/reason"
	"gosentinel/internal/rule"
	"gosentinel/internal/window"
)

type Entry struct {
	Resource string
	Reason   reason.Reason
	Blocked  bool
	Start    time.Time
	res      *resource
	clock    clock.Clock
}

type resource struct {
	key      string
	win      *window.Window
	brk      *circuit.Breaker
	adapt    *limiter.Adaptive
	rule     atomic.Value // rule.Snapshot
	adaptSeq atomic.Uint64
}

type Engine struct {
	clock   clock.Clock
	mu      sync.RWMutex
	res     map[string]*resource
	onEvent func(circuit.Event)
	events  []circuit.Event
	evtMu   sync.Mutex
}

func New(c clock.Clock) *Engine {
	if c == nil {
		c = clock.Real{}
	}
	e := &Engine{clock: c, res: make(map[string]*resource)}
	e.onEvent = func(ev circuit.Event) {
		e.evtMu.Lock()
		if len(e.events) > 256 {
			e.events = e.events[len(e.events)-128:]
		}
		e.events = append(e.events, ev)
		e.evtMu.Unlock()
	}
	return e
}

func (e *Engine) DrainEvents() []circuit.Event {
	e.evtMu.Lock()
	defer e.evtMu.Unlock()
	out := e.events
	e.events = nil
	return out
}

func (e *Engine) ApplyRules(rules []rule.Snapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()
	seen := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		r.Normalize()
		key := r.ResourceKey()
		seen[key] = struct{}{}
		res := e.res[key]
		if res == nil {
			res = e.newResource(key, r)
			e.res[key] = res
			continue
		}
		prev, _ := res.rule.Load().(rule.Snapshot)
		res.rule.Store(r)
		res.adapt.ReplaceCeiling(r.QPS)
		_ = prev
	}
}

func (e *Engine) SetRule(r rule.Snapshot) {
	r.Normalize()
	e.ApplyRules([]rule.Snapshot{r})
}

func (e *Engine) newResource(key string, r rule.Snapshot) *resource {
	res := &resource{
		key:   key,
		win:   window.New(e.clock),
		adapt: limiter.NewAdaptive(r.QPS),
	}
	res.brk = circuit.New(e.clock, key, e.onEvent)
	res.rule.Store(r)
	return res
}

func (e *Engine) lookup(service, resourceName, method string) *resource {
	e.mu.RLock()
	if res, ok := e.res[rule.ResourceKey(service, resourceName, method)]; ok {
		e.mu.RUnlock()
		return res
	}
	if res, ok := e.res[rule.ResourceKey(service, resourceName, "*")]; ok {
		e.mu.RUnlock()
		return res
	}
	if res, ok := e.res[rule.ResourceKey("*", resourceName, "*")]; ok {
		e.mu.RUnlock()
		return res
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	key := rule.ResourceKey(service, resourceName, method)
	if res, ok := e.res[key]; ok {
		return res
	}
	def := rule.Default()
	def.Service = service
	def.Resource = resourceName
	def.Method = method
	res := e.newResource(key, def)
	e.res[key] = res
	return res
}

func (e *Engine) Enter(service, resourceName, method string) Entry {
	res := e.lookup(service, resourceName, method)
	r, _ := res.rule.Load().(rule.Snapshot)
	start := e.clock.Now()

	if !r.Enabled {
		res.win.AddPass()
		return Entry{Resource: res.key, Reason: reason.Disabled, Start: start, res: res, clock: e.clock}
	}

	if why := res.brk.Allow(r); why == reason.CircuitOpen {
		res.win.AddBlock()
		res.win.AddFallback()
		return Entry{Resource: res.key, Reason: why, Blocked: true, Start: start, res: res, clock: e.clock}
	} else if why == reason.CircuitProbe {
		res.win.AddPass()
		return Entry{Resource: res.key, Reason: why, Start: start, res: res, clock: e.clock}
	}

	limit := r.QPS
	reasonLimited := reason.RateLimited
	if r.Mode == rule.ModeAdaptive {
		limit = res.adapt.Effective()
		if limit <= 0 {
			limit = r.QPS
		}
		reasonLimited = reason.AdaptiveLimited
	}
	if int64(res.win.PassCount()) >= limit {
		res.win.AddBlock()
		return Entry{Resource: res.key, Reason: reasonLimited, Blocked: true, Start: start, res: res, clock: e.clock}
	}
	res.win.AddPass()
	return Entry{Resource: res.key, Reason: reason.Pass, Start: start, res: res, clock: e.clock}
}

type Finish int

const (
	FinishOK Finish = iota
	FinishError
	FinishBusiness
	FinishFallback
)

func (e *Entry) Exit(fin Finish) {
	if e == nil || e.res == nil || e.Blocked {
		return
	}
	if !e.Start.IsZero() && e.clock != nil {
		us := e.clock.Now().Sub(e.Start).Microseconds()
		if us < 0 {
			us = 0
		}
		e.res.win.AddLatency(uint64(us))
	}
	failed := fin == FinishError || fin == FinishFallback
	if fin == FinishFallback {
		e.res.win.AddFallback()
	}
	if failed {
		e.res.win.AddError()
	}
	r, _ := e.res.rule.Load().(rule.Snapshot)
	snap := e.res.win.Snapshot()
	e.res.brk.Observe(r, failed, snap.Pass, snap.Error)
	if r.Mode == rule.ModeAdaptive && e.res.adaptSeq.Add(1)%16 == 0 {
		e.res.adapt.Observe(r, snap)
	}
}

func (e *Engine) Metrics() []ResourceMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]ResourceMetrics, 0, len(e.res))
	for _, res := range e.res {
		r, _ := res.rule.Load().(rule.Snapshot)
		snap := res.win.Snapshot()
		out = append(out, ResourceMetrics{
			Key:        res.key,
			Service:    r.Service,
			Resource:   r.Resource,
			Method:     r.Method,
			State:      res.brk.State().String(),
			QPSLimit:   r.QPS,
			Effective:  res.adapt.Effective(),
			Version:    r.Version,
			Enabled:    r.Enabled,
			Mode:       string(r.Mode),
			Snapshot:   snap,
		})
	}
	return out
}

func (e *Engine) ResetCircuit(service, resourceName, method string) {
	res := e.lookup(service, resourceName, method)
	res.brk.Reset()
}

func (e *Engine) RuleOf(service, resourceName, method string) rule.Snapshot {
	res := e.lookup(service, resourceName, method)
	r, _ := res.rule.Load().(rule.Snapshot)
	return r
}

type ResourceMetrics struct {
	Key       string
	Service   string
	Resource  string
	Method    string
	State     string
	QPSLimit  int64
	Effective int64
	Version   int64
	Enabled   bool
	Mode      string
	Snapshot  window.Snapshot
}
