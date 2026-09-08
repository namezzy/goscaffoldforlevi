package auth

// Observer receives one call per authentication event worth counting.
//
// Declared here rather than in the telemetry package so that internal/auth
// never imports a metrics library; ADR-0001 confines instrumentation to
// technical packages and Phase 8 audits for that import.
type Observer interface {
	// LoginAttempt records one completed login attempt and whether it
	// succeeded.
	//
	// Both outcomes go through one call on purpose. If only successes were
	// counted, the natural expression of a failure rate would be failures over
	// successes, and during the incident that matters most — every login
	// failing — the denominator goes to zero and the alert never fires. With
	// attempts as the denominator, total failure reads as exactly 100%.
	LoginAttempt(ok bool)
}

type nopObserver struct{}

func (nopObserver) LoginAttempt(bool) {}

// NopObserver returns an Observer that records nothing, so instrumentation
// stays optional and a nil interface can never reach the request path.
func NopObserver() Observer { return nopObserver{} }
