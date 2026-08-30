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
	App      App
	HTTP     HTTP
	Database Database
	Redis    Redis
	Auth     Auth
	OTel     OTel
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
}

// OTel configures telemetry export. Instrumentation is always compiled in;
// Enabled only controls whether spans and metrics leave the process.
type OTel struct {
	Enabled       bool
	Endpoint      string
	SamplingRatio float64
}

// Addr returns the listen address for the HTTP server.
func (h HTTP) Addr() string {
	return net.JoinHostPort(h.Host, strconv.Itoa(h.Port))
}
