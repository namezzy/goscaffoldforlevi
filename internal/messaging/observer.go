package messaging

// Observer receives one call per lifecycle event of a message.
//
// The interface lives here, in the package that calls it, so that
// internal/messaging never imports a metrics library. ADR-0001 confines
// instrumentation to technical packages and Phase 8 audits for exactly that
// import; a domain package that depends on Prometheus cannot be tested,
// vendored, or reused without dragging a registry along.
//
// The method names carry a Messaging prefix that reads redundantly from inside
// this package. That is deliberate: it makes *telemetry.Metrics satisfy this
// interface directly, with no adapter type in between. An adapter is a place
// where a label can be transposed or an argument dropped, and nothing about
// the resulting wrong number would fail a test.
//
// Every method must be safe to call concurrently and must not block. A
// consumer runs these on the message path, so an Observer that takes a
// contended lock slows down delivery, and one that blocks stops it.
type Observer interface {
	// MessagingPublished records one event handed to the broker.
	MessagingPublished(eventType string)

	// MessagingConsumed records the terminal outcome of one event, exactly
	// once per Process call, with the time spent across all its attempts.
	MessagingConsumed(eventType, consumer string, seconds float64, err error)

	// MessagingRetry records an attempt that failed and will be tried again.
	MessagingRetry(eventType, consumer string)

	// MessagingDeadLettered records an event parked after exhausting its
	// attempts or failing permanently.
	MessagingDeadLettered(eventType, consumer string)

	// MessagingDuplicate records a redelivery this consumer had already
	// handled. This is the normal, safe outcome of at-least-once delivery, and
	// it is counted apart from real work: folded into the success counter, a
	// broker redelivering everything twice would read as twice the throughput
	// rather than as a broker to look at.
	MessagingDuplicate(eventType, consumer string)

	// MessagingOutboxPending reports the number of undelivered outbox rows
	// after each relay sweep.
	//
	// This is the only signal that separates "nothing is happening" from
	// "nothing is getting out": a stalled relay publishes nothing, so every
	// rate in the system reads zero, exactly as it does on a quiet Sunday.
	MessagingOutboxPending(n int64)
}

// nopObserver discards everything, so that instrumentation stays optional.
//
// Observability is off by default per §5.1, and a nil interface field on the
// message path is a panic in the one place where a panic costs every message
// rather than one request.
type nopObserver struct{}

func (nopObserver) MessagingPublished(string)                        {}
func (nopObserver) MessagingConsumed(string, string, float64, error) {}
func (nopObserver) MessagingRetry(string, string)                    {}
func (nopObserver) MessagingDeadLettered(string, string)             {}
func (nopObserver) MessagingDuplicate(string, string)                {}
func (nopObserver) MessagingOutboxPending(int64)                     {}

// NopObserver returns an Observer that records nothing.
func NopObserver() Observer { return nopObserver{} }
