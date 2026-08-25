package limiter

import (
	"sync"
	"sync/atomic"

	"gosentinel/internal/rule"
	"gosentinel/internal/window"
)

// Adaptive applies AIMD against a configured QPS ceiling.
type Adaptive struct {
	mu         sync.Mutex
	effective  atomic.Int64
	healthyWin int
}

func NewAdaptive(ceiling int64) *Adaptive {
	a := &Adaptive{}
	if ceiling <= 0 {
		ceiling = 100
	}
	a.effective.Store(ceiling)
	return a
}

func (a *Adaptive) Effective() int64 {
	return a.effective.Load()
}

func (a *Adaptive) Observe(r rule.Snapshot, snap window.Snapshot) int64 {
	ceiling := r.QPS
	min := r.AdaptiveMinQPS
	if min < 1 {
		min = 1
	}
	if ceiling < min {
		ceiling = min
	}
	cur := a.effective.Load()
	if cur <= 0 || cur > ceiling {
		cur = ceiling
	}

	overloaded := snap.ErrorRatio() >= r.AdaptiveErrorRate || snap.AvgLatencyMs() >= float64(r.AdaptiveLatencyMs)
	a.mu.Lock()
	defer a.mu.Unlock()
	if overloaded {
		a.healthyWin = 0
		next := int64(float64(cur) * r.AdaptiveDecrease)
		if next < min {
			next = min
		}
		a.effective.Store(next)
		return next
	}
	a.healthyWin++
	if a.healthyWin >= r.AdaptiveHysteresis {
		next := cur + r.AdaptiveIncrease
		if next > ceiling {
			next = ceiling
		}
		a.effective.Store(next)
		a.healthyWin = 0
		return next
	}
	a.effective.Store(cur)
	return cur
}

func (a *Adaptive) ReplaceCeiling(qps int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cur := a.effective.Load()
	if cur <= 0 || cur > qps {
		a.effective.Store(qps)
	}
}
