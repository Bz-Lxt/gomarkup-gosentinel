package sentinel

import "gosentinel/internal/reason"

// Reason is a stable decision code for middleware, telemetry and fallbacks.
type Reason = reason.Reason

const (
	ReasonPass            = reason.Pass
	ReasonRateLimited     = reason.RateLimited
	ReasonAdaptiveLimited = reason.AdaptiveLimited
	ReasonCircuitOpen     = reason.CircuitOpen
	ReasonCircuitProbe    = reason.CircuitProbe
	ReasonDisabled        = reason.Disabled
)

// Result classifies how a completed request finished.
type Result string

const (
	ResultOK       Result = "OK"
	ResultError    Result = "ERROR"
	ResultBusiness Result = "BUSINESS"
	ResultFallback Result = "FALLBACK"
)
