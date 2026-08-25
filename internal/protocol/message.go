package protocol

const SchemaVersion int64 = 1

type Envelope struct {
	SchemaVersion int64  `json:"schema_version"`
	MsgID         string `json:"msg_id"`
	Type          string `json:"type"`
	RuleVersion   int64  `json:"rule_version"`
	SentAt        string `json:"sent_at"`
	NodeID        string `json:"node_id,omitempty"`
	Service       string `json:"service,omitempty"`
	Instance      string `json:"instance,omitempty"`
	Payload       any    `json:"payload,omitempty"`
}

const (
	TypeHello          = "hello"
	TypeRulesSnapshot  = "rules_snapshot"
	TypeAck            = "ack"
	TypeNack           = "nack"
	TypeTelemetry      = "telemetry"
	TypePing           = "ping"
	TypePong           = "pong"
	TypeResetCircuit   = "reset_circuit"
	TypeDashboardTick  = "dashboard_tick"
)

type Hello struct {
	NodeID    string   `json:"node_id"`
	Service   string   `json:"service"`
	Instance  string   `json:"instance"`
	Version   int64    `json:"rule_version"`
	Caps      []string `json:"capabilities"`
}

type Ack struct {
	MsgID   string `json:"ack_of"`
	Version int64  `json:"version"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

type Telemetry struct {
	Resources []ResourcePoint `json:"resources"`
}

type ResourcePoint struct {
	Resource   string  `json:"resource"`
	Method     string  `json:"method"`
	Pass       uint64  `json:"pass"`
	Block      uint64  `json:"block"`
	Error      uint64  `json:"error"`
	Fallback   uint64  `json:"fallback"`
	Total      uint64  `json:"total"`
	QPS        float64 `json:"qps"`
	BlockRatio float64 `json:"block_ratio"`
	ErrorRatio float64 `json:"error_ratio"`
	AvgLatency float64 `json:"avg_latency_ms"`
	State      string  `json:"state"`
	Effective  int64   `json:"effective_qps"`
	Limit      int64   `json:"qps_limit"`
	Version    int64   `json:"version"`
}

type ResetCircuit struct {
	Service  string `json:"service"`
	Resource string `json:"resource"`
	Method   string `json:"method"`
}
