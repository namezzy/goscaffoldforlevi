package user_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
	"github.com/namezzy/goscaffoldforlevi/internal/user"
)

type countingObserver struct{ n atomic.Int64 }

func (c *countingObserver) UserRegistered() { c.n.Add(1) }

func TestTheMetricsRegistryIsAUserObserver(t *testing.T) {
	var _ user.Observer = telemetry.NewMetrics()
}

// The registration counter must only move for users that actually exist.
// Counting the attempt would make the signup graph a graph of form
// submissions, so a validation regression that rejected every signup would
// show as record traffic instead of as an outage.
func TestOnlyACommittedRegistrationIsCounted(t *testing.T) {
	obs := &countingObserver{}
	svc := newService(newFakeRepository())
	svc.SetObserver(obs)

	if _, err := svc.Create(context.Background(), user.CreateInput{
		Email: "not an email", Name: "N", Password: validPassword, Role: auth.RoleUser,
	}); err == nil {
		t.Fatal("Create() accepted an invalid email")
	}
	if got := obs.n.Load(); got != 0 {
		t.Errorf("counted %d registrations for a rejected signup", got)
	}

	if _, err := svc.Create(context.Background(), user.CreateInput{
		Email: "new@example.com", Name: "N", Password: validPassword, Role: auth.RoleUser,
	}); err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if got := obs.n.Load(); got != 1 {
		t.Errorf("counted %d registrations for one signup", got)
	}
}

// A service without an observer must work: instrumentation is optional, and a
// nil interface on the request path is a panic on every signup.
func TestAUserServiceWithoutAnObserverStillRegisters(t *testing.T) {
	svc := newService(newFakeRepository())

	if _, err := svc.Create(context.Background(), user.CreateInput{
		Email: "new@example.com", Name: "N", Password: validPassword, Role: auth.RoleUser,
	}); err != nil {
		t.Fatalf("Create() = %v", err)
	}
}

// Found by a mutation that survived: raising the counter before checking the
// transaction's error passed every test, because the only rejected signup in
// them failed validation, which returns long before the transaction opens.
//
// The surviving mutant is the interesting failure mode. Validation errors are
// the user's problem; a failing transaction is ours, and it is exactly when
// the registration graph must stop. Counting inside the transaction would
// leave a database outage looking like a normal afternoon of signups.
func TestARolledBackRegistrationIsNotCounted(t *testing.T) {
	obs := &countingObserver{}
	repo := newFakeRepository()
	svc := newServiceWithPublisher(repo, refusingPublisher{}, rollbackTx{repo: repo})
	svc.SetObserver(obs)

	if _, err := svc.Create(context.Background(), user.CreateInput{
		Email: "new@example.com", Name: "N", Password: validPassword, Role: auth.RoleUser,
	}); err == nil {
		t.Fatal("Create() succeeded despite the transaction failing")
	}

	if got := obs.n.Load(); got != 0 {
		t.Errorf("counted %d registrations for a rolled back signup", got)
	}
}

// refusingPublisher fails the outbox write, which fails the transaction the
// insert shares with it.
type refusingPublisher struct{}

func (refusingPublisher) Publish(context.Context, messaging.Event) error {
	return errors.New("outbox is unavailable")
}
