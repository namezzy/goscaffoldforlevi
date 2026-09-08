package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/namezzy/goscaffoldforlevi/internal/buildinfo"
)

// InsecureJWTSecrets are the placeholder values shipped in .env.example.
// Refusing them in production is not paranoia: deploying with the example
// secret is a real and recurring incident, and the deployment pipeline is not
// a reliable place to catch it.
var InsecureJWTSecrets = []string{
	"change-me-in-production-0000000000",
	"secret",
	"changeme",
	"development-only-do-not-use-in-prod",
}

// MinJWTSecretLen is the minimum accepted HS256 key length in bytes.
// HMAC-SHA256 keys shorter than the 32-byte output add no security.
const MinJWTSecretLen = 32

// Load reads configuration from the environment, applying .env as a fallback
// for values that are not already set.
//
// Precedence is: real environment variables > .env file > defaults. godotenv
// does not overwrite existing variables, which gives exactly that ordering.
// A missing .env is not an error; production is not expected to have one.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		// A malformed .env is worth reporting; a missing one is not.
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return Config{}, fmt.Errorf("load .env: %w", err)
		}
	}

	var p parser

	cfg := Config{
		App: App{
			Name:        p.str("APP_NAME", "goscaffoldforlevi"),
			Environment: Environment(p.str("APP_ENV", string(EnvDevelopment))),
			// The build stamp is the default, not a hardcoded string: an image built
			// by CI reports its real version without anyone setting an environment
			// variable, while APP_VERSION stays available to override it (compose
			// uses that). Commit and build time have no environment counterpart —
			// they can only be true if the linker set them.
			Version:  p.str("APP_VERSION", buildinfo.Version),
			LogLevel: p.logLevel("LOG_LEVEL", slog.LevelInfo),
		},
		HTTP: HTTP{
			Host:                p.str("HTTP_HOST", "0.0.0.0"),
			Port:                p.int("HTTP_PORT", 8080),
			ReadHeaderTimeout:   p.dur("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:         p.dur("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:        p.dur("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:         p.dur("HTTP_IDLE_TIMEOUT", 120*time.Second),
			MaxHeaderBytes:      p.int("HTTP_MAX_HEADER_BYTES", 1<<20),
			RequestTimeout:      p.dur("HTTP_REQUEST_TIMEOUT", 20*time.Second),
			MaxBodyBytes:        int64(p.int("HTTP_MAX_BODY_BYTES", 1<<20)),
			ShutdownTimeout:     p.dur("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
			ReadinessDrainDelay: p.dur("HTTP_READINESS_DRAIN_DELAY", 5*time.Second),
			CORSAllowedOrigins:  p.csv("HTTP_CORS_ALLOWED_ORIGINS", nil),
			TrustedProxyCount:   p.int("HTTP_TRUSTED_PROXY_COUNT", 0),
		},
		Database: Database{
			URL:               p.secret("DATABASE_URL", ""),
			MaxConns:          p.int32("DATABASE_MAX_CONNS", 10),
			MinConns:          p.int32("DATABASE_MIN_CONNS", 2),
			MaxConnLifetime:   p.dur("DATABASE_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:   p.dur("DATABASE_MAX_CONN_IDLE_TIME", 30*time.Minute),
			HealthCheckPeriod: p.dur("DATABASE_HEALTH_CHECK_PERIOD", time.Minute),
			ConnectTimeout:    p.dur("DATABASE_CONNECT_TIMEOUT", 5*time.Second),
		},
		Redis: Redis{
			Addr:        p.str("REDIS_ADDR", "localhost:6379"),
			Password:    p.secret("REDIS_PASSWORD", ""),
			DB:          p.int("REDIS_DB", 0),
			Required:    p.bool("REDIS_REQUIRED", false),
			DialTimeout: p.dur("REDIS_DIAL_TIMEOUT", 3*time.Second),
			ReadTimeout: p.dur("REDIS_READ_TIMEOUT", 2*time.Second),
			PoolSize:    p.int("REDIS_POOL_SIZE", 10),
		},
		Auth: Auth{
			JWTSecret:  p.secret("AUTH_JWT_SECRET", ""),
			Issuer:     p.str("AUTH_ISSUER", "goscaffoldforlevi"),
			Audience:   p.str("AUTH_AUDIENCE", "goscaffoldforlevi-api"),
			AccessTTL:  p.dur("AUTH_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: p.dur("AUTH_REFRESH_TTL", 720*time.Hour),

			// OWASP's recommended Argon2id profile: 64 MiB, three passes,
			// four lanes.
			Argon2Time:    p.uint32("AUTH_ARGON2_TIME", 3),
			Argon2Memory:  p.uint32("AUTH_ARGON2_MEMORY_KIB", 64*1024),
			Argon2Threads: p.uint8("AUTH_ARGON2_THREADS", 4),
		},
		RateLimit: RateLimit{
			Enabled: p.bool("RATELIMIT_ENABLED", true),

			IPQuota:  p.int("RATELIMIT_IP_QUOTA", 100),
			IPPeriod: p.dur("RATELIMIT_IP_PERIOD", time.Minute),
			IPBurst:  p.int("RATELIMIT_IP_BURST", 200),

			UserQuota:  p.int("RATELIMIT_USER_QUOTA", 600),
			UserPeriod: p.dur("RATELIMIT_USER_PERIOD", time.Minute),
			UserBurst:  p.int("RATELIMIT_USER_BURST", 1000),

			LoginIPQuota:  p.int("RATELIMIT_LOGIN_IP_QUOTA", 10),
			LoginIPPeriod: p.dur("RATELIMIT_LOGIN_IP_PERIOD", 15*time.Minute),
			LoginIPBurst:  p.int("RATELIMIT_LOGIN_IP_BURST", 10),

			LoginIDQuota:  p.int("RATELIMIT_LOGIN_ID_QUOTA", 5),
			LoginIDPeriod: p.dur("RATELIMIT_LOGIN_ID_PERIOD", 15*time.Minute),
			LoginIDBurst:  p.int("RATELIMIT_LOGIN_ID_BURST", 5),
		},
		Messaging: Messaging{
			Enabled: p.bool("MESSAGING_ENABLED", false),

			NATSURLs:      p.csv("MESSAGING_NATS_URLS", []string{"nats://localhost:4222"}),
			StreamName:    p.str("MESSAGING_STREAM_NAME", "EVENTS"),
			SubjectPrefix: p.str("MESSAGING_SUBJECT_PREFIX", "events"),
			ConsumerName:  p.str("MESSAGING_CONSUMER_NAME", "worker"),

			ConnectTimeout: p.dur("MESSAGING_CONNECT_TIMEOUT", 5*time.Second),
			PublishTimeout: p.dur("MESSAGING_PUBLISH_TIMEOUT", 10*time.Second),
			StreamMaxAge:   p.dur("MESSAGING_STREAM_MAX_AGE", 7*24*time.Hour),
			AckWait:        p.dur("MESSAGING_ACK_WAIT", 5*time.Minute),
			MaxDeliver:     p.int("MESSAGING_MAX_DELIVER", 10),

			RelayPollInterval: p.dur("MESSAGING_RELAY_POLL_INTERVAL", time.Second),
			RelayBatchSize:    p.int32("MESSAGING_RELAY_BATCH_SIZE", 100),

			HandlerTimeout: p.dur("MESSAGING_HANDLER_TIMEOUT", 30*time.Second),
			MaxAttempts:    p.int("MESSAGING_MAX_ATTEMPTS", 5),
			BaseBackoff:    p.dur("MESSAGING_BASE_BACKOFF", time.Second),
			MaxBackoff:     p.dur("MESSAGING_MAX_BACKOFF", 30*time.Second),

			PublishedRetention: p.dur("MESSAGING_PUBLISHED_RETENTION", 7*24*time.Hour),
			ProcessedRetention: p.dur("MESSAGING_PROCESSED_RETENTION", 30*24*time.Hour),
			PruneInterval:      p.dur("MESSAGING_PRUNE_INTERVAL", time.Hour),
		},
		OTel: OTel{
			Enabled:       p.bool("OTEL_ENABLED", false),
			Endpoint:      p.str("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Protocol:      p.str("OTEL_EXPORTER_OTLP_PROTOCOL", OTLPGRPC),
			Insecure:      p.bool("OTEL_EXPORTER_OTLP_INSECURE", false),
			SamplingRatio: p.float("OTEL_SAMPLING_RATIO", 1.0),
		},
		Metrics: Metrics{
			Enabled: p.bool("METRICS_ENABLED", true),
			// Loopback by default: a metrics port that binds every interface
			// the moment someone runs the binary is exposed before anyone
			// decides to expose it.
			Addr: p.str("METRICS_ADDR", "127.0.0.1:9464"),
			Path: p.str("METRICS_PATH", "/metrics"),
		},
	}

	if err := p.err(); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks every constraint that must hold for the process to be able to
// serve traffic correctly. It returns all problems at once so that a
// misconfigured deployment can be fixed in a single pass.
func (c Config) Validate() error {
	var probs problems

	// App
	switch c.App.Environment {
	case EnvDevelopment, EnvStaging, EnvProduction:
	default:
		probs.add("APP_ENV", "must be one of development, staging, production")
	}
	if c.App.Name == "" {
		probs.add("APP_NAME", "must not be empty")
	}

	// HTTP
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		probs.add("HTTP_PORT", "must be between 1 and 65535")
	}
	probs.positive("HTTP_READ_HEADER_TIMEOUT", c.HTTP.ReadHeaderTimeout)
	probs.positive("HTTP_READ_TIMEOUT", c.HTTP.ReadTimeout)
	probs.positive("HTTP_WRITE_TIMEOUT", c.HTTP.WriteTimeout)
	probs.positive("HTTP_IDLE_TIMEOUT", c.HTTP.IdleTimeout)
	probs.positive("HTTP_REQUEST_TIMEOUT", c.HTTP.RequestTimeout)
	probs.positive("HTTP_SHUTDOWN_TIMEOUT", c.HTTP.ShutdownTimeout)
	if c.HTTP.MaxBodyBytes <= 0 {
		probs.add("HTTP_MAX_BODY_BYTES", "must be greater than zero")
	}
	if c.HTTP.MaxHeaderBytes <= 0 {
		probs.add("HTTP_MAX_HEADER_BYTES", "must be greater than zero")
	}
	if c.HTTP.TrustedProxyCount < 0 {
		probs.add("HTTP_TRUSTED_PROXY_COUNT", "must not be negative")
	}
	// A handler deadline longer than the write deadline means the client is
	// disconnected before the handler can answer: the work is wasted.
	if c.HTTP.RequestTimeout >= c.HTTP.WriteTimeout {
		probs.add("HTTP_REQUEST_TIMEOUT", "must be shorter than HTTP_WRITE_TIMEOUT")
	}
	if c.HTTP.ReadinessDrainDelay >= c.HTTP.ShutdownTimeout {
		probs.add("HTTP_READINESS_DRAIN_DELAY", "must be shorter than HTTP_SHUTDOWN_TIMEOUT")
	}
	if c.App.Environment.IsProduction() {
		for _, o := range c.HTTP.CORSAllowedOrigins {
			if o == "*" {
				probs.add("HTTP_CORS_ALLOWED_ORIGINS", "wildcard origin is not allowed in production")
			}
		}
	}

	// Database
	if c.Database.URL.IsEmpty() {
		probs.add("DATABASE_URL", "is required")
	} else if !strings.HasPrefix(c.Database.URL.Reveal(), "postgres://") &&
		!strings.HasPrefix(c.Database.URL.Reveal(), "postgresql://") {
		probs.add("DATABASE_URL", "must be a postgres:// or postgresql:// URL")
	}
	if c.Database.MaxConns < 1 {
		probs.add("DATABASE_MAX_CONNS", "must be at least 1")
	}
	if c.Database.MinConns < 0 || c.Database.MinConns > c.Database.MaxConns {
		probs.add("DATABASE_MIN_CONNS", "must be between 0 and DATABASE_MAX_CONNS")
	}
	probs.positive("DATABASE_CONNECT_TIMEOUT", c.Database.ConnectTimeout)

	// Redis
	if c.Redis.Addr == "" {
		probs.add("REDIS_ADDR", "is required")
	}
	probs.positive("REDIS_DIAL_TIMEOUT", c.Redis.DialTimeout)

	// Auth
	switch {
	case c.Auth.JWTSecret.IsEmpty():
		probs.add("AUTH_JWT_SECRET", "is required")
	case len(c.Auth.JWTSecret.Reveal()) < MinJWTSecretLen:
		probs.add("AUTH_JWT_SECRET",
			fmt.Sprintf("must be at least %d bytes", MinJWTSecretLen))
	case c.App.Environment.IsProduction() && isInsecureSecret(c.Auth.JWTSecret.Reveal()):
		probs.add("AUTH_JWT_SECRET", "must not use a well-known placeholder value in production")
	}
	probs.positive("AUTH_ACCESS_TTL", c.Auth.AccessTTL)
	// A long-lived access token cannot be revoked (see ADR-0007), so an
	// oversized TTL silently removes the mitigation the design relies on.
	if c.Auth.AccessTTL > time.Hour {
		probs.add("AUTH_ACCESS_TTL", "must not exceed 1h; access tokens cannot be revoked")
	}
	probs.positive("AUTH_REFRESH_TTL", c.Auth.RefreshTTL)
	if c.Auth.RefreshTTL <= c.Auth.AccessTTL {
		probs.add("AUTH_REFRESH_TTL", "must be longer than AUTH_ACCESS_TTL")
	}

	// Argon2. The floors match OWASP's minimum acceptable profile; going below
	// them is a security regression that nothing else in the system would
	// surface.
	if c.Auth.Argon2Time < 2 {
		probs.add("AUTH_ARGON2_TIME", "must be at least 2")
	}
	if c.Auth.Argon2Memory < 19*1024 {
		probs.add("AUTH_ARGON2_MEMORY_KIB", "must be at least 19456 (19 MiB)")
	}
	if c.Auth.Argon2Threads < 1 {
		probs.add("AUTH_ARGON2_THREADS", "must be at least 1")
	}

	// Rate limiting. The checks only run when limiting is enabled, but the
	// production guard below runs unconditionally: disabling limits in
	// production removes the only bound on Argon2 memory use.
	if c.App.Environment.IsProduction() && !c.RateLimit.Enabled {
		probs.add("RATELIMIT_ENABLED",
			"must be true in production; login rate limiting bounds Argon2 memory use")
	}
	if c.RateLimit.Enabled {
		for _, r := range []struct {
			prefix       string
			quota, burst int
			period       time.Duration
		}{
			{"RATELIMIT_IP", c.RateLimit.IPQuota, c.RateLimit.IPBurst, c.RateLimit.IPPeriod},
			{"RATELIMIT_USER", c.RateLimit.UserQuota, c.RateLimit.UserBurst, c.RateLimit.UserPeriod},
			{"RATELIMIT_LOGIN_IP", c.RateLimit.LoginIPQuota, c.RateLimit.LoginIPBurst, c.RateLimit.LoginIPPeriod},
			{"RATELIMIT_LOGIN_ID", c.RateLimit.LoginIDQuota, c.RateLimit.LoginIDBurst, c.RateLimit.LoginIDPeriod},
		} {
			if r.quota <= 0 {
				probs.add(r.prefix+"_QUOTA", "must be positive")
			}
			if r.burst <= 0 {
				probs.add(r.prefix+"_BURST", "must be positive")
			}
			probs.positive(r.prefix+"_PERIOD", r.period)
			// A burst smaller than nothing is meaningless, but a burst smaller
			// than the quota is worse: it caps throughput below the advertised
			// limit, so the service rejects requests it promised to serve.
			if r.burst > 0 && r.quota > 0 && r.burst < r.quota {
				probs.add(r.prefix+"_BURST", "must not be smaller than "+r.prefix+"_QUOTA")
			}
		}
	}

	// Messaging
	//
	// Everything here is gated on Enabled. A subsystem that is switched off
	// must not be able to stop the process from starting, or turning it off
	// stops being a way out of a problem.
	if c.Messaging.Enabled {
		if len(c.Messaging.NATSURLs) == 0 {
			probs.add("MESSAGING_NATS_URLS", "is required when MESSAGING_ENABLED is true")
		}
		if c.Messaging.ConsumerName == "" {
			probs.add("MESSAGING_CONSUMER_NAME", "must not be empty")
		}
		if c.Messaging.MaxAttempts < 1 {
			probs.add("MESSAGING_MAX_ATTEMPTS", "must be at least 1")
		}
		if c.Messaging.RelayBatchSize < 1 {
			probs.add("MESSAGING_RELAY_BATCH_SIZE", "must be at least 1")
		}
		probs.positive("MESSAGING_RELAY_POLL_INTERVAL", c.Messaging.RelayPollInterval)
		probs.positive("MESSAGING_HANDLER_TIMEOUT", c.Messaging.HandlerTimeout)
		probs.positive("MESSAGING_BASE_BACKOFF", c.Messaging.BaseBackoff)
		probs.positive("MESSAGING_PRUNE_INTERVAL", c.Messaging.PruneInterval)

		// The broker must not redeliver a message the consumer is still
		// retrying. If it does, two workers process the same event at once;
		// the dedupe log keeps that from duplicating the side effect, but one
		// of them then fails its claim and the message churns. The check turns
		// a subtle production symptom into a refusal to start.
		if budget := c.Messaging.retryBudget(); c.Messaging.AckWait < budget {
			probs.add("MESSAGING_ACK_WAIT",
				"must exceed the consumer retry budget of "+budget.String()+
					" (MAX_ATTEMPTS x HANDLER_TIMEOUT plus backoff)")
		}

		// Pruning the dedupe log inside the redelivery window makes an old
		// message processable a second time, which is the one failure the log
		// exists to prevent.
		if c.Messaging.ProcessedRetention <= c.Messaging.StreamMaxAge {
			probs.add("MESSAGING_PROCESSED_RETENTION",
				"must exceed MESSAGING_STREAM_MAX_AGE, or a redelivered message "+
					"would be processed twice")
		}
	}

	// OTel
	if c.OTel.SamplingRatio < 0 || c.OTel.SamplingRatio > 1 {
		probs.add("OTEL_SAMPLING_RATIO", "must be between 0 and 1")
	}
	if c.OTel.Enabled && c.OTel.Endpoint == "" {
		probs.add("OTEL_EXPORTER_OTLP_ENDPOINT", "is required when OTEL_ENABLED is true")
	}
	switch c.OTel.Protocol {
	case OTLPGRPC, OTLPHTTP:
	default:
		probs.add("OTEL_EXPORTER_OTLP_PROTOCOL",
			`must be "`+OTLPGRPC+`" or "`+OTLPHTTP+`"`)
	}

	// Metrics
	if c.Metrics.Enabled {
		if !strings.HasPrefix(c.Metrics.Path, "/") {
			probs.add("METRICS_PATH", "must begin with /")
		}
		if _, _, err := net.SplitHostPort(c.Metrics.Addr); err != nil {
			probs.add("METRICS_ADDR", "must be host:port")
		} else if c.App.Environment == EnvProduction && c.Metrics.Addr == c.HTTP.Addr() {
			// Metrics name every route template, dependency and build version,
			// and expose enough timing to make enumeration cheaper. Serving
			// them from the port that terminates public traffic is a
			// disclosure, and it is a silent one, so it is refused here rather
			// than left to whoever writes the ingress rules.
			probs.add("METRICS_ADDR",
				"must not be the public HTTP address in production; "+
					"use a separate port that ingress does not route to")
		}
	}

	return probs.err()
}

func isInsecureSecret(s string) bool {
	for _, bad := range InsecureJWTSecrets {
		if s == bad {
			return true
		}
	}
	return false
}

// problems accumulates validation failures.
type problems struct{ items []string }

func (p *problems) add(key, reason string) {
	p.items = append(p.items, key+": "+reason)
}

func (p *problems) positive(key string, d time.Duration) {
	if d <= 0 {
		p.add(key, "must be greater than zero")
	}
}

func (p *problems) err() error {
	if len(p.items) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(p.items, "\n  - "))
}

// parser reads typed values from the environment, collecting parse failures
// instead of panicking or silently falling back to a default.
type parser struct{ problems }

func (p *parser) lookup(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return "", false
	}
	return strings.TrimSpace(v), true
}

func (p *parser) str(key, def string) string {
	if v, ok := p.lookup(key); ok {
		return v
	}
	return def
}

func (p *parser) secret(key, def string) Secret {
	return Secret(p.str(key, def))
}

func (p *parser) int(key string, def int) int {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		p.add(key, "must be an integer")
		return def
	}
	return n
}

// int32 parses a value that the pgx pool requires as int32. The range check
// is not pedantry: an out-of-range value would wrap silently and produce a
// negative pool size, which fails much later and much less clearly.
func (p *parser) int32(key string, def int32) int32 {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		p.add(key, "must be a 32-bit integer")
		return def
	}
	return int32(n)
}

// uint32 parses a value that must be non-negative and fit in 32 bits. Argon2
// takes its cost parameters as uint32, and a negative or oversized value would
// wrap into a nonsensical cost rather than being rejected.
func (p *parser) uint32(key string, def uint32) uint32 {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		p.add(key, "must be a non-negative 32-bit integer")
		return def
	}
	return uint32(n)
}

func (p *parser) uint8(key string, def uint8) uint8 {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	n, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		p.add(key, "must be an integer between 0 and 255")
		return def
	}
	return uint8(n)
}

func (p *parser) float(key string, def float64) float64 {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		p.add(key, "must be a number")
		return def
	}
	return f
}

func (p *parser) bool(key string, def bool) bool {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		p.add(key, "must be a boolean (true/false)")
		return def
	}
	return b
}

func (p *parser) dur(key string, def time.Duration) time.Duration {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		p.add(key, "must be a duration such as 30s or 5m")
		return def
	}
	return d
}

func (p *parser) logLevel(key string, def slog.Level) slog.Level {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	var level slog.Level
	// UnmarshalText accepts DEBUG/INFO/WARN/ERROR and offsets like WARN+2.
	if err := level.UnmarshalText([]byte(v)); err != nil {
		p.add(key, "must be one of debug, info, warn, error")
		return def
	}
	return level
}

func (p *parser) csv(key string, def []string) []string {
	v, ok := p.lookup(key)
	if !ok {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
