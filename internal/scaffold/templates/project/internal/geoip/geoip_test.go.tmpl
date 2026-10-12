package geoip

import (
	"net"
	"net/netip"
	"testing"

	"github.com/oschwald/geoip2-golang"
)

// fakeReader stands in for *geoip2.Reader so private-address short-circuiting
// and locale fallback can be tested without a real MaxMind database file.
type fakeReader struct {
	called  bool
	country *geoip2.Country
	err     error
}

func (f *fakeReader) Country(net.IP) (*geoip2.Country, error) {
	f.called = true
	return f.country, f.err
}

func (f *fakeReader) Close() error { return nil }

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", s, err)
	}
	return addr
}

func TestPrivateAndLoopbackAddressesNeverReachTheDatabase(t *testing.T) {
	t.Parallel()

	cases := []string{
		"127.0.0.1", // loopback
		"::1",       // loopback, IPv6
		"10.0.0.5",  // RFC 1918
		"192.168.1.1",
		"172.16.0.1",
		"169.254.1.1", // link-local
	}

	for _, ip := range cases {
		t.Run(ip, func(t *testing.T) {
			t.Parallel()

			fake := &fakeReader{}
			r := &Resolver{db: fake, lang: "en"}
			loc := r.Lookup(mustAddr(t, ip))

			if fake.called {
				t.Errorf("Lookup(%s) queried the database; private addresses must short-circuit", ip)
			}
			if !loc.Private {
				t.Errorf("Lookup(%s).Private = false, want true", ip)
			}
			if loc.CountryName != "内网" {
				t.Errorf("Lookup(%s).CountryName = %q, want 内网", ip, loc.CountryName)
			}
		})
	}
}

func TestNoDatabaseConfiguredAnswersUnknownForPublicAddresses(t *testing.T) {
	t.Parallel()

	r, err := Open("", "zh-CN")
	if err != nil {
		t.Fatalf("Open(\"\") = %v, want no error — an empty path is a supported configuration", err)
	}
	defer r.Close()

	loc := r.Lookup(mustAddr(t, "8.8.8.8"))
	if loc.CountryCode != "" || loc.CountryName != "" || loc.Private {
		t.Errorf("Lookup() with no database = %+v, want a zero Location", loc)
	}
}

func TestLookupPrefersTheConfiguredLocale(t *testing.T) {
	t.Parallel()

	fake := &fakeReader{country: &geoip2.Country{}}
	fake.country.Country.IsoCode = "CN"
	fake.country.Country.Names = map[string]string{"en": "China", "zh-CN": "中国"}

	r := &Resolver{db: fake, lang: "zh-CN"}
	loc := r.Lookup(mustAddr(t, "1.2.3.4"))

	if loc.CountryCode != "CN" {
		t.Errorf("CountryCode = %q, want CN", loc.CountryCode)
	}
	if loc.CountryName != "中国" {
		t.Errorf("CountryName = %q, want 中国 (the configured locale)", loc.CountryName)
	}
	if loc.Private {
		t.Error("a public address was reported as Private")
	}
}

func TestLookupFallsBackToEnglishThenToTheISOCode(t *testing.T) {
	t.Parallel()

	t.Run("falls back to english when the locale is missing", func(t *testing.T) {
		t.Parallel()
		fake := &fakeReader{country: &geoip2.Country{}}
		fake.country.Country.IsoCode = "US"
		fake.country.Country.Names = map[string]string{"en": "United States"}

		r := &Resolver{db: fake, lang: "zh-CN"}
		loc := r.Lookup(mustAddr(t, "1.2.3.4"))
		if loc.CountryName != "United States" {
			t.Errorf("CountryName = %q, want the English fallback", loc.CountryName)
		}
	})

	t.Run("falls back to the ISO code when no name is available", func(t *testing.T) {
		t.Parallel()
		fake := &fakeReader{country: &geoip2.Country{}}
		fake.country.Country.IsoCode = "FR"

		r := &Resolver{db: fake, lang: "zh-CN"}
		loc := r.Lookup(mustAddr(t, "1.2.3.4"))
		if loc.CountryName != "FR" {
			t.Errorf("CountryName = %q, want the ISO code fallback", loc.CountryName)
		}
	})
}

func TestLookupIsUnknownWhenTheRecordHasNoCountry(t *testing.T) {
	t.Parallel()

	// A real database answers this for addresses it has no data for
	// (unassigned ranges, some anycast space) rather than an error.
	fake := &fakeReader{country: &geoip2.Country{}}
	r := &Resolver{db: fake, lang: "en"}

	loc := r.Lookup(mustAddr(t, "1.2.3.4"))
	if loc.CountryCode != "" || loc.CountryName != "" {
		t.Errorf("Lookup() = %+v, want a zero Location for an empty record", loc)
	}
}

func TestLookupIsUnknownOnADatabaseError(t *testing.T) {
	t.Parallel()

	fake := &fakeReader{err: net.ErrClosed}
	r := &Resolver{db: fake, lang: "en"}

	loc := r.Lookup(mustAddr(t, "1.2.3.4"))
	if loc.CountryCode != "" || loc.CountryName != "" {
		t.Errorf("Lookup() = %+v, want a zero Location rather than propagating the error", loc)
	}
}

func TestLookupOnAnInvalidAddressIsUnknown(t *testing.T) {
	t.Parallel()

	fake := &fakeReader{}
	r := &Resolver{db: fake, lang: "en"}

	loc := r.Lookup(netip.Addr{})
	if fake.called {
		t.Error("an invalid address reached the database")
	}
	if loc != (Location{}) {
		t.Errorf("Lookup(invalid) = %+v, want the zero Location", loc)
	}
}

func TestCloseIsSafeWithoutADatabase(t *testing.T) {
	t.Parallel()

	var r *Resolver
	if err := r.Close(); err != nil {
		t.Errorf("Close() on a nil Resolver = %v, want nil", err)
	}

	empty := &Resolver{}
	if err := empty.Close(); err != nil {
		t.Errorf("Close() on a Resolver without a database = %v, want nil", err)
	}
}
