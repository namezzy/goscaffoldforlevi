package ratelimit_test

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/ratelimit"
)

// The rejection counter must not be sampled.
//
// The rejection *log* is sampled deliberately, because an attack would
// otherwise turn the defence into a log flood. A counter costs nothing per
// event, so sampling it buys nothing and destroys the only number that says
// how large the attack is: a sampled counter reports one rejection per
// interval no matter whether the attacker sent two requests or two million.
func TestEveryRejectionIsCountedEvenThoughTheLogIsSampled(t *testing.T) {
	var rejections, logLines atomic.Int64

	log := slog.New(countingHandler{
		Handler: slog.NewTextHandler(io.Discard, nil),
		count:   &logLines,
	})

	cfg := ratelimit.Config{
		Enabled: true,
		IP:      ratelimit.Rule{Quota: 1, Period: time.Hour, Burst: 1},
	}
	g := ratelimit.NewGuard(cfg, nil, log)
	g.RejectionObserver = func(ratelimit.Scope) { rejections.Add(1) }

	const attempts = 50
	for i := 0; i < attempts; i++ {
		if _, err := g.Allow(context.Background(), ratelimit.ScopeIP, "203.0.113.9"); err != nil {
			t.Fatalf("Allow() = %v", err)
		}
	}

	denied := int64(attempts) - int64(cfg.IP.Burst*2)
	if got := rejections.Load(); got != denied {
		t.Errorf("counted %d rejections, want every one of %d", got, denied)
	}
	if logLines.Load() >= denied {
		t.Errorf("the log was not sampled: %d lines for %d rejections",
			logLines.Load(), denied)
	}
}

// Allowed requests must not be counted as rejections: a counter that ticks on
// success makes the rejection rate equal the request rate, and the dashboard
// reads as a permanent attack.
func TestAnAllowedRequestIsNotCountedAsARejection(t *testing.T) {
	var rejections atomic.Int64

	cfg := ratelimit.Config{
		Enabled: true,
		IP:      ratelimit.Rule{Quota: 100, Period: time.Hour, Burst: 100},
	}
	g := ratelimit.NewGuard(cfg, nil, discard())
	g.RejectionObserver = func(ratelimit.Scope) { rejections.Add(1) }

	for i := 0; i < 10; i++ {
		if _, err := g.Allow(context.Background(), ratelimit.ScopeIP, "203.0.113.9"); err != nil {
			t.Fatalf("Allow() = %v", err)
		}
	}

	if got := rejections.Load(); got != 0 {
		t.Errorf("counted %d rejections for ten allowed requests", got)
	}
}

// A guard without an observer must work: instrumentation is optional, and a
// nil func called on the request path is a panic on every rejection — which
// would turn the rate limiter from a defence into the outage.
func TestAGuardWithoutARejectionObserverStillRejects(t *testing.T) {
	cfg := ratelimit.Config{
		Enabled: true,
		IP:      ratelimit.Rule{Quota: 1, Period: time.Hour, Burst: 1},
	}
	g := ratelimit.NewGuard(cfg, nil, discard())

	for i := 0; i < 5; i++ {
		if _, err := g.Allow(context.Background(), ratelimit.ScopeIP, "203.0.113.9"); err != nil {
			t.Fatalf("Allow() = %v", err)
		}
	}
}

func discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type countingHandler struct {
	slog.Handler
	count *atomic.Int64
}

func (h countingHandler) Handle(ctx context.Context, r slog.Record) error {
	h.count.Add(1)
	return h.Handler.Handle(ctx, r)
}
