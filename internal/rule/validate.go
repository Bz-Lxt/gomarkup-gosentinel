package rule

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e FieldError) Error() string {
	return e.Field + ": " + e.Message
}

type ValidationError struct {
	Details []FieldError
}

func (e *ValidationError) Error() string {
	return "validation_error"
}

func Validate(s Snapshot) error {
	var result *ValidationError
	add := func(field, code, msg string) {
		if result == nil {
			result = &ValidationError{}
		}
		result.Details = append(result.Details, FieldError{Field: field, Code: code, Message: msg})
	}
	if strings.TrimSpace(s.Service) == "" {
		add("service", "required", "service is required")
	}
	if strings.TrimSpace(s.Resource) == "" {
		add("resource", "required", "resource is required")
	}
	if utf8.RuneCountInString(s.Service) > 64 {
		add("service", "max_length", "service must be ≤ 64 characters")
	}
	if utf8.RuneCountInString(s.Resource) > 128 {
		add("resource", "max_length", "resource must be ≤ 128 characters")
	}
	if s.Mode != ModeFixed && s.Mode != ModeAdaptive && s.Mode != "" {
		add("mode", "enum", "mode must be fixed or adaptive")
	}
	if s.QPS < 1 || s.QPS > 1_000_000 {
		add("qps", "out_of_range", "qps must be between 1 and 1000000")
	}
	if s.AdaptiveMinQPS < 1 || s.AdaptiveMinQPS > s.QPS && s.QPS > 0 {
		add("adaptive_min_qps", "out_of_range", "adaptive_min_qps must be in [1, qps]")
	}
	if s.AdaptiveDecrease <= 0 || s.AdaptiveDecrease >= 1 {
		add("adaptive_decrease", "out_of_range", "adaptive_decrease must be in (0, 1)")
	}
	if s.AdaptiveIncrease < 1 || s.AdaptiveIncrease > 10000 {
		add("adaptive_increase", "out_of_range", "adaptive_increase must be in [1, 10000]")
	}
	if s.AdaptiveLatencyMs < 1 || s.AdaptiveLatencyMs > 60_000 {
		add("adaptive_latency_ms", "out_of_range", "adaptive_latency_ms must be in [1, 60000]")
	}
	if s.AdaptiveErrorRate <= 0 || s.AdaptiveErrorRate >= 1 {
		add("adaptive_error_rate", "out_of_range", "adaptive_error_rate must be in (0, 1)")
	}
	if s.AdaptiveHysteresis < 1 || s.AdaptiveHysteresis > 60 {
		add("adaptive_hysteresis", "out_of_range", "adaptive_hysteresis must be in [1, 60]")
	}
	if s.ErrorRate <= 0 || s.ErrorRate >= 1 {
		add("error_rate", "out_of_range", "error_rate must be in (0, 1)")
	}
	if s.MinRequests < 1 || s.MinRequests > 100000 {
		add("min_requests", "out_of_range", "min_requests must be in [1, 100000]")
	}
	if s.OpenTimeoutMs < 100 || s.OpenTimeoutMs > 600_000 {
		add("open_timeout_ms", "out_of_range", "open_timeout_ms must be in [100, 600000]")
	}
	if s.HalfOpenProbes < 1 || s.HalfOpenProbes > 32 {
		add("half_open_probes", "out_of_range", "half_open_probes must be in [1, 32]")
	}
	if s.Fallback != "" && utf8.RuneCountInString(s.Fallback) > 64 {
		add("fallback", "max_length", "fallback must be ≤ 64 characters")
	}
	return result
}

func ValidateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id is required")
	}
	if utf8.RuneCountInString(id) > 64 {
		return fmt.Errorf("id too long")
	}
	return nil
}
