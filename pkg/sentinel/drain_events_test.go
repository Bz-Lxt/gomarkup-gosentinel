package sentinel_test

import (
	"testing"

	"gosentinel/pkg/sentinel"
)

func TestDrainEventsKeepsDeliveredBatchStable(t *testing.T) {
	guard := sentinel.New(sentinel.Options{Service: "checkout"})
	for i := 0; i < 20; i++ {
		token := guard.Entry("/pay", "POST")
		if token.Blocked {
			t.Fatalf("request %d unexpectedly blocked", i+1)
		}
		token.Exit(sentinel.ResultError)
	}

	opened := guard.Engine().DrainEvents()
	if len(opened) != 1 {
		t.Fatalf("opening batch has %d events, want 1", len(opened))
	}
	wantOpened := opened[0]
	if wantOpened.From != "CLOSED" || wantOpened.To != "OPEN" || wantOpened.Reason != "error_rate" {
		t.Fatalf("opening event = %+v", wantOpened)
	}

	guard.ResetCircuit("/pay", "POST")
	reset := guard.Engine().DrainEvents()
	if len(reset) != 1 || reset[0].From != "OPEN" || reset[0].To != "CLOSED" {
		t.Fatalf("reset batch = %+v", reset)
	}

	if opened[0] != wantOpened {
		t.Fatalf("delivered opening event changed after reset: got %+v, want %+v", opened[0], wantOpened)
	}
}
