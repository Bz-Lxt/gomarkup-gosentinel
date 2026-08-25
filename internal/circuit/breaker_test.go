package circuit

import (
	"testing"
	"time"

	"gosentinel/internal/clock"
	"gosentinel/internal/reason"
	"gosentinel/internal/rule"
)

func TestOpensOnceAboveErrorRate(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	var events []Event
	b := New(clk, "r", func(e Event) { events = append(events, e) })
	r := rule.Default()
	r.MinRequests = 20
	r.ErrorRate = 0.5
	for i := 0; i < 20; i++ {
		if b.Allow(r) != reason.Pass {
			t.Fatal("closed should pass")
		}
	}
	b.Observe(r, true, 20, 10)
	if b.State() != Closed {
		t.Fatalf("equal 50%% must stay closed, got %s", b.State())
	}
	b.Observe(r, true, 20, 11)
	if b.State() != Open {
		t.Fatalf("got %s", b.State())
	}
	openEvents := 0
	for _, e := range events {
		if e.To == "OPEN" {
			openEvents++
		}
	}
	if openEvents != 1 {
		t.Fatalf("open events=%d", openEvents)
	}
	if b.Allow(r) != reason.CircuitOpen {
		t.Fatal("open should block")
	}
}

func TestHalfOpenRecoverAndFail(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	b := New(clk, "r", nil)
	r := rule.Default()
	r.HalfOpenProbes = 3
	r.OpenTimeout = 5 * time.Second
	r.MinRequests = 1
	r.ErrorRate = 0.5
	b.Observe(r, true, 4, 3)
	if b.State() != Open {
		t.Fatalf("want open %s", b.State())
	}
	clk.Advance(5 * time.Second)
	var probes int
	for i := 0; i < 10; i++ {
		if b.Allow(r) == reason.CircuitProbe {
			probes++
		}
	}
	if probes != 3 {
		t.Fatalf("probes=%d", probes)
	}
	b.Observe(r, false, 1, 0)
	b.Observe(r, false, 1, 0)
	b.Observe(r, false, 1, 0)
	if b.State() != Closed {
		t.Fatalf("want closed after probes, %s", b.State())
	}

	b2 := New(clk, "r2", nil)
	b2.Observe(r, true, 4, 3)
	clk.Advance(5 * time.Second)
	if b2.Allow(r) != reason.CircuitProbe {
		t.Fatal("probe")
	}
	b2.Observe(r, true, 1, 1)
	if b2.State() != Open {
		t.Fatalf("fail probe should reopen, %s", b2.State())
	}
}

func TestResetKeepsExplicit(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	b := New(clk, "r", nil)
	r := rule.Default()
	r.MinRequests = 1
	b.Observe(r, true, 4, 3)
	b.Reset()
	if b.State() != Closed {
		t.Fatalf("%s", b.State())
	}
}
