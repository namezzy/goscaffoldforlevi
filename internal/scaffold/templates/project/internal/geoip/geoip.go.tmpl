// Package geoip resolves a client address to the country it geolocates to,
// using a local MaxMind GeoLite2 database.
//
// It is an optional dependency in the same sense Redis is: GeoLite2 is
// licensed data this repository does not ship (MaxMind requires a free
// account to download it — see .env.example), so a deployment that has not
// configured a database path gets a Resolver that still classifies private
// addresses correctly and answers "unknown" for everything else, rather
// than an error at startup.
package geoip

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/oschwald/geoip2-golang"
)

// Location is what a lookup answers: enough to label a request on a
// dashboard, not a precise position. There is no city, latitude or
// longitude — see ADR-0002's spirit applied to a new dependency: this
// package exists for "which country is this traffic from", and a caller
// that needs more than that should reach for a City-tier database directly
// rather than have this port grow a field for every consumer that appears.
type Location struct {
	// CountryCode is the ISO 3166-1 alpha-2 code ("CN", "US"). Empty when
	// unresolved.
	CountryCode string
	// CountryName is localised to the Resolver's configured language where
	// the database carries that locale, English where it does not, and the
	// ISO code as a last resort. "内网" for a private or loopback address —
	// see Private below for why those never reach the database at all.
	CountryName string
	// Private reports a loopback or RFC 1918/4193 address. Regions do not
	// apply to those, and — this is the point — no database lookup is
	// attempted for one: most traffic to a service run locally or inside a
	// cluster network is exactly this, and looking it up would only ever
	// answer "unknown" at the cost of a lookup on every request.
	Private bool
}

// reader is the subset of *geoip2.Reader this package calls, declared so a
// test can supply a fake instead of shipping a real database.
type reader interface {
	Country(net.IP) (*geoip2.Country, error)
	Close() error
}

// Resolver looks up locations. The zero value is not usable; build one with
// Open.
type Resolver struct {
	db   reader // nil when no database was configured — lookups degrade
	lang string
}

// Open builds a Resolver. An empty path is a supported configuration —
// GEOIP_DATABASE_PATH is unset by default — and returns a Resolver that
// still classifies private addresses, with every public one unresolved.
//
// lang selects which locale to read out of the database's name tables
// ("zh-CN", "en", ...); an empty value falls back to "en".
func Open(path, lang string) (*Resolver, error) {
	if lang == "" {
		lang = "en"
	}
	if path == "" {
		return &Resolver{lang: lang}, nil
	}
	db, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open geoip database %s: %w", path, err)
	}
	return &Resolver{db: db, lang: lang}, nil
}

// Close releases the underlying database file. Safe to call on a Resolver
// that was never backed by one.
func (r *Resolver) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// Lookup resolves addr. It never returns an error: a database miss, a
// corrupt record or no database configured are all "unknown", because a
// live request feed has nowhere useful to put a lookup failure and no
// reason to interrupt the request over one.
func (r *Resolver) Lookup(addr netip.Addr) Location {
	if r == nil || !addr.IsValid() {
		return Location{}
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return Location{Private: true, CountryName: "内网"}
	}
	if r.db == nil {
		return Location{}
	}

	record, err := r.db.Country(net.IP(addr.AsSlice()))
	if err != nil || record == nil || record.Country.IsoCode == "" {
		return Location{}
	}

	name := record.Country.Names[r.lang]
	if name == "" {
		name = record.Country.Names["en"]
	}
	if name == "" {
		name = record.Country.IsoCode
	}
	return Location{CountryCode: record.Country.IsoCode, CountryName: name}
}
