package sentinel

import (
	"context"
	"sync"

	"gosentinel/internal/clock"
	"gosentinel/internal/engine"
	"gosentinel/internal/rule"
)

// Guard is the public embeddable protector.
type Guard struct {
	Service string
	eng     *engine.Engine
	mu      sync.RWMutex
	fall    map[string]Fallback
}

type Fallback func(ctx context.Context, resource string, reason Reason) error

type Options struct {
	Service string
	Clock   clock.Clock
	Rules   []rule.Snapshot
}

func New(opts Options) *Guard {
	g := &Guard{
		Service: opts.Service,
		eng:     engine.New(opts.Clock),
		fall:    make(map[string]Fallback),
	}
	if len(opts.Rules) > 0 {
		g.eng.ApplyRules(opts.Rules)
	}
	return g
}

func (g *Guard) Engine() *engine.Engine { return g.eng }

func (g *Guard) RegisterFallback(resource string, fb Fallback) {
	g.mu.Lock()
	g.fall[resource] = fb
	g.mu.Unlock()
}

func (g *Guard) fallback(resource string) Fallback {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if fb, ok := g.fall[resource]; ok {
		return fb
	}
	return g.fall["*"]
}

type Token struct {
	Reason  Reason
	Blocked bool
	entry   engine.Entry
	guard   *Guard
	res     string
}

func (g *Guard) Entry(resource, method string) *Token {
	en := g.eng.Enter(g.Service, resource, method)
	return &Token{Reason: en.Reason, Blocked: en.Blocked, entry: en, guard: g, res: resource}
}

func (t *Token) Exit(result Result) {
	if t == nil {
		return
	}
	fin := engine.FinishOK
	switch result {
	case ResultError:
		fin = engine.FinishError
	case ResultBusiness:
		fin = engine.FinishBusiness
	case ResultFallback:
		fin = engine.FinishFallback
	}
	t.entry.Exit(fin)
}

func (g *Guard) Protect(ctx context.Context, resource, method string, fn func() error) (Reason, error) {
	tok := g.Entry(resource, method)
	if tok.Blocked {
		if fb := g.fallback(resource); fb != nil {
			err := fb(ctx, resource, tok.Reason)
			tok.Exit(ResultFallback)
			return tok.Reason, err
		}
		tok.Exit(ResultFallback)
		return tok.Reason, ErrBlocked{Reason: tok.Reason, Resource: resource}
	}
	err := fn()
	if err != nil {
		tok.Exit(ResultError)
		return tok.Reason, err
	}
	tok.Exit(ResultOK)
	return tok.Reason, nil
}

func (g *Guard) ApplyRules(rules []rule.Snapshot) { g.eng.ApplyRules(rules) }

func (g *Guard) ResetCircuit(resource, method string) {
	g.eng.ResetCircuit(g.Service, resource, method)
}

type ErrBlocked struct {
	Reason   Reason
	Resource string
}

func (e ErrBlocked) Error() string {
	return string(e.Reason) + ":" + e.Resource
}
