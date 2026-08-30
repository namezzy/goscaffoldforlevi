package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
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
			Name:        p.str("APP_NAME", "go-enterprise"),
			Environment: Environment(p.str("APP_ENV", string(EnvDevelopment))),
			Version:     p.str("APP_VERSION", "dev"),
			LogLevel:    p.logLevel("LOG_LEVEL", slog.LevelInfo),
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
			Issuer:     p.str("AUTH_ISSUER", "go-enterprise"),
			Audience:   p.str("AUTH_AUDIENCE", "go-enterprise-api"),
			AccessTTL:  p.dur("AUTH_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: p.dur("AUTH_REFRESH_TTL", 720*time.Hour),
		},
		OTel: OTel{
			Enabled:       p.bool("OTEL_ENABLED", false),
			Endpoint:      p.str("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			SamplingRatio: p.float("OTEL_SAMPLING_RATIO", 1.0),
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

	// OTel
	if c.OTel.SamplingRatio < 0 || c.OTel.SamplingRatio > 1 {
		probs.add("OTEL_SAMPLING_RATIO", "must be between 0 and 1")
	}
	if c.OTel.Enabled && c.OTel.Endpoint == "" {
		probs.add("OTEL_EXPORTER_OTLP_ENDPOINT", "is required when OTEL_ENABLED is true")
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
