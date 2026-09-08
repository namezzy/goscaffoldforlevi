package config_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/config"
)

const plaintext = "super-secret-value-that-must-not-leak"

// TestSecretNeverLeaks covers every path by which a value normally escapes a
// Go process: fmt verbs, structured logging, JSON encoding. Redaction that
// covers only some of them is worse than none, because it creates confidence
// that is not warranted.
func TestSecretNeverLeaks(t *testing.T) {
	s := config.Secret(plaintext)

	renderings := map[string]string{
		"%v":         fmt.Sprintf("%v", s),
		"%s":         fmt.Sprint(s),
		"%q":         fmt.Sprintf("%q", s),
		"%#v":        fmt.Sprintf("%#v", s),
		"%+v":        fmt.Sprintf("%+v", s),
		"String()":   s.String(),
		"in struct":  fmt.Sprintf("%+v", struct{ Password config.Secret }{s}),
		"in pointer": fmt.Sprintf("%+v", &struct{ Password config.Secret }{s}),
	}

	for name, out := range renderings {
		if strings.Contains(out, plaintext) {
			t.Errorf("%s leaked the secret: %s", name, out)
		}
	}

	encoded, err := json.Marshal(struct {
		Password config.Secret `json:"password"`
	}{s})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(encoded), plaintext) {
		t.Errorf("json.Marshal leaked the secret: %s", encoded)
	}

	var buf strings.Builder
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("config", slog.Any("password", s))
	if strings.Contains(buf.String(), plaintext) {
		t.Errorf("slog leaked the secret: %s", buf.String())
	}
}

func TestSecretRevealReturnsThePlaintext(t *testing.T) {
	if got := config.Secret(plaintext).Reveal(); got != plaintext {
		t.Fatalf("Reveal() = %q, want the plaintext", got)
	}
}

// validConfig returns a configuration that passes Validate, so that each test
// can change exactly one thing and attribute the failure to it.
func validConfig() config.Config {
	return config.Config{
		App: config.App{
			Name:        "goscaffoldforlevi",
			Environment: config.EnvProduction,
			Version:     "1.0.0",
			LogLevel:    slog.LevelInfo,
		},
		HTTP: config.HTTP{
			Host:                "0.0.0.0",
			Port:                8080,
			ReadHeaderTimeout:   5 * secondsUnit,
			ReadTimeout:         15 * secondsUnit,
			WriteTimeout:        30 * secondsUnit,
			IdleTimeout:         60 * secondsUnit,
			MaxHeaderBytes:      1 << 20,
			RequestTimeout:      15 * secondsUnit,
			MaxBodyBytes:        1 << 20,
			ShutdownTimeout:     30 * secondsUnit,
			ReadinessDrainDelay: 5 * secondsUnit,
			CORSAllowedOrigins:  []string{"https://app.example.com"},
			TrustedProxyCount:   0,
		},
		Database: config.Database{
			URL:               config.Secret("postgres://user:pass@localhost:5432/db"),
			MaxConns:          10,
			MinConns:          2,
			MaxConnLifetime:   3600 * secondsUnit,
			MaxConnIdleTime:   600 * secondsUnit,
			HealthCheckPeriod: 60 * secondsUnit,
			ConnectTimeout:    5 * secondsUnit,
		},
		Redis: config.Redis{
			Addr:        "localhost:6379",
			DB:          0,
			Required:    false,
			DialTimeout: 3 * secondsUnit,
			ReadTimeout: 3 * secondsUnit,
			PoolSize:    10,
		},
		Auth: config.Auth{
			JWTSecret:  config.Secret("a-genuinely-random-32-byte-secret!!"),
			Issuer:     "goscaffoldforlevi",
			Audience:   "goscaffoldforlevi-api",
			AccessTTL:  900 * secondsUnit,
			RefreshTTL: 2592000 * secondsUnit,

			Argon2Time:    3,
			Argon2Memory:  64 * 1024,
			Argon2Threads: 4,
		},
		RateLimit: config.RateLimit{
			Enabled:       true,
			IPQuota:       100,
			IPPeriod:      60 * secondsUnit,
			IPBurst:       200,
			UserQuota:     600,
			UserPeriod:    60 * secondsUnit,
			UserBurst:     1000,
			LoginIPQuota:  10,
			LoginIPPeriod: 900 * secondsUnit,
			LoginIPBurst:  10,
			LoginIDQuota:  5,
			LoginIDPeriod: 900 * secondsUnit,
			LoginIDBurst:  5,
		},
		OTel: config.OTel{
			Enabled:       false,
			Endpoint:      "localhost:4317",
			Protocol:      config.OTLPGRPC,
			SamplingRatio: 0.1,
		},
		Metrics: config.Metrics{
			Enabled: true,
			Addr:    "127.0.0.1:9464",
			Path:    "/metrics",
		},
	}
}

func TestValidateAcceptsAGoodConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*config.Config)
		want   string
	}{
		{
			name:   "unknown environment",
			mutate: func(c *config.Config) { c.App.Environment = "prod" },
			want:   "APP_ENV",
		},
		{
			name:   "port out of range",
			mutate: func(c *config.Config) { c.HTTP.Port = 70000 },
			want:   "HTTP_PORT",
		},
		{
			// A request timeout above the write timeout means the connection
			// is torn down before the timeout handler can answer, so the
			// client sees a reset instead of a 504.
			name:   "request timeout exceeds write timeout",
			mutate: func(c *config.Config) { c.HTTP.RequestTimeout = 60 * secondsUnit },
			want:   "HTTP_REQUEST_TIMEOUT",
		},
		{
			name:   "drain delay exceeds shutdown budget",
			mutate: func(c *config.Config) { c.HTTP.ReadinessDrainDelay = 60 * secondsUnit },
			want:   "HTTP_READINESS_DRAIN_DELAY",
		},
		{
			name:   "wildcard CORS in production",
			mutate: func(c *config.Config) { c.HTTP.CORSAllowedOrigins = []string{"*"} },
			want:   "CORS_ALLOWED_ORIGINS",
		},
		{
			name:   "non-postgres database url",
			mutate: func(c *config.Config) { c.Database.URL = config.Secret("mysql://localhost/db") },
			want:   "DATABASE_URL",
		},
		{
			name:   "short jwt secret",
			mutate: func(c *config.Config) { c.Auth.JWTSecret = config.Secret("too-short") },
			want:   "AUTH_JWT_SECRET",
		},
		{
			name:   "access ttl too long",
			mutate: func(c *config.Config) { c.Auth.AccessTTL = 7200 * secondsUnit },
			want:   "AUTH_ACCESS_TTL",
		},
		{
			name:   "refresh ttl not longer than access ttl",
			mutate: func(c *config.Config) { c.Auth.RefreshTTL = 60 * secondsUnit },
			want:   "AUTH_REFRESH_TTL",
		},
		{
			name:   "argon2 memory below the owasp floor",
			mutate: func(c *config.Config) { c.Auth.Argon2Memory = 4096 },
			want:   "AUTH_ARGON2_MEMORY_KIB",
		},
		{
			name:   "rate limiting disabled in production",
			mutate: func(c *config.Config) { c.RateLimit.Enabled = false },
			want:   "RATELIMIT_ENABLED",
		},
		{
			name:   "login burst smaller than login quota",
			mutate: func(c *config.Config) { c.RateLimit.LoginIDBurst = 1 },
			want:   "RATELIMIT_LOGIN_ID_BURST",
		},
		{
			name:   "min conns above max conns",
			mutate: func(c *config.Config) { c.Database.MinConns = 50 },
			want:   "DATABASE_MIN_CONNS",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := validConfig()
			c.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %s", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Validate() = %v, want it to mention %s", err, c.want)
			}
		})
	}
}

// TestValidateRejectsExampleSecretsInProduction is the check that matters
// most: shipping .env.example values to production is a recurring real-world
// incident, and the pipeline is not a reliable place to catch it.
func TestValidateRejectsExampleSecretsInProduction(t *testing.T) {
	for _, insecure := range config.InsecureJWTSecrets {
		cfg := validConfig()
		cfg.Auth.JWTSecret = config.Secret(insecure)

		err := cfg.Validate()
		if err == nil {
			t.Errorf("Validate() accepted the placeholder secret %q in production", insecure)
			continue
		}
		if strings.Contains(err.Error(), insecure) {
			t.Errorf("Validate() echoed the secret in its error: %v", err)
		}
	}
}

func TestValidateAllowsExampleSecretsInDevelopment(t *testing.T) {
	// Development must stay frictionless, or developers will work around the
	// check in ways that also disable it in production.
	cfg := validConfig()
	cfg.App.Environment = config.EnvDevelopment
	cfg.Auth.JWTSecret = config.Secret(config.InsecureJWTSecrets[0])
	cfg.HTTP.CORSAllowedOrigins = []string{"*"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil in development", err)
	}
}

// TestValidateReportsEveryProblem verifies the accumulating behaviour: fixing
// configuration one restart at a time is intolerable in a deploy loop.
func TestValidateReportsEveryProblem(t *testing.T) {
	cfg := validConfig()
	cfg.HTTP.Port = -1
	cfg.App.Environment = "staging-ish"
	cfg.Auth.JWTSecret = config.Secret("short")

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error")
	}

	for _, want := range []string{"HTTP_PORT", "APP_ENV", "AUTH_JWT_SECRET"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
	}
}

func TestAddrJoinsHostAndPort(t *testing.T) {
	cfg := config.HTTP{Host: "127.0.0.1", Port: 8080}
	if got := cfg.Addr(); got != "127.0.0.1:8080" {
		t.Fatalf("Addr() = %q, want 127.0.0.1:8080", got)
	}
}

// secondsUnit keeps the fixture readable without importing time in every line.
const secondsUnit = time.Second

// --- Phase 5: observability configuration ---

func TestValidateRejectsObservabilitySettings(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*config.Config)
		want   string
	}{
		{
			name:   "unknown OTLP protocol",
			mutate: func(c *config.Config) { c.OTel.Protocol = "thrift" },
			want:   "OTEL_EXPORTER_OTLP_PROTOCOL",
		},
		{
			name:   "metrics path is not a path",
			mutate: func(c *config.Config) { c.Metrics.Path = "metrics" },
			want:   "METRICS_PATH",
		},
		{
			name:   "metrics address has no port",
			mutate: func(c *config.Config) { c.Metrics.Addr = "127.0.0.1" },
			want:   "METRICS_ADDR",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %s", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error does not mention %s: %v", tc.want, err)
			}
		})
	}
}

// Metrics describe internal topology: route templates, dependency names, build
// version, and enough timing to make enumeration cheaper. Serving them on the
// port the internet reaches is a disclosure, and the mistake is silent, so the
// loader refuses it in production rather than leaving it to a deployment review.
func TestValidateRefusesToServeMetricsOnThePublicPortInProduction(t *testing.T) {
	cfg := validConfig()
	cfg.App.Environment = config.EnvProduction
	cfg.Metrics.Enabled = true
	cfg.Metrics.Addr = cfg.HTTP.Addr()

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want production to reject metrics on the public port")
	}
	if !strings.Contains(err.Error(), "METRICS_ADDR") {
		t.Errorf("error does not mention METRICS_ADDR: %v", err)
	}
}

// The same overlap in development is a convenience, not a disclosure: nothing
// is reachable from outside. Refusing it everywhere would make people set two
// ports to run one binary locally, and a rule that gets in the way at every
// `go run` is a rule that gets disabled.
func TestValidateAllowsMetricsOnThePublicPortInDevelopment(t *testing.T) {
	cfg := validConfig()
	cfg.App.Environment = config.EnvDevelopment
	cfg.Auth.JWTSecret = config.Secret("a-genuinely-random-32-byte-secret!!")
	cfg.Metrics.Enabled = true
	cfg.Metrics.Addr = cfg.HTTP.Addr()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want development to allow the overlap", err)
	}
}

// Disabled means disabled: a config that would be rejected while serving is
// irrelevant when nothing is served, and rejecting it would block startup over
// a setting that has no effect.
func TestValidateIgnoresTheMetricsAddressWhenMetricsAreOff(t *testing.T) {
	cfg := validConfig()
	cfg.App.Environment = config.EnvProduction
	cfg.Metrics.Enabled = false
	cfg.Metrics.Addr = cfg.HTTP.Addr()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want no error when metrics are disabled", err)
	}
}
