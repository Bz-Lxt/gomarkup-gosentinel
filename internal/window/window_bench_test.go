package window

import (
	"testing"
	"time"

	"gosentinel/internal/clock"
)

func BenchmarkAddPass(b *testing.B) {
	w := New(clock.Real{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.AddPass()
	}
}

func BenchmarkSnapshot(b *testing.B) {
	clk := clock.NewFake(time.Unix(1_700_000_000, 0))
	w := New(clk)
	for i := 0; i < 100; i++ {
		w.AddPass()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.Snapshot()
	}
}
