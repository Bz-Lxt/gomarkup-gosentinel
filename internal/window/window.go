package window

import (
	"sync"
	"sync/atomic"

	"gosentinel/internal/clock"
)

const (
	BucketCount = 10
	BucketSpan  = 100 // milliseconds
	WindowSpan  = BucketCount * BucketSpan
)

type Snapshot struct {
	Total    uint64
	Pass     uint64
	Block    uint64
	Error    uint64
	Fallback uint64
	Latency  uint64 // sum of microseconds
}

func (s Snapshot) QPS() float64 {
	return float64(s.Pass+s.Block) // 1s window ≈ count
}

func (s Snapshot) ErrorRatio() float64 {
	completed := s.Pass
	if completed == 0 {
		return 0
	}
	return float64(s.Error) / float64(completed)
}

func (s Snapshot) BlockRatio() float64 {
	total := s.Pass + s.Block
	if total == 0 {
		return 0
	}
	return float64(s.Block) / float64(total)
}

func (s Snapshot) AvgLatencyMs() float64 {
	if s.Pass == 0 {
		return 0
	}
	return float64(s.Latency) / float64(s.Pass) / 1000
}

type bucket struct {
	epoch    atomic.Uint64
	total    atomic.Uint64
	pass     atomic.Uint64
	block    atomic.Uint64
	errorN   atomic.Uint64
	fallback atomic.Uint64
	latency  atomic.Uint64
}

func (b *bucket) reset(slot uint64) {
	b.total.Store(0)
	b.pass.Store(0)
	b.block.Store(0)
	b.errorN.Store(0)
	b.fallback.Store(0)
	b.latency.Store(0)
	b.epoch.Store(slot)
}

// Window keeps the last 1s as 10 atomic 100ms buckets.
type Window struct {
	clock    clock.Clock
	buckets  [BucketCount]bucket
	rotateMu [BucketCount]sync.Mutex
}

func New(c clock.Clock) *Window {
	if c == nil {
		c = clock.Real{}
	}
	return &Window{clock: c}
}

func (w *Window) slot() uint64 {
	ms := w.clock.Now().UnixMilli()
	if ms < 0 {
		ms = 0
	}
	return uint64(ms) / BucketSpan
}

func (w *Window) ready(slot uint64) *bucket {
	idx := int(slot % BucketCount)
	b := &w.buckets[idx]
	if b.epoch.Load() == slot {
		return b
	}
	w.rotateMu[idx].Lock()
	defer w.rotateMu[idx].Unlock()
	if b.epoch.Load() == slot {
		return b
	}
	b.reset(slot)
	return b
}

func (w *Window) AddTotal() {
	w.ready(w.slot()).total.Add(1)
}

func (w *Window) AddPass() {
	slot := w.slot()
	b := w.ready(slot)
	b.total.Add(1)
	b.pass.Add(1)
}

func (w *Window) AddBlock() {
	slot := w.slot()
	b := w.ready(slot)
	b.total.Add(1)
	b.block.Add(1)
}

func (w *Window) AddError() {
	w.ready(w.slot()).errorN.Add(1)
}

func (w *Window) AddFallback() {
	w.ready(w.slot()).fallback.Add(1)
}

func (w *Window) AddLatency(us uint64) {
	w.ready(w.slot()).latency.Add(us)
}

func (w *Window) Snapshot() Snapshot {
	now := w.slot()
	var s Snapshot
	for i := 0; i < BucketCount; i++ {
		b := &w.buckets[i]
		ep := b.epoch.Load()
		if ep == 0 || now < ep || now-ep >= BucketCount {
			continue
		}
		s.Total += b.total.Load()
		s.Pass += b.pass.Load()
		s.Block += b.block.Load()
		s.Error += b.errorN.Load()
		s.Fallback += b.fallback.Load()
		s.Latency += b.latency.Load()
	}
	return s
}

func (w *Window) PassCount() uint64 {
	return w.Snapshot().Pass
}
