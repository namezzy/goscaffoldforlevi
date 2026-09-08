package telemetry

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RouteUnmatched is the route label used for requests that matched no route.
//
// Using the raw path would let anyone mint series by hand — GET /aaaa,
// GET /aaab, and so on until the process runs out of memory — so every
// unrouted request collapses onto this one value.
const RouteUnmatched = "unmatched"

// Prometheus instruments for this application.
//
// The instrument set is deliberately small and every label is drawn from a
// bounded vocabulary. That constraint is the whole design: a Prometheus series
// lives as long as the process, so a label whose values come from the request
// is a memory leak that anyone on the internet can trigger. ADR-0001 keeps
// this package as the only one that knows about a metrics library, so business
// code records an event by calling a method here and never sees a Counter.

// PoolStats is the slice of *pgxpool.Stat the pool collector needs.
//
// It is an interface so that this package does not import pgx: the database
// driver has no business being a dependency of the metrics registry, and the
// test can supply three numbers instead of a live pool.
type PoolStats interface {
	TotalConns() int32
	IdleConns() int32
	AcquiredConns() int32
}

// Metrics owns a registry and the instruments registered in it.
//
// The registry is per-instance rather than prometheus.DefaultRegisterer,
// because duplicate registration panics: a global would turn "construct the
// application twice in one test binary" into a crash, and would let any
// imported library add series to our scrape output without asking.
type Metrics struct {
	registry *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	httpErrors   *prometheus.CounterVec

	rateLimitRejected *prometheus.CounterVec

	redisOperations *prometheus.CounterVec
	redisDuration   *prometheus.HistogramVec

	messagingPublished  *prometheus.CounterVec
	messagingConsumed   *prometheus.CounterVec
	messagingDuration   *prometheus.HistogramVec
	messagingRetries    *prometheus.CounterVec
	messagingDLQ        *prometheus.CounterVec
	messagingDuplicates *prometheus.CounterVec
	outboxPending       prometheus.Gauge

	userRegistrations prometheus.Counter
	logins            prometheus.Counter
	loginFailures     prometheus.Counter
}

// NewMetrics builds the registry and every instrument in it.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		registry: reg,

		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP requests by method, route template and status class.",
		}, []string{"method", "route", "status"}),

		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request latency by method and route template.",
			// Tuned for an API, not for a batch job: the interesting range is
			// 5ms to 5s, and the default buckets spend half their resolution
			// above one second where nothing should be.
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"method", "route"}),

		httpErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "HTTP responses that indicate a server fault (5xx).",
		}, []string{"method", "route", "status"}),

		rateLimitRejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "rate_limit_rejected_total",
			Help: "Requests rejected by a rate limiter, by scope.",
		}, []string{"scope"}),

		redisOperations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "redis_operation_total",
			Help: "Redis commands by command name and outcome.",
		}, []string{"command", "outcome"}),

		redisDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "redis_operation_duration_seconds",
			Help:    "Redis command latency by command name.",
			Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.05, 0.1, 0.5},
		}, []string{"command"}),

		messagingPublished: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messaging_published_total",
			Help: "Events published to the broker by the outbox relay.",
		}, []string{"event_type"}),

		messagingConsumed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messaging_consumed_total",
			Help: "Events processed by a consumer, by outcome.",
		}, []string{"event_type", "consumer", "outcome"}),

		messagingDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "messaging_consume_duration_seconds",
			Help:    "Handler latency by event type and consumer.",
			Buckets: []float64{0.005, 0.025, 0.1, 0.5, 1, 5, 15, 30},
		}, []string{"event_type", "consumer"}),

		messagingRetries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messaging_retry_total",
			Help: "Handler attempts that failed and will be retried.",
		}, []string{"event_type", "consumer"}),

		messagingDLQ: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messaging_dlq_total",
			Help: "Events moved to the dead letter table.",
		}, []string{"event_type", "consumer"}),

		messagingDuplicates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "messaging_duplicate_total",
			Help: "Redeliveries this consumer had already handled.",
		}, []string{"event_type", "consumer"}),

		outboxPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "outbox_pending_events",
			Help: "Outbox rows not yet delivered to the broker.",
		}),

		userRegistrations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "user_registration_total",
			Help: "Users created.",
		}),

		logins: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "login_total",
			Help: "Login attempts, successful or not.",
		}),

		loginFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "login_failure_total",
			Help: "Login attempts that were rejected.",
		}),
	}

	reg.MustRegister(
		m.httpRequests, m.httpDuration, m.httpErrors,
		m.rateLimitRejected,
		m.redisOperations, m.redisDuration,
		m.messagingPublished, m.messagingConsumed, m.messagingDuration,
		m.messagingRetries, m.messagingDLQ, m.messagingDuplicates, m.outboxPending,
		m.userRegistrations, m.logins, m.loginFailures,
	)

	// Goroutine count and heap size are what a Go service dies of, and neither
	// is visible from request metrics: a leaked goroutine per request looks
	// perfectly healthy until the process is killed.
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return m
}

// Registry exposes the gatherer for the scrape handler and for tests.
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// Handler serves the Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		// A scrape that fails should say so in the response rather than in a
		// log nobody reads; Prometheus marks the target down and the gap is
		// visible on the graph.
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
}

// RegisterPoolCollector adds db_pool_* gauges backed by the live pool.
//
// It is separate from NewMetrics because the pool is built later and because
// processes without a database — cmd/adminctl, most tests — must still get a
// working registry rather than a nil dereference.
func (m *Metrics) RegisterPoolCollector(stats PoolStats) {
	m.registry.MustRegister(
		gaugeFunc("db_pool_total_conns",
			"Connections currently held by the pool, idle or in use.",
			func() float64 { return float64(stats.TotalConns()) }),
		gaugeFunc("db_pool_idle_conns",
			"Connections sitting idle in the pool.",
			func() float64 { return float64(stats.IdleConns()) }),
		gaugeFunc("db_pool_acquired_conns",
			"Connections checked out by callers.",
			func() float64 { return float64(stats.AcquiredConns()) }),
	)
}

func gaugeFunc(name, help string, f func() float64) prometheus.Collector {
	return prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: name, Help: help}, f)
}

// HTTPRequest records one served request.
//
// route must be the chi route template. Passing a concrete path here is the
// unbounded-cardinality bug this package exists to prevent, so an empty route
// is replaced rather than trusted.
func (m *Metrics) HTTPRequest(method, route string, status int, seconds float64) {
	if route == "" {
		route = RouteUnmatched
	}
	class := statusClass(status)

	m.httpRequests.WithLabelValues(method, route, class).Inc()
	m.httpDuration.WithLabelValues(method, route).Observe(seconds)

	// Only server faults. A 404 or a 401 is the API working as designed, and
	// an error rate that rises whenever a client mistypes a URL is an alert
	// people learn to silence.
	if status >= 500 {
		m.httpErrors.WithLabelValues(method, route, class).Inc()
	}
}

// statusClass reduces a status code to its class.
//
// The exact code is not a label because the set is only bounded by convention:
// handlers invent codes and proxies pass odd ones through, so the label set
// would be widened from outside. Every query anyone writes is about the class.
func statusClass(status int) string {
	if status < 100 || status > 599 {
		return "unknown"
	}
	return strconv.Itoa(status/100) + "xx"
}

// RateLimitRejected records one rejected request. scope comes from a closed
// set defined in the ratelimit package.
func (m *Metrics) RateLimitRejected(scope string) {
	m.rateLimitRejected.WithLabelValues(scope).Inc()
}

// RedisOperation records one command. Only the command name is labelled: keys
// hold user identifiers and values hold tokens, so neither may be recorded.
func (m *Metrics) RedisOperation(command string, seconds float64, err error) {
	m.redisOperations.WithLabelValues(command, outcome(err)).Inc()
	m.redisDuration.WithLabelValues(command).Observe(seconds)
}

// MessagingPublished records one event handed to the broker.
func (m *Metrics) MessagingPublished(eventType string) {
	m.messagingPublished.WithLabelValues(eventType).Inc()
}

// MessagingConsumed records one completed handler attempt.
func (m *Metrics) MessagingConsumed(eventType, consumer string, seconds float64, err error) {
	m.messagingConsumed.WithLabelValues(eventType, consumer, outcome(err)).Inc()
	m.messagingDuration.WithLabelValues(eventType, consumer).Observe(seconds)
}

// MessagingRetry records an attempt that failed and will be tried again.
func (m *Metrics) MessagingRetry(eventType, consumer string) {
	m.messagingRetries.WithLabelValues(eventType, consumer).Inc()
}

// MessagingDeadLettered records an event that exhausted its attempts.
func (m *Metrics) MessagingDeadLettered(eventType, consumer string) {
	m.messagingDLQ.WithLabelValues(eventType, consumer).Inc()
}

// MessagingDuplicate records a redelivery that was already handled.
//
// This is at-least-once working as designed, so it is kept out of the consumed
// counter: folded in, a broker redelivering everything twice would read as
// twice the throughput instead of as a broker to look at.
func (m *Metrics) MessagingDuplicate(eventType, consumer string) {
	m.messagingDuplicates.WithLabelValues(eventType, consumer).Inc()
}

// MessagingOutboxPending records the current outbox depth.
func (m *Metrics) MessagingOutboxPending(n int64) {
	m.outboxPending.Set(float64(n))
}

// UserRegistered records one created user.
func (m *Metrics) UserRegistered() { m.userRegistrations.Inc() }

// LoginAttempt records one login attempt.
//
// A failure increments both counters. If login_total counted only successes,
// "failure rate" would be failures over successes, which reads below 100%
// during an outage in which every single attempt fails — exactly when the
// number needs to be unambiguous.
func (m *Metrics) LoginAttempt(ok bool) {
	m.logins.Inc()
	if !ok {
		m.loginFailures.Inc()
	}
}

func outcome(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
