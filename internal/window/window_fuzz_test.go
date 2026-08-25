package window

import (
	"testing"
	"time"

	"gosentinel/internal/clock"
)

func FuzzAddAndRotate(f *testing.F) {
	f.Add(uint8(1), uint16(0))
	f.Add(uint8(3), uint16(100))
	f.Fuzz(func(t *testing.T, kind uint8, advanceMs uint16) {
		clk := clock.NewFake(time.Unix(1_700_000_000, 0))
		w := New(clk)
		switch kind % 4 {
		case 0:
			w.AddPass()
		case 1:
			w.AddBlock()
		case 2:
			w.AddError()
		default:
			w.AddFallback()
		}
		clk.Advance(time.Duration(advanceMs) * time.Millisecond)
		snap := w.Snapshot()
		if snap.Pass+snap.Block > snap.Total+1 {
			t.Fatalf("inconsistent %+v", snap)
		}
	})
}
