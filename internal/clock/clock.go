package clock

import (
	"sync"
	"sync/atomic"
	"time"
)

// Clock is injectable so window rotation can be tested deterministically.
type Clock interface {
	Now() time.Time
}

type Real struct{}

func (Real) Now() time.Time { return time.Now() }

// Fake is a monotonically advancing test clock.
type Fake struct {
	ns atomic.Int64
}

func NewFake(t time.Time) *Fake {
	f := &Fake{}
	f.ns.Store(t.UnixNano())
	return f
}

func (f *Fake) Now() time.Time {
	return time.Unix(0, f.ns.Load())
}

func (f *Fake) Set(t time.Time) {
	f.ns.Store(t.UnixNano())
}

func (f *Fake) Advance(d time.Duration) {
	f.ns.Add(int64(d))
}

// MutexFake serializes Now with Advance for tests that mix both.
type MutexFake struct {
	mu sync.Mutex
	t  time.Time
}

func NewMutexFake(t time.Time) *MutexFake {
	return &MutexFake{t: t}
}

func (f *MutexFake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *MutexFake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}
