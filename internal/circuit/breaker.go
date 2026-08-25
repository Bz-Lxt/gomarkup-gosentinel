package circuit

import (
	"sync"
	"sync/atomic"
	"time"

	"gosentinel/internal/clock"
	"gosentinel/internal/reason"
	"gosentinel/internal/rule"
)

type State uint32

const (
	Closed State = iota
	Open
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Open:
		return "OPEN"
	case HalfOpen:
		return "HALF_OPEN"
	default:
		return "CLOSED"
	}
}

type Event struct {
	Resource string
	From     string
	To       string
	Reason   string
	At       time.Time
}

type Breaker struct {
	clock        clock.Clock
	state        atomic.Uint32
	openedAtNS   atomic.Int64
	probes       atomic.Int32
	probeFails   atomic.Int32
	probeSuccess atomic.Int32
	mu           sync.Mutex
	onEvent      func(Event)
	resource     string
}

func New(c clock.Clock, resource string, onEvent func(Event)) *Breaker {
	if c == nil {
		c = clock.Real{}
	}
	return &Breaker{clock: c, resource: resource, onEvent: onEvent}
}

func (b *Breaker) State() State {
	return State(b.state.Load())
}

func (b *Breaker) Allow(r rule.Snapshot) reason.Reason {
	switch b.State() {
	case Closed:
		return reason.Pass
	case Open:
		if b.clock.Now().UnixNano()-b.openedAtNS.Load() >= int64(r.OpenTimeout) {
			b.transition(Open, HalfOpen, "open_timeout")
		}
		if b.State() == Open {
			return reason.CircuitOpen
		}
		fallthrough
	case HalfOpen:
		n := b.probes.Add(1)
		if n > r.HalfOpenProbes {
			b.probes.Add(-1)
			return reason.CircuitOpen
		}
		return reason.CircuitProbe
	default:
		return reason.Pass
	}
}

func (b *Breaker) Observe(r rule.Snapshot, failed bool, completed, errors uint64) {
	switch b.State() {
	case HalfOpen:
		if failed {
			b.probeFails.Add(1)
			b.transition(HalfOpen, Open, "probe_failed")
			return
		}
		ok := b.probeSuccess.Add(1)
		if ok >= r.HalfOpenProbes {
			b.transition(HalfOpen, Closed, "probes_succeeded")
		}
	case Closed:
		if completed >= uint64(r.MinRequests) && float64(errors) > float64(completed)*r.ErrorRate {
			b.transition(Closed, Open, "error_rate")
		}
	}
}

func (b *Breaker) Reset() {
	b.transition(b.State(), Closed, "explicit_reset")
}

func (b *Breaker) transition(from, to State, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	cur := State(b.state.Load())
	if reason != "explicit_reset" && cur != from {
		return
	}
	b.state.Store(uint32(to))
	if to == Open {
		b.openedAtNS.Store(b.clock.Now().UnixNano())
	}
	if to != HalfOpen {
		b.probes.Store(0)
		b.probeFails.Store(0)
		b.probeSuccess.Store(0)
	} else {
		b.probes.Store(0)
		b.probeFails.Store(0)
		b.probeSuccess.Store(0)
	}
	if b.onEvent != nil {
		b.onEvent(Event{
			Resource: b.resource,
			From:     from.String(),
			To:       to.String(),
			Reason:   reason,
			At:       b.clock.Now(),
		})
	}
}
