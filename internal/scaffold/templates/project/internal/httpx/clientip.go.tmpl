package httpx

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP resolves the address to attribute a request to.
//
// This is a security boundary, not a convenience. X-Forwarded-For is a request
// header: any client can send one, so trusting it blindly lets an attacker
// spend everyone else's rate-limit quota and forge the address in audit logs by
// sending "X-Forwarded-For: 1.2.3.4".
//
// trustedProxies is the number of reverse proxies that sit in front of this
// service and are known to append to the header. Only that many entries from
// the right are trustworthy, because everything further left was supplied by
// something the deployment does not control. The value defaults to zero, which
// ignores the header entirely — the correct behaviour when nothing is known
// about the topology, and the reason a deployment behind an ingress must set it
// explicitly.
func ClientIP(r *http.Request, trustedProxies int) netip.Addr {
	remote := remoteAddr(r)
	if trustedProxies <= 0 {
		return remote
	}

	forwarded := r.Header.Values("X-Forwarded-For")
	if len(forwarded) == 0 {
		return remote
	}

	// Multiple headers are equivalent to one comma-joined header, and both
	// forms occur in practice.
	var hops []string
	for _, h := range forwarded {
		for _, part := range strings.Split(h, ",") {
			if p := strings.TrimSpace(part); p != "" {
				hops = append(hops, p)
			}
		}
	}
	if len(hops) == 0 {
		return remote
	}

	// The rightmost entry was written by the nearest proxy, so counting from
	// the right walks back through exactly the hops that are trusted. Index
	// len-1-trustedProxies is the address the outermost trusted proxy saw.
	idx := len(hops) - trustedProxies
	if idx < 0 {
		// Fewer hops than expected: the request did not pass through the full
		// chain. Falling back to the peer address is the conservative reading;
		// picking hops[0] would accept a value the client chose.
		return remote
	}
	if idx >= len(hops) {
		idx = len(hops) - 1
	}

	addr, err := parseAddr(hops[idx])
	if err != nil {
		return remote
	}
	return addr
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := parseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr
}

func parseAddr(s string) (netip.Addr, error) {
	// A forwarded entry may carry a port, and IPv6 entries may be bracketed.
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.Trim(s, "[]")
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, err
	}
	// Unmap so that an IPv4-in-IPv6 address and the same IPv4 address share one
	// rate-limit bucket instead of two.
	return addr.Unmap(), nil
}

// ClientIPString renders the client address for use as a rate-limit key. An
// unparseable address yields "unknown", which groups all such requests into a
// single bucket rather than letting them escape limiting.
func ClientIPString(r *http.Request, trustedProxies int) string {
	addr := ClientIP(r, trustedProxies)
	if !addr.IsValid() {
		return "unknown"
	}
	return addr.String()
}
