package reason

type Reason string

const (
	Pass            Reason = "PASS"
	RateLimited     Reason = "RATE_LIMITED"
	AdaptiveLimited Reason = "ADAPTIVE_LIMITED"
	CircuitOpen     Reason = "CIRCUIT_OPEN"
	CircuitProbe    Reason = "CIRCUIT_PROBE"
	Disabled        Reason = "DISABLED"
)

func (r Reason) Blocked() bool {
	return r == RateLimited || r == AdaptiveLimited || r == CircuitOpen
}
