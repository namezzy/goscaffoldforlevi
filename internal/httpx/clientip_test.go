package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
)

// TestClientIPIgnoresForwardedHeadersByDefault is the most important case here.
// X-Forwarded-For is a request header, so a client can set it to anything; a
// service that trusts it without being told to lets an attacker spend other
// people's rate-limit quota and forge the address in the audit trail.
func TestClientIPIgnoresForwardedHeadersByDefault(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:54321"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")

	if got := httpx.ClientIPString(r, 0); got != "10.0.0.1" {
		t.Errorf("ClientIPString(0) = %q, want the peer address", got)
	}
}

func TestClientIPCountsFromTheRight(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		forwarded      string
		trustedProxies int
		want           string
	}{
		// With one trusted proxy, the rightmost entry is the address that proxy
		// observed; everything left of it was supplied by the client.
		{"one proxy", "9.9.9.9, 1.2.3.4", 1, "1.2.3.4"},
		{"two proxies", "9.9.9.9, 1.2.3.4, 10.0.0.5", 2, "1.2.3.4"},
		// A forged prefix cannot reach the chosen index.
		{"forged prefix ignored", "evil, 203.0.113.7", 1, "203.0.113.7"},
		// Fewer hops than configured means the request did not traverse the
		// expected chain, so the peer address is the only trustworthy value.
		{"short chain falls back", "1.2.3.4", 3, "10.0.0.1"},
		{"unparseable entry falls back", "not-an-ip", 1, "10.0.0.1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = "10.0.0.1:54321"
			r.Header.Set("X-Forwarded-For", tc.forwarded)

			if got := httpx.ClientIPString(r, tc.trustedProxies); got != tc.want {
				t.Errorf("ClientIPString(%d) = %q, want %q", tc.trustedProxies, got, tc.want)
			}
		})
	}
}

func TestClientIPHandlesRepeatedHeaders(t *testing.T) {
	t.Parallel()

	// Multiple header lines are equivalent to one comma-joined line, and both
	// forms occur in practice; treating only the last line would let a client
	// hide hops by splitting them.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:54321"
	r.Header.Add("X-Forwarded-For", "9.9.9.9")
	r.Header.Add("X-Forwarded-For", "1.2.3.4")

	if got := httpx.ClientIPString(r, 1); got != "1.2.3.4" {
		t.Errorf("ClientIPString(1) = %q, want 1.2.3.4", got)
	}
}

func TestClientIPNormalisesAddresses(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		remote string
		want   string
	}{
		"ipv6 with port":      {"[2001:db8::1]:443", "2001:db8::1"},
		"ipv4 mapped to ipv6": {"[::ffff:192.0.2.1]:443", "192.0.2.1"},
		"no port":             {"192.0.2.9", "192.0.2.9"},
		"unparseable":         {"garbage", "unknown"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remote

			// An IPv4-mapped address and the plain IPv4 address must share one
			// bucket, or a client could double its quota by switching form.
			if got := httpx.ClientIPString(r, 0); got != tc.want {
				t.Errorf("ClientIPString() = %q, want %q", got, tc.want)
			}
		})
	}
}
