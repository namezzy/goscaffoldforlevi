package auth_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/telemetry"
)

type loginCounter struct{ ok, failed atomic.Int64 }

func (c *loginCounter) LoginAttempt(ok bool) {
	if ok {
		c.ok.Add(1)
		return
	}
	c.failed.Add(1)
}

func TestTheMetricsRegistryIsAnAuthObserver(t *testing.T) {
	var _ auth.Observer = telemetry.NewMetrics()
}

// Every terminating path of Login must be counted exactly once. The failure
// paths are the point: an unknown email, a wrong password, and a disabled
// account are three different branches that all return the same error, and a
// counter attached to only one of them would under-report a credential
// stuffing run by whatever fraction of the guesses hit real accounts.
func TestEveryLoginOutcomeIsCountedExactlyOnce(t *testing.T) {
	for _, tc := range []struct {
		name           string
		email, pass    string
		wantOK, wantNo int64
	}{
		{"correct credentials", "a@example.com", testPassword, 1, 0},
		{"unknown account", "nobody@example.com", testPassword, 0, 1},
		{"wrong password", "a@example.com", "wrong horse battery staple", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &loginCounter{}
			svc := observedFixture(t, c)

			_, err := svc.Login(context.Background(), tc.email, tc.pass, auth.RequestMeta{})
			if (err == nil) != (tc.wantOK == 1) {
				t.Fatalf("Login() error = %v, want success=%t", err, tc.wantOK == 1)
			}
			if got := c.ok.Load(); got != tc.wantOK {
				t.Errorf("counted %d successes, want %d", got, tc.wantOK)
			}
			if got := c.failed.Load(); got != tc.wantNo {
				t.Errorf("counted %d failures, want %d", got, tc.wantNo)
			}
		})
	}
}

// A malformed request is a client error, not a credential check, and counting
// it as a login failure would let anyone raise the authentication failure rate
// by posting empty bodies — turning the security alert into a nuisance.
func TestAMalformedLoginRequestIsNotCountedAsAFailedLogin(t *testing.T) {
	c := &loginCounter{}
	svc := observedFixture(t, c)

	if _, err := svc.Login(context.Background(), "", "", auth.RequestMeta{}); err == nil {
		t.Fatal("Login() accepted an empty request")
	}
	if got := c.ok.Load() + c.failed.Load(); got != 0 {
		t.Errorf("counted %d login attempts for a malformed request", got)
	}
}

// A service without an observer must work: a nil interface on the login path
// is a panic on every sign-in.
func TestAnAuthServiceWithoutAnObserverStillLogsIn(t *testing.T) {
	svc := observedFixture(t, nil)

	if _, err := svc.Login(context.Background(), "a@example.com", testPassword, auth.RequestMeta{}); err != nil {
		t.Fatalf("Login() = %v", err)
	}
}

func observedFixture(t *testing.T, o auth.Observer) *auth.Service {
	t.Helper()
	f := newServiceFixture(t, newFakeAccounts().
		with("a@example.com", accountFor(t, "user-1", true)))
	if o != nil {
		f.svc.SetObserver(o)
	}
	return f.svc
}
