package api_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"gosentinel/internal/api"
	"gosentinel/internal/hub"
	"gosentinel/internal/protocol"
	"gosentinel/internal/store"
	"gosentinel/internal/telemetry"
)

func TestNodeReconnectKeepsReplacementOnline(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	agg := telemetry.New()
	h := hub.New(st, agg, nil)
	handler := (&api.Server{Store: st, Hub: h, Agg: agg}).Routes()

	oldConn := openNodeConnection(t, handler, "worker-7", "old-process")
	defer oldConn.Close()
	newConn := openNodeConnection(t, handler, "worker-7", "replacement-process")
	defer newConn.Close()

	_ = oldConn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := oldConn.ReadMessage(); err == nil {
		t.Fatal("superseded node connection remained open")
	}

	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		node := nodeStatus(t, handler, "worker-7")
		if node.Instance != "replacement-process" {
			t.Fatalf("node list reports instance %q, want replacement-process", node.Instance)
		}
		if !node.Connected {
			t.Fatal("node list marked the active replacement connection offline")
		}
		time.Sleep(time.Millisecond)
	}
}

func openNodeConnection(t *testing.T, handler http.Handler, nodeID, instance string) *websocket.Conn {
	t.Helper()
	clientSide, serverSide := net.Pipe()
	listener := &singleConnListener{conn: serverSide, closed: make(chan struct{})}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
	})

	u := &url.URL{Scheme: "ws", Host: "control.local", Path: "/ws/nodes"}
	conn, _, err := websocket.NewClient(clientSide, u, nil, 1024, 1024)
	if err != nil {
		t.Fatalf("open node websocket: %v", err)
	}
	if err := conn.WriteJSON(protocol.Envelope{
		SchemaVersion: protocol.SchemaVersion,
		Type:          protocol.TypeHello,
		Payload: protocol.Hello{
			NodeID: nodeID, Service: "checkout", Instance: instance,
		},
	}); err != nil {
		t.Fatalf("send node hello: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var snapshot protocol.Envelope
	if err := conn.ReadJSON(&snapshot); err != nil {
		t.Fatalf("read initial rules snapshot: %v", err)
	}
	if snapshot.Type != protocol.TypeRulesSnapshot {
		t.Fatalf("initial message type %q, want rules snapshot", snapshot.Type)
	}
	return conn
}

func nodeStatus(t *testing.T, handler http.Handler, nodeID string) struct {
	ID        string `json:"id"`
	Instance  string `json:"instance"`
	Connected bool   `json:"connected"`
} {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list nodes returned %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data []struct {
			ID        string `json:"id"`
			Instance  string `json:"instance"`
			Connected bool   `json:"connected"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode node list: %v", err)
	}
	for _, node := range response.Data {
		if node.ID == nodeID {
			return node
		}
	}
	t.Fatalf("node %q missing from list", nodeID)
	return struct {
		ID        string `json:"id"`
		Instance  string `json:"instance"`
		Connected bool   `json:"connected"`
	}{}
}

type singleConnListener struct {
	mu       sync.Mutex
	conn     net.Conn
	accepted bool
	closed   chan struct{}
	once     sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if !l.accepted {
		l.accepted = true
		conn := l.conn
		l.mu.Unlock()
		return conn, nil
	}
	l.mu.Unlock()
	<-l.closed
	return nil, net.ErrClosed
}

func (l *singleConnListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *singleConnListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "control.local" }

var _ net.Listener = (*singleConnListener)(nil)
var _ net.Addr = pipeAddr{}
