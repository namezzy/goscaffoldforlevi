// Package requesttrack keeps a short rolling window of recently completed
// HTTP requests in memory, and lets admin tooling watch new ones arrive live.
//
// It is not an audit log and not durable: state resets on every deploy and
// only the most recent events are kept. That is a deliberate trade for a
// "what is happening right now" view, not a queryable history — long-term
// metrics and traces are what deployments/observability's Prometheus/Tempo
// stack already exists to answer.
package requesttrack

import (
	"sync"
	"time"
)

// Event is one completed HTTP request, reduced to what an operator watching
// live traffic needs. It carries no request or response body: those can
// contain credentials and personal data, and a live feed is exactly the kind
// of thing whose output ends up on someone's screen during a call.
type Event struct {
	Time       time.Time
	RequestID  string
	Method     string
	Path       string
	Status     int
	Bytes      int64
	DurationMS float64
	// IP is the resolved client address (see httpx.ClientIP) rendered as
	// text, empty if it could not be determined. It is the address a rate
	// limit or an audit trail would attribute the request to — not
	// necessarily the TCP peer, if a trusted proxy is configured.
	IP string
	// CountryCode is the ISO 3166-1 alpha-2 code from package geoip, empty
	// when unresolved.
	CountryCode string
	// CountryName is geoip's localised label — "内网" for a private or
	// loopback address, empty when a public address could not be resolved
	// (no database configured, or no match).
	CountryName string
}

// DefaultCapacity is how many recent events Tracker keeps.
const DefaultCapacity = 500

// subscriberBuffer bounds how far behind a live subscriber may fall before
// its oldest unseen events are dropped in favour of newer ones.
const subscriberBuffer = 64

// Tracker is a bounded ring buffer of recent events plus a live fan-out to
// subscribers. All methods are safe for concurrent use.
type Tracker struct {
	mu   sync.Mutex
	buf  []Event
	next int
	full bool

	subs map[chan Event]struct{}
}

// NewTracker builds a Tracker holding up to capacity events. A non-positive
// capacity falls back to DefaultCapacity rather than building a tracker that
// can hold nothing.
func NewTracker(capacity int) *Tracker {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Tracker{
		buf:  make([]Event, capacity),
		subs: make(map[chan Event]struct{}),
	}
}

// Record stores e and hands it to every live subscriber.
//
// A subscriber that is not keeping up has the event dropped rather than the
// caller blocked: Record runs on every request's hot path, and a slow admin
// dashboard tab must never be able to slow down real traffic. The send to
// each subscriber happens after the lock is released, so a blocked send
// cannot stall Record for every other caller either.
func (t *Tracker) Record(e Event) {
	t.mu.Lock()
	t.buf[t.next] = e
	t.next = (t.next + 1) % len(t.buf)
	if t.next == 0 {
		t.full = true
	}
	subs := make([]chan Event, 0, len(t.subs))
	for ch := range t.subs {
		subs = append(subs, ch)
	}
	t.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- e:
		default:
			// The subscriber's buffer is full; this event is dropped for
			// that subscriber. Its next successful receive is simply not
			// contiguous with the one before it, which a live tail view can
			// tolerate in a way a log cannot.
		}
	}
}

// Recent returns the stored events, oldest first.
func (t *Tracker) Recent() []Event {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.full {
		out := make([]Event, t.next)
		copy(out, t.buf[:t.next])
		return out
	}
	out := make([]Event, len(t.buf))
	n := copy(out, t.buf[t.next:])
	copy(out[n:], t.buf[:t.next])
	return out
}

// Subscribe registers a live listener. The returned channel receives every
// event recorded from this point on. cancel unregisters it and closes the
// channel; it must be called once the caller is done watching, or the
// channel and its goroutine-side reader leak for the life of the process.
func (t *Tracker) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	t.mu.Lock()
	t.subs[ch] = struct{}{}
	t.mu.Unlock()

	cancel := func() {
		t.mu.Lock()
		if _, ok := t.subs[ch]; ok {
			delete(t.subs, ch)
			close(ch)
		}
		t.mu.Unlock()
	}
	return ch, cancel
}
