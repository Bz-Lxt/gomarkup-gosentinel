package hub

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"gosentinel/internal/log"
	"gosentinel/internal/protocol"
	"gosentinel/internal/store"
	"gosentinel/internal/telemetry"
	"gosentinel/internal/timeutil"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 30 * time.Second
	pingPeriod = 20 * time.Second
	maxMsg     = 32 << 10
	sendQ      = 64
)

type Node struct {
	ID        string
	Service   string
	Instance  string
	Version   int64
	LastSeen  time.Time
	Connected bool
	send      chan []byte
	drop      atomic.Bool
}

type Hub struct {
	store   *store.FileStore
	agg     *telemetry.Aggregator
	up      websocket.Upgrader
	mu      sync.Mutex
	nodes   map[string]*Node
	dash    map[*dashClient]struct{}
	pending map[string]time.Time
	acks    map[int64]ackStat
}

type ackStat struct {
	Started time.Time
	Target  int
	Ack     int
	Nack    int
}

type dashClient struct {
	send chan []byte
}

func New(st *store.FileStore, agg *telemetry.Aggregator, origins []string) *Hub {
	return &Hub{
		store:   st,
		agg:     agg,
		up:      websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, CheckOrigin: CheckOrigin(origins)},
		nodes:   make(map[string]*Node),
		dash:    make(map[*dashClient]struct{}),
		pending: make(map[string]time.Time),
		acks:    make(map[int64]ackStat),
	}
}

func (h *Hub) ServeNodes(w http.ResponseWriter, r *http.Request) {
	conn, err := h.up.Upgrade(w, r, nil)
	if err != nil {
		log.Logger().Warn("ws upgrade failed", "err", err)
		return
	}
	go h.readNode(conn)
}

func (h *Hub) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	conn, err := h.up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	cl := &dashClient{send: make(chan []byte, sendQ)}
	h.mu.Lock()
	h.dash[cl] = struct{}{}
	h.mu.Unlock()
	go writePump(conn, cl.send)
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.dash, cl)
			h.mu.Unlock()
			conn.Close()
		}()
		conn.SetReadLimit(maxMsg)
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongWait)) })
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *Hub) readNode(conn *websocket.Conn) {
	defer conn.Close()
	conn.SetReadLimit(maxMsg)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(pongWait)) })

	var node *Node
	outDone := make(chan struct{})
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var env protocol.Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		if env.SchemaVersion != 0 && env.SchemaVersion != protocol.SchemaVersion {
			h.reply(conn, nackOf(env, "unknown_schema"))
			continue
		}
		switch env.Type {
		case protocol.TypeHello:
			var hello protocol.Hello
			if err := decodePayload(env.Payload, &hello); err != nil || hello.NodeID == "" {
				h.reply(conn, nackOf(env, "invalid_hello"))
				continue
			}
			node = h.attach(hello, conn, outDone)
			h.pushSnapshot(node)
		case protocol.TypeAck, protocol.TypeNack:
			if node != nil {
				h.onAck(env, env.Type == protocol.TypeAck)
				node.Version = env.RuleVersion
				node.LastSeen = timeutil.Now()
			}
		case protocol.TypeTelemetry:
			if node != nil {
				var tel protocol.Telemetry
				if err := decodePayload(env.Payload, &tel); err == nil {
					h.agg.Ingest(node.Service, node.Instance, tel)
				}
				node.LastSeen = timeutil.Now()
			}
		case protocol.TypePing:
			h.reply(conn, envelope(protocol.TypePong, env.RuleVersion, nil))
		default:
			h.reply(conn, nackOf(env, "unknown_type"))
		}
	}
	if node != nil {
		h.detach(node)
	}
	close(outDone)
}

func (h *Hub) attach(hello protocol.Hello, conn *websocket.Conn, done <-chan struct{}) *Node {
	n := &Node{
		ID:        hello.NodeID,
		Service:   hello.Service,
		Instance:  hello.Instance,
		Version:   hello.Version,
		LastSeen:  timeutil.Now(),
		Connected: true,
		send:      make(chan []byte, sendQ),
	}
	h.mu.Lock()
	if old, ok := h.nodes[n.ID]; ok {
		old.drop.Store(true)
		closeQuiet(old.send)
	}
	h.nodes[n.ID] = n
	h.mu.Unlock()
	go func() {
		writePump(conn, n.send)
	}()
	return n
}

func (h *Hub) detach(n *Node) {
	h.mu.Lock()
	if cur, ok := h.nodes[n.ID]; ok {
		cur.Connected = false
		cur.LastSeen = timeutil.Now()
	}
	h.mu.Unlock()
}

func (h *Hub) pushSnapshot(n *Node) {
	doc := h.store.Document()
	env := envelope(protocol.TypeRulesSnapshot, doc.Version, doc.Rules)
	raw, _ := json.Marshal(env)
	select {
	case n.send <- raw:
	default:
		n.drop.Store(true)
	}
}

func (h *Hub) BroadcastRules() {
	doc := h.store.Document()
	env := envelope(protocol.TypeRulesSnapshot, doc.Version, doc.Rules)
	raw, _ := json.Marshal(env)
	h.mu.Lock()
	target := 0
	for _, n := range h.nodes {
		if !n.Connected || n.drop.Load() {
			continue
		}
		target++
		select {
		case n.send <- raw:
		default:
			n.drop.Store(true)
		}
	}
	h.acks[doc.Version] = ackStat{Started: timeutil.Now(), Target: target}
	h.mu.Unlock()
}

func (h *Hub) BroadcastReset(service, resource, method string) {
	env := envelope(protocol.TypeResetCircuit, h.store.Version(), protocol.ResetCircuit{
		Service: service, Resource: resource, Method: method,
	})
	raw, _ := json.Marshal(env)
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, n := range h.nodes {
		if !n.Connected {
			continue
		}
		select {
		case n.send <- raw:
		default:
		}
	}
}

func (h *Hub) onAck(env protocol.Envelope, ok bool) {
	h.mu.Lock()
	st := h.acks[env.RuleVersion]
	if ok {
		st.Ack++
	} else {
		st.Nack++
	}
	h.acks[env.RuleVersion] = st
	h.mu.Unlock()
}

func (h *Hub) Convergence(version int64) map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	st := h.acks[version]
	online := 0
	stale := 0
	for _, n := range h.nodes {
		if n.Connected {
			online++
			if n.Version < version {
				stale++
			}
		}
	}
	elapsed := int64(0)
	if !st.Started.IsZero() {
		elapsed = timeutil.Now().Sub(st.Started).Milliseconds()
	}
	return map[string]any{
		"version":        version,
		"target_nodes":   st.Target,
		"ack":            st.Ack,
		"nack":           st.Nack,
		"online":         online,
		"not_converged":  stale,
		"elapsed_ms":     elapsed,
	}
}

func (h *Hub) Nodes() []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]map[string]any, 0, len(h.nodes))
	for _, n := range h.nodes {
		out = append(out, map[string]any{
			"id":        n.ID,
			"service":   n.Service,
			"instance":  n.Instance,
			"version":   n.Version,
			"connected": n.Connected,
			"last_seen": timeutil.Format(n.LastSeen),
		})
	}
	return out
}

func (h *Hub) PublishDashboard(tick any) {
	env := envelope(protocol.TypeDashboardTick, h.store.Version(), tick)
	raw, _ := json.Marshal(env)
	h.mu.Lock()
	defer h.mu.Unlock()
	for cl := range h.dash {
		select {
		case cl.send <- raw:
		default:
		}
	}
}

func (h *Hub) reply(conn *websocket.Conn, env protocol.Envelope) {
	raw, _ := json.Marshal(env)
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	_ = conn.WriteMessage(websocket.TextMessage, raw)
}

func writePump(conn *websocket.Conn, ch <-chan []byte) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()
	for {
		select {
		case msg, ok := <-ch:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func envelope(typ string, ver int64, payload any) protocol.Envelope {
	return protocol.Envelope{
		SchemaVersion: protocol.SchemaVersion,
		MsgID:         timeutil.Now().Format("150405.000000"),
		Type:          typ,
		RuleVersion:   ver,
		SentAt:        timeutil.RFC3339(timeutil.Now()),
		Payload:       payload,
	}
}

func nackOf(env protocol.Envelope, reason string) protocol.Envelope {
	out := envelope(protocol.TypeNack, env.RuleVersion, protocol.Ack{MsgID: env.MsgID, Version: env.RuleVersion, OK: false, Error: reason})
	return out
}

func decodePayload(p any, dest any) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func closeQuiet(ch chan []byte) {
	defer func() { _ = recover() }()
	close(ch)
}
