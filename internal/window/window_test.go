package window

import (
	"sync"
	"testing"
	"time"

	"gosentinel/internal/clock"
)

func TestSnapshotIgnoresExpiredBuckets(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	w := New(clk)
	w.AddPass()
	if got := w.Snapshot().Pass; got != 1 {
		t.Fatalf("pass=%d", got)
	}
	clk.Advance(1100 * time.Millisecond)
	if got := w.Snapshot().Pass; got != 0 {
		t.Fatalf("expired pass=%d", got)
	}
}

func TestBucketRotationKeepsCurrent(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	w := New(clk)
	for i := 0; i < 10; i++ {
		w.AddPass()
		clk.Advance(100 * time.Millisecond)
	}
	w.AddPass()
	snap := w.Snapshot()
	if snap.Pass < 9 || snap.Pass > 11 {
		t.Fatalf("window pass=%d want ~10", snap.Pass)
	}
}

func TestLargeJumpClears(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	w := New(clk)
	w.AddPass()
	w.AddBlock()
	clk.Advance(30 * time.Second)
	if snap := w.Snapshot(); snap.Pass != 0 || snap.Block != 0 {
		t.Fatalf("jump leftover %+v", snap)
	}
}

func TestConcurrentAdds(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	w := New(clk)
	var wg sync.WaitGroup
	const n = 200
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			w.AddPass()
		}()
	}
	wg.Wait()
	if got := w.Snapshot().Pass; got != n {
		t.Fatalf("pass=%d want %d", got, n)
	}
}

func TestRatios(t *testing.T) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	w := New(clk)
	for i := 0; i < 8; i++ {
		w.AddPass()
	}
	for i := 0; i < 2; i++ {
		w.AddBlock()
	}
	w.AddError()
	snap := w.Snapshot()
	if snap.BlockRatio() < 0.19 || snap.BlockRatio() > 0.21 {
		t.Fatalf("block ratio %f", snap.BlockRatio())
	}
	if snap.ErrorRatio() < 0.12 || snap.ErrorRatio() > 0.13 {
		t.Fatalf("error ratio %f", snap.ErrorRatio())
	}
}
