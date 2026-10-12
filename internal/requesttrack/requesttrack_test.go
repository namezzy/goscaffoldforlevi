package requesttrack_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/requesttrack"
)

func eventN(n int) requesttrack.Event {
	return requesttrack.Event{
		Time:   time.Unix(int64(n), 0),
		Path:   fmt.Sprintf("/n/%d", n),
		Method: "GET",
		Status: 200,
	}
}

func TestRecentReturnsFewerThanCapacityOldestFirst(t *testing.T) {
	t.Parallel()

	tr := requesttrack.NewTracker(5)
	for i := 0; i < 3; i++ {
		tr.Record(eventN(i))
	}

	got := tr.Recent()
	if len(got) != 3 {
		t.Fatalf("Recent() returned %d events, want 3", len(got))
	}
	for i, e := range got {
		if e.Path != fmt.Sprintf("/n/%d", i) {
			t.Errorf("Recent()[%d].Path = %q, want /n/%d", i, e.Path, i)
		}
	}
}

// TestRecentWrapsWithoutLosingOrder is the reason this is a ring buffer test
// and not just a slice test: writing past capacity must overwrite the
// oldest entry and Recent must still report oldest-first, not just replay
// the underlying array in storage order.
func TestRecentWrapsWithoutLosingOrder(t *testing.T) {
	t.Parallel()

	tr := requesttrack.NewTracker(3)
	for i := 0; i < 7; i++ {
		tr.Record(eventN(i))
	}

	got := tr.Recent()
	if len(got) != 3 {
		t.Fatalf("Recent() returned %d events, want 3 (capacity)", len(got))
	}
	// Only the last 3 of 7 writes (4, 5, 6) should survive, oldest first.
	want := []int{4, 5, 6}
	for i, n := range want {
		if got[i].Path != fmt.Sprintf("/n/%d", n) {
			t.Errorf("Recent()[%d].Path = %q, want /n/%d", i, got[i].Path, n)
		}
	}
}

// TestRecentWrapsExactlyAtCapacity guards the boundary where next wraps back
// to zero on the write that exactly fills the buffer — the one case where
// "oldest is at buf[next]" degenerates to "oldest is at buf[0]".
func TestRecentWrapsExactlyAtCapacity(t *testing.T) {
	t.Parallel()

	tr := requesttrack.NewTracker(4)
	for i := 0; i < 4; i++ {
		tr.Record(eventN(i))
	}

	got := tr.Recent()
	if len(got) != 4 {
		t.Fatalf("Recent() returned %d events, want 4", len(got))
	}
	for i := range got {
		if got[i].Path != fmt.Sprintf("/n/%d", i) {
			t.Errorf("Recent()[%d].Path = %q, want /n/%d", i, got[i].Path, i)
		}
	}
}

func TestSubscribeReceivesEventsRecordedAfterward(t *testing.T) {
	t.Parallel()

	tr := requesttrack.NewTracker(10)
	tr.Record(eventN(0)) // before subscribing — must not be delivered

	ch, cancel := tr.Subscribe()
	defer cancel()

	tr.Record(eventN(1))

	select {
	case e := <-ch:
		if e.Path != "/n/1" {
			t.Errorf("received %q, want /n/1", e.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the subscribed event")
	}
}

// TestASlowSubscriberDropsEventsRatherThanBlockingRecord is the guarantee
// the whole package exists to provide: a live admin dashboard tab that stops
// reading must never be able to slow down the request path recording into
// it.
func TestASlowSubscriberDropsEventsRatherThanBlockingRecord(t *testing.T) {
	t.Parallel()

	tr := requesttrack.NewTracker(1000)
	ch, cancel := tr.Subscribe()
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Far more than the subscriber buffer holds, and nothing ever reads
		// ch. If Record blocked on a full subscriber channel, this would
		// hang instead of returning.
		for i := 0; i < 1000; i++ {
			tr.Record(eventN(i))
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Record blocked on a slow subscriber instead of dropping")
	}

	// The buffer itself is unaffected — Recent still has everything.
	if got := len(tr.Recent()); got != 1000 {
		t.Errorf("Recent() has %d events, want 1000", got)
	}
	_ = ch
}

func TestCancelStopsDeliveryAndClosesTheChannel(t *testing.T) {
	t.Parallel()

	tr := requesttrack.NewTracker(10)
	ch, cancel := tr.Subscribe()
	cancel()

	tr.Record(eventN(0))

	_, ok := <-ch
	if ok {
		t.Error("the channel delivered an event after cancel")
	}
}
