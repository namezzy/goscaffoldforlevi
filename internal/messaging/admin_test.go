package messaging_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/messaging"
)

// fakeAdminStore records the limit it was called with, so the clamping logic
// in Admin can be tested without a database.
type fakeAdminStore struct {
	gotLimit int32
}

func (f *fakeAdminStore) OutboxStatus(context.Context) (messaging.OutboxStatus, error) {
	return messaging.OutboxStatus{}, nil
}

func (f *fakeAdminStore) ListDeadLetters(_ context.Context, _ string, limit int32) (messaging.DeadLetterPage, error) {
	f.gotLimit = limit
	return messaging.DeadLetterPage{}, nil
}

func (f *fakeAdminStore) Replay(context.Context, uuid.UUID) error { return nil }

func TestListDeadLettersClampsTheLimit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		limit int32
		want  int32
	}{
		{"zero falls back to the default", 0, messaging.DefaultDeadLetterLimit},
		{"negative falls back to the default", -5, messaging.DefaultDeadLetterLimit},
		{"within range passes through unchanged", 10, 10},
		{"over the maximum is clamped down", messaging.MaxDeadLetterLimit + 1000, messaging.MaxDeadLetterLimit},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeAdminStore{}
			admin := messaging.NewAdmin(store)

			if _, err := admin.ListDeadLetters(context.Background(), "", tc.limit); err != nil {
				t.Fatalf("ListDeadLetters: %v", err)
			}
			if store.gotLimit != tc.want {
				t.Errorf("store received limit %d, want %d", store.gotLimit, tc.want)
			}
		})
	}
}
