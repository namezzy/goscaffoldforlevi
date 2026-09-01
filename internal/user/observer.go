package user

// Observer receives one call per business event worth counting.
//
// The interface is declared here, in the package that calls it, so that
// internal/user never imports a metrics library. ADR-0001 confines
// instrumentation to technical packages and Phase 8 audits for that import.
//
// The method name is the metrics side's, which makes *telemetry.Metrics
// satisfy this directly with no adapter in between. An adapter is a place
// where a call can be dropped, and a counter that silently stops moving is
// indistinguishable from a product with no signups.
type Observer interface {
	// UserRegistered records one created user.
	UserRegistered()
}

type nopObserver struct{}

func (nopObserver) UserRegistered() {}

// NopObserver returns an Observer that records nothing, so instrumentation
// stays optional and a nil interface can never reach the request path.
func NopObserver() Observer { return nopObserver{} }
