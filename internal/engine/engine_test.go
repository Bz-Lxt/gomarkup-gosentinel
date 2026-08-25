package engine

import (
	"sync"
	"testing"
	"time"

	"gosentinel/internal/clock"
	"gosentinel/internal/rule"
)

func TestFixedLimit(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	e := New(clk)
	r := rule.Default()
	r.Service = "svc"
	r.Resource = "api"
	r.QPS = 100
	e.SetRule(r)
	var pass, block int
	for i := 0; i < 150; i++ {
		en := e.Enter("svc", "api", "*")
		if en.Blocked {
			block++
			en.Exit(FinishFallback)
		} else {
			pass++
			en.Exit(FinishOK)
		}
	}
	if pass != 100 {
		t.Fatalf("pass=%d", pass)
	}
	if block != 50 {
		t.Fatalf("block=%d", block)
	}
	if pass+block != 150 {
		t.Fatalf("sum")
	}
}

func TestRuleHotSwap(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	e := New(clk)
	r := rule.Default()
	r.Service, r.Resource, r.QPS = "svc", "api", 100
	e.SetRule(r)
	for i := 0; i < 40; i++ {
		en := e.Enter("svc", "api", "*")
		en.Exit(FinishOK)
	}
	r.QPS = 50
	e.SetRule(r)
	var pass int
	for i := 0; i < 20; i++ {
		en := e.Enter("svc", "api", "*")
		if !en.Blocked {
			pass++
		}
		en.Exit(FinishOK)
	}
	if pass != 10 {
		t.Fatalf("after swap pass=%d want 10 (40 already used of 50)", pass)
	}
}

func TestBlockedExitDoesNotCountError(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	e := New(clk)
	r := rule.Default()
	r.Service, r.Resource, r.QPS, r.MinRequests = "svc", "api", 5, 20
	e.SetRule(r)
	for i := 0; i < 20; i++ {
		en := e.Enter("svc", "api", "*")
		if en.Blocked {
			en.Exit(FinishFallback)
			continue
		}
		en.Exit(FinishOK)
	}
	ms := e.Metrics()
	if ms[0].Snapshot.Error != 0 {
		t.Fatalf("blocked must not add errors: %+v", ms[0].Snapshot)
	}
	if ms[0].State != "CLOSED" {
		t.Fatalf("circuit should stay closed, %s", ms[0].State)
	}
}

func TestDisabledBypass(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	e := New(clk)
	r := rule.Default()
	r.Service, r.Resource, r.Enabled, r.QPS = "svc", "api", false, 1
	e.SetRule(r)
	for i := 0; i < 5; i++ {
		en := e.Enter("svc", "api", "*")
		if en.Blocked {
			t.Fatal("disabled should not block")
		}
	}
}

func TestConcurrentEnter(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	e := New(clk)
	r := rule.Default()
	r.Service, r.Resource, r.QPS = "svc", "api", 80
	e.SetRule(r)
	var wg sync.WaitGroup
	var pass, block int64
	var mu sync.Mutex
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			en := e.Enter("svc", "api", "*")
			mu.Lock()
			if en.Blocked {
				block++
			} else {
				pass++
			}
			mu.Unlock()
			en.Exit(FinishOK)
		}()
	}
	wg.Wait()
	if pass < 70 || pass > 95 {
		t.Fatalf("concurrent pass=%d block=%d", pass, block)
	}
	if pass+block != 200 {
		t.Fatalf("lost requests")
	}
}
