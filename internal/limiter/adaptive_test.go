package limiter

import (
	"testing"

	"gosentinel/internal/rule"
	"gosentinel/internal/window"
)

func TestAIMDBounds(t *testing.T) {
	r := rule.Default()
	r.QPS = 100
	r.AdaptiveMinQPS = 10
	r.AdaptiveDecrease = 0.5
	r.AdaptiveIncrease = 10
	r.AdaptiveHysteresis = 2
	r.AdaptiveErrorRate = 0.3
	r.AdaptiveLatencyMs = 50
	a := NewAdaptive(100)
	over := window.Snapshot{Pass: 20, Error: 10, Latency: 20 * 80_000}
	n := a.Observe(r, over)
	if n > 50 || n < 10 {
		t.Fatalf("decrease %d", n)
	}
	healthy := window.Snapshot{Pass: 20, Error: 0, Latency: 20 * 1_000}
	_ = a.Observe(r, healthy)
	up := a.Observe(r, healthy)
	if up <= n {
		t.Fatalf("should increase %d -> %d", n, up)
	}
	if up > 100 {
		t.Fatalf("above ceiling %d", up)
	}
}
