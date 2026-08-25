package node_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"gosentinel/internal/node"
	"gosentinel/internal/protocol"
	"gosentinel/internal/rule"
)

func TestClientRejectsInvalidRuleSnapshot(t *testing.T) {
	guard := node.NewGuard("checkout")
	current := rule.Default()
	current.Service = "checkout"
	current.Resource = "/pay"
	current.QPS = 25
	guard.ApplyRules([]rule.Snapshot{current})

	type sessionResult struct {
		envelope protocol.Envelope
		err      error
	}
	result := make(chan sessionResult, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			result <- sessionResult{err: err}
			return
		}
		defer conn.Close()

		_, raw, err := conn.ReadMessage()
		if err != nil {
			result <- sessionResult{err: err}
			return
		}
		var hello protocol.Envelope
		if err := json.Unmarshal(raw, &hello); err != nil {
			result <- sessionResult{err: err}
			return
		}
		if hello.Type != protocol.TypeHello {
			result <- sessionResult{err: fmt.Errorf("first message type = %q, want %q", hello.Type, protocol.TypeHello)}
			return
		}

		invalid := rule.Default()
		invalid.Service = "checkout"
		invalid.Resource = "/pay"
		invalid.QPS = 1_000_001
		snapshot := protocol.Envelope{
			SchemaVersion: protocol.SchemaVersion,
			MsgID:         "snapshot-2",
			Type:          protocol.TypeRulesSnapshot,
			RuleVersion:   2,
			Payload:       []rule.Snapshot{invalid},
		}
		if err := conn.WriteJSON(snapshot); err != nil {
			result <- sessionResult{err: err}
			return
		}

		_, raw, err = conn.ReadMessage()
		if err != nil {
			result <- sessionResult{err: err}
			return
		}
		var reply protocol.Envelope
		if err := json.Unmarshal(raw, &reply); err != nil {
			result <- sessionResult{err: err}
			return
		}
		result <- sessionResult{envelope: reply}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	client := &node.Client{
		URL:      "ws" + strings.TrimPrefix(server.URL, "http"),
		NodeID:   "node-1",
		Service:  "checkout",
		Instance: "checkout-1",
		Guard:    guard,
	}
	go func() {
		client.Run(ctx)
		close(done)
	}()

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.envelope.Type != protocol.TypeNack {
			t.Fatalf("snapshot reply type = %q, want %q", got.envelope.Type, protocol.TypeNack)
		}
		var ack protocol.Ack
		payload, err := json.Marshal(got.envelope.Payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(payload, &ack); err != nil {
			t.Fatal(err)
		}
		if ack.OK {
			t.Fatal("invalid snapshot was acknowledged as successful")
		}
		if ack.MsgID != "snapshot-2" {
			t.Fatalf("nack references %q, want snapshot-2", ack.MsgID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for snapshot response")
	}

	installed := guard.Engine().RuleOf("checkout", "/pay", "*")
	if installed.QPS != current.QPS {
		t.Fatalf("rejected snapshot changed qps to %d, want previous value %d", installed.QPS, current.QPS)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("client did not stop after cancellation")
	}
}
