// Package config defines the application configuration, loads it from the
// environment, and validates it before anything else starts.
//
// Configuration is grouped by component so that each constructor receives only
// the section it needs. Passing the whole Config around would be a quieter form
// of global state. See docs/architecture.md section 2.
package config

import (
	"log/slog"
	"net"
	"strconv"
	"time"
)

// Environment names the deployment context. It is validated on load.
type Environment string

// The recognised deployment environments. Anything else is rejected on load,
// because a typo such as "prod" would silently disable the production checks.
const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// IsProduction reports whether stricter defaults and checks apply.
func (e Environment) IsProduction() bool { return e == EnvProduction }

// Secret is a string that must never reach a log, a response, or a stack dump.
//
// It implements the interfaces used by fmt, encoding/json and log/slog so that
// redaction is the default behaviour rather than something each call site has
// to remember. Use Reveal only where the plaintext is genuinely needed.
type Secret string

func (s Secret) String() string { return "[REDACTED]" }

// GoString redacts the value under the %#v verb, which is what most debug
// dumps and struct printers reach for.
func (s Secret) GoString() string { return `"[REDACTED]"` }

// LogValue satisfies slog.LogValuer so structured logs redact automatically.
func (s Secret) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

// MarshalText covers encoding/json and anything else using TextMarshaler.
func (s Secret) MarshalText() ([]byte, error) { return []byte("[REDACTED]"), nil }

// Reveal returns the plaintext. Every call site is a place to audit.
func (s Secret) Reveal() string { return string(s) }

// IsEmpty reports whether the secret carries no value.
func (s Secret) IsEmpty() bool { return s == "" }

// Config is the root configuration.
type Config struct {
	App       App
	HTTP      HTTP
	Database  Database
	Redis     Redis
	Auth      Auth
	RateLimit RateLimit
	Messaging Messaging
	OTel      OTel
	Metrics   Metrics
}

// Messaging configures the outbox relay, the broker and the consumer.
type Messaging struct {
	// Enabled controls delivery, not recording.
	//
	// Off — the default — means events are discarded rather than written, so a
	// service that needs no asynchronous work runs no relay, needs no broker,
	// and deploys no worker. That default is the reason including messaging in
	// the scaffold costs nothing to the projects that do not want it
	// (ADR-0009).
	Enabled bool

	// NATSURLs are the brokers the worker publishes to and consumes from.
	NATSURLs []string
	// StreamName and SubjectPrefix namespace this service inside a shared
	// cluster.
	StreamName    string
	SubjectPrefix string
	// ConsumerName is the durable consumer name. It appears in the dedupe log,
	// so changing it makes every past event look unprocessed.
	ConsumerName string

	// ConnectTimeout and PublishTimeout bound broker calls.
	ConnectTimeout time.Duration
	PublishTimeout time.Duration
	// StreamMaxAge is how long the broker keeps a delivered message.
	StreamMaxAge time.Duration
	// AckWait must exceed the consumer's whole retry budget, or the broker
	// redelivers a message that is still being worked on.
	AckWait time.Duration
	// MaxDeliver is the broker-side redelivery cap, a backstop behind the
	// consumer's own retry limit.
	MaxDeliver int

	// RelayPollInterval is the floor on end-to-end latency: an event waits at
	// most this long before the relay looks for it.
	RelayPollInterval time.Duration
	RelayBatchSize    int32

	// HandlerTimeout bounds one attempt at one event.
	HandlerTimeout time.Duration
	// MaxAttempts is how many times an event is tried before it is parked.
	MaxAttempts int
	// BaseBackoff and MaxBackoff shape the full-jitter retry schedule.
	BaseBackoff time.Duration
	MaxBackoff  time.Duration

	// PublishedRetention and ProcessedRetention are how long delivered events
	// and dedupe records are kept. ProcessedRetention must stay longer than
	// any possible redelivery window: prune it too early and an old message
	// becomes processable again.
	PublishedRetention time.Duration
	ProcessedRetention time.Duration
	PruneInterval      time.Duration
}

// App holds process-level identity used by logs, traces and metrics.
type App struct {
	Name        string
	Environment Environment
	Version     string

	// LogLevel is the minimum level emitted. It is a runtime setting because
	// raising verbosity during an incident must not require a rebuild.
	LogLevel slog.Level
}

// HTTP configures the public server and the middleware that guards it.
type HTTP struct {
	Host string
	Port int

	// Server-level timeouts. ReadHeaderTimeout is the one that actually
	// protects against Slowloris; the others bound total request time.
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int

	// RequestTimeout bounds handler execution and is propagated through the
	// request context, so database and cache calls inherit the deadline.
	RequestTimeout time.Duration

	// MaxBodyBytes caps request bodies before they are read into memory.
	MaxBodyBytes int64

	// ShutdownTimeout is the total budget for graceful shutdown. Kubernetes
	// terminationGracePeriodSeconds must be larger than this value.
	ShutdownTimeout time.Duration

	// ReadinessDrainDelay is how long /ready reports failure before the server
	// stops accepting connections, giving load balancers time to notice.
	ReadinessDrainDelay time.Duration

	CORSAllowedOrigins []string

	// TrustedProxyCount is the number of reverse proxies in front of the
	// service. Zero means X-Forwarded-For is ignored entirely, which is the
	// only safe default: the header is client-controlled.
	TrustedProxyCount int
}

// Database configures the PostgreSQL connection pool.
type Database struct {
	URL Secret

	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
}

// Redis configures the Redis client.
type Redis struct {
	Addr     string
	Password Secret
	DB       int

	// Required decides whether a failed ping aborts startup. It also decides
	// whether Redis participates in the readiness probe.
	Required bool

	DialTimeout time.Duration
	ReadTimeout time.Duration
	PoolSize    int
}

// Auth configures token issuance. Consumed from Phase 3 onwards; validated
// from Phase 2 so that a misconfigured deployment fails at startup rather than
// at the first login.
type Auth struct {
	JWTSecret  Secret
	Issuer     string
	Audience   string
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	// Argon2 cost parameters. They are configurable because the right values
	// depend on the hardware the service runs on, and because raising them is
	// how the deployment responds to hardware getting cheaper. Lowering them
	// below the floors enforced in Validate is refused rather than warned
	// about: a weakened password hash is invisible until it matters.
	Argon2Time    uint32
	Argon2Memory  uint32 // KiB
	Argon2Threads uint8
}

// RateLimit configures the token buckets described in ADR-0008.
//
// Each scope is expressed as "Quota requests per Period, with a bucket depth of
// Burst" so that an operator can read the policy without knowing what a token
// bucket is.
type RateLimit struct {
	// Enabled turns limiting off entirely. It exists for local development and
	// for load testing; production must leave it on, because the login path
	// depends on it to bound Argon2 memory use.
	Enabled bool

	IPQuota  int
	IPPeriod time.Duration
	IPBurst  int

	UserQuota  int
	UserPeriod time.Duration
	UserBurst  int

	LoginIPQuota  int
	LoginIPPeriod time.Duration
	LoginIPBurst  int

	LoginIDQuota  int
	LoginIDPeriod time.Duration
	LoginIDBurst  int
}

// OTel configures telemetry export. Instrumentation is always compiled in;
// Enabled only controls whether spans and metrics leave the process.
type OTel struct {
	Enabled  bool
	Endpoint string

	// Protocol selects the OTLP transport: "grpc" or "http".
	//
	// gRPC is the default because the Collector's gRPC receiver is the one
	// every distribution enables out of the box, and because the endpoint form
	// (host:port, no scheme) is harder to get subtly wrong than the HTTP one,
	// where omitting /v1/traces produces a 404 that looks like a network fault.
	Protocol string

	// Insecure disables TLS to the collector. True is right for a collector
	// sharing a pod or a compose network and wrong across any other boundary,
	// so it defaults to false and Validate warns when production sets it.
	Insecure bool

	SamplingRatio float64
}

// OTLP protocol values for OTel.Protocol.
const (
	OTLPGRPC = "grpc"
	OTLPHTTP = "http"
)

// Metrics configures the Prometheus scrape endpoint.
//
// It is a separate listener rather than a route on the API router, because
// the two have opposite exposure requirements: the API is meant to be reachable
// from the internet and /metrics is meant not to be. Sharing a port makes that
// distinction a matter of ingress rules written somewhere else, which is how
// metrics endpoints end up public.
type Metrics struct {
	Enabled bool
	Addr    string
	Path    string
}

// Addr returns the listen address for the HTTP server.
func (h HTTP) Addr() string {
	return net.JoinHostPort(h.Host, strconv.Itoa(h.Port))
}

// retryBudget is the longest a consumer can legitimately spend on one event:
// every attempt running to its timeout, plus the worst case of every backoff.
func (m Messaging) retryBudget() time.Duration {
	budget := time.Duration(m.MaxAttempts) * m.HandlerTimeout
	ceiling := m.BaseBackoff
	for i := 1; i < m.MaxAttempts; i++ {
		if ceiling > m.MaxBackoff {
			ceiling = m.MaxBackoff
		}
		budget += ceiling
		ceiling *= 2
	}
	return budget
}
