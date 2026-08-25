package rule

import (
	"strings"
	"time"
)

const SchemaVersion = 1

type Mode string

const (
	ModeFixed    Mode = "fixed"
	ModeAdaptive Mode = "adaptive"
)

type Snapshot struct {
	ID                 string        `json:"id"`
	Service            string        `json:"service"`
	Resource           string        `json:"resource"`
	Method             string        `json:"method"`
	Enabled            bool          `json:"enabled"`
	Mode               Mode          `json:"mode"`
	QPS                int64         `json:"qps"`
	AdaptiveMinQPS     int64         `json:"adaptive_min_qps"`
	AdaptiveDecrease   float64       `json:"adaptive_decrease"`
	AdaptiveIncrease   int64         `json:"adaptive_increase"`
	AdaptiveLatencyMs  int64         `json:"adaptive_latency_ms"`
	AdaptiveErrorRate  float64       `json:"adaptive_error_rate"`
	AdaptiveHysteresis int           `json:"adaptive_hysteresis"`
	ErrorRate          float64       `json:"error_rate"`
	MinRequests        int64         `json:"min_requests"`
	OpenTimeout        time.Duration `json:"open_timeout_ns"`
	OpenTimeoutMs      int64         `json:"open_timeout_ms"`
	HalfOpenProbes     int32         `json:"half_open_probes"`
	Fallback           string        `json:"fallback"`
	Version            int64         `json:"version"`
	UpdatedAt          string        `json:"updated_at"`
}

func (s Snapshot) ResourceKey() string {
	return ResourceKey(s.Service, s.Resource, s.Method)
}

func ResourceKey(service, resource, method string) string {
	return strings.Join([]string{
		strings.TrimSpace(service),
		strings.TrimSpace(resource),
		strings.TrimSpace(method),
	}, "|")
}

func Default() Snapshot {
	return Snapshot{
		Enabled:            true,
		Mode:               ModeFixed,
		QPS:                100,
		AdaptiveMinQPS:     10,
		AdaptiveDecrease:   0.7,
		AdaptiveIncrease:   5,
		AdaptiveLatencyMs:  200,
		AdaptiveErrorRate:  0.3,
		AdaptiveHysteresis: 3,
		ErrorRate:          0.5,
		MinRequests:        20,
		OpenTimeout:        5 * time.Second,
		OpenTimeoutMs:      5000,
		HalfOpenProbes:     3,
		Fallback:           "default",
	}
}

func (s *Snapshot) Normalize() {
	if s.Mode == "" {
		s.Mode = ModeFixed
	}
	if s.QPS <= 0 {
		s.QPS = 100
	}
	if s.AdaptiveMinQPS <= 0 {
		s.AdaptiveMinQPS = 1
	}
	if s.AdaptiveDecrease <= 0 || s.AdaptiveDecrease >= 1 {
		s.AdaptiveDecrease = 0.7
	}
	if s.AdaptiveIncrease <= 0 {
		s.AdaptiveIncrease = 5
	}
	if s.AdaptiveLatencyMs <= 0 {
		s.AdaptiveLatencyMs = 200
	}
	if s.AdaptiveErrorRate <= 0 || s.AdaptiveErrorRate >= 1 {
		s.AdaptiveErrorRate = 0.3
	}
	if s.AdaptiveHysteresis <= 0 {
		s.AdaptiveHysteresis = 3
	}
	if s.ErrorRate <= 0 || s.ErrorRate >= 1 {
		s.ErrorRate = 0.5
	}
	if s.MinRequests <= 0 {
		s.MinRequests = 20
	}
	if s.OpenTimeoutMs <= 0 {
		s.OpenTimeoutMs = 5000
	}
	s.OpenTimeout = time.Duration(s.OpenTimeoutMs) * time.Millisecond
	if s.HalfOpenProbes <= 0 {
		s.HalfOpenProbes = 3
	}
	if s.Method == "" {
		s.Method = "*"
	}
	if s.Fallback == "" {
		s.Fallback = "default"
	}
}

func Match(pattern, value string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	return pattern == value
}
