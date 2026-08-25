package node

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"gosentinel/internal/log"
	"gosentinel/internal/protocol"
	"gosentinel/internal/rule"
	"gosentinel/internal/timeutil"
	"gosentinel/pkg/sentinel"
)

type Client struct {
	URL      string
	NodeID   string
	Service  string
	Instance string
	Guard    *sentinel.Guard
	version  atomic.Int64
}

func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if err := c.session(ctx); err != nil && ctx.Err() == nil {
			log.Logger().Warn("control plane session ended", "err", err)
		}
		d := backoff + time.Duration(rand.Int63n(int64(backoff/2)+1))
		if backoff < 30*time.Second {
			backoff *= 2
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(d):
		}
	}
}

func (c *Client) session(ctx context.Context) error {
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second, Proxy: http.ProxyFromEnvironment}
	conn, _, err := dialer.DialContext(ctx, c.URL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := c.write(conn, protocol.TypeHello, protocol.Hello{
		NodeID: c.NodeID, Service: c.Service, Instance: c.Instance,
		Version: c.version.Load(), Caps: []string{"gin", "grpc", "telemetry"},
	}); err != nil {
		return err
	}
	tel := time.NewTicker(time.Second)
	defer tel.Stop()
	errCh := make(chan error, 1)
	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			c.handle(conn, raw)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case <-tel.C:
			_ = c.write(conn, protocol.TypeTelemetry, c.snapshot())
		}
	}
}

func (c *Client) handle(conn *websocket.Conn, raw []byte) {
	var env protocol.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return
	}
	if env.SchemaVersion != 0 && env.SchemaVersion != protocol.SchemaVersion {
		_ = c.write(conn, protocol.TypeNack, protocol.Ack{MsgID: env.MsgID, OK: false, Error: "unknown_schema"})
		return
	}
	switch env.Type {
	case protocol.TypeRulesSnapshot:
		var rules []rule.Snapshot
		b, _ := json.Marshal(env.Payload)
		if err := json.Unmarshal(b, &rules); err != nil {
			_ = c.write(conn, protocol.TypeNack, protocol.Ack{MsgID: env.MsgID, OK: false, Error: err.Error()})
			return
		}
		var validationErr error
		for i := range rules {
			rules[i].Normalize()
			validationErr = rule.Validate(rules[i])
			if validationErr != nil {
				break
			}
		}
		if validationErr != nil {
			_ = c.write(conn, protocol.TypeNack, protocol.Ack{MsgID: env.MsgID, Version: env.RuleVersion, OK: false, Error: validationErr.Error()})
			return
		}
		if env.RuleVersion < c.version.Load() {
			_ = c.write(conn, protocol.TypeNack, protocol.Ack{MsgID: env.MsgID, Version: env.RuleVersion, OK: false, Error: "stale_version"})
			return
		}
		c.Guard.ApplyRules(rules)
		c.version.Store(env.RuleVersion)
		_ = c.write(conn, protocol.TypeAck, protocol.Ack{MsgID: env.MsgID, Version: env.RuleVersion, OK: true})
	case protocol.TypeResetCircuit:
		var rc protocol.ResetCircuit
		b, _ := json.Marshal(env.Payload)
		_ = json.Unmarshal(b, &rc)
		c.Guard.ResetCircuit(rc.Resource, rc.Method)
		_ = c.write(conn, protocol.TypeAck, protocol.Ack{MsgID: env.MsgID, Version: env.RuleVersion, OK: true})
	case protocol.TypePong, protocol.TypePing:
	}
}

func (c *Client) snapshot() protocol.Telemetry {
	ms := c.Guard.Engine().Metrics()
	out := protocol.Telemetry{Resources: make([]protocol.ResourcePoint, 0, len(ms))}
	for _, m := range ms {
		out.Resources = append(out.Resources, protocol.ResourcePoint{
			Resource: m.Resource, Method: m.Method,
			Pass: m.Snapshot.Pass, Block: m.Snapshot.Block, Error: m.Snapshot.Error,
			Fallback: m.Snapshot.Fallback, Total: m.Snapshot.Total,
			QPS: m.Snapshot.QPS(), BlockRatio: m.Snapshot.BlockRatio(), ErrorRatio: m.Snapshot.ErrorRatio(),
			AvgLatency: m.Snapshot.AvgLatencyMs(), State: m.State, Effective: m.Effective,
			Limit: m.QPSLimit, Version: m.Version,
		})
	}
	return out
}

func (c *Client) write(conn *websocket.Conn, typ string, payload any) error {
	env := protocol.Envelope{
		SchemaVersion: protocol.SchemaVersion,
		MsgID:         timeutil.Now().Format("150405.000000"),
		Type:          typ,
		RuleVersion:   c.version.Load(),
		SentAt:        timeutil.RFC3339(timeutil.Now()),
		NodeID:        c.NodeID,
		Service:       c.Service,
		Instance:      c.Instance,
		Payload:       payload,
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, raw)
}

func (c *Client) ApplyLocal(rules []rule.Snapshot) { c.Guard.ApplyRules(rules) }

func NewGuard(service string) *sentinel.Guard {
	return sentinel.New(sentinel.Options{Service: service})
}
