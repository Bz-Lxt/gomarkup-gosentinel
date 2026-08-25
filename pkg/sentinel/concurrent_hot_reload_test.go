package sentinel_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"gosentinel/internal/rule"
	"gosentinel/pkg/sentinel"
)

func TestProtectAppliesReloadedCircuitPolicyToInflightRequests(t *testing.T) {
	initial := rule.Default()
	initial.Service = "checkout"
	initial.Resource = "charge"
	initial.Method = "POST"
	initial.ErrorRate = 0.9
	initial.MinRequests = 2

	guard := sentinel.New(sentinel.Options{Service: "checkout", Rules: []rule.Snapshot{initial}})
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	done := make(chan struct{}, 2)
	backendErr := errors.New("backend unavailable")

	run := func(result error) {
		defer func() { done <- struct{}{} }()
		_, _ = guard.Protect(context.Background(), "charge", "POST", func() error {
			started <- struct{}{}
			<-release
			return result
		})
	}
	go run(nil)
	go run(backendErr)

	<-started
	<-started
	updated := initial
	updated.ErrorRate = 0.25
	updated.Version++
	guard.ApplyRules([]rule.Snapshot{updated})
	close(release)
	<-done
	<-done

	var called atomic.Bool
	gotReason, gotErr := guard.Protect(context.Background(), "charge", "POST", func() error {
		called.Store(true)
		return nil
	})
	if gotReason != sentinel.ReasonCircuitOpen {
		t.Fatalf("reason = %q, want %q", gotReason, sentinel.ReasonCircuitOpen)
	}
	if called.Load() {
		t.Fatal("request reached the protected function after the reloaded circuit policy should have opened")
	}
	var blocked sentinel.ErrBlocked
	if !errors.As(gotErr, &blocked) {
		t.Fatalf("error = %v, want sentinel.ErrBlocked", gotErr)
	}
}
