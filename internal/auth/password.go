package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/example/go-enterprise/internal/apperr"
)

// Password length bounds (ADR-0007).
//
// The minimum is 12 with no character-class rules. Composition rules push users
// toward predictable substitutions ("Password1!") that add almost no entropy
// while guaranteeing the password is hard to type; length is the only knob that
// reliably buys work factor.
//
// The maximum exists purely as a denial-of-service bound: Argon2id hashes the
// whole input, so an unbounded password is an unbounded amount of work.
const (
	MinPasswordLen = 12
	MaxPasswordLen = 128
)

// Argon2Params are the cost parameters. They are stored inside every hash, so
// changing them here only affects new and re-hashed passwords; existing users
// keep verifying against the parameters they were created with.
type Argon2Params struct {
	Time    uint32
	Memory  uint32 // KiB
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

// DefaultArgon2Params follows the OWASP recommendation of 64 MiB / t=3 / p=4.
//
// Memory is the parameter that matters against GPU and ASIC attackers, which is
// why it is set high rather than trading it for more iterations. The cost is
// that every login allocates 64 MiB — the reason login rate limiting is not
// optional in this codebase (ADR-0008).
var DefaultArgon2Params = Argon2Params{
	Time:    3,
	Memory:  64 * 1024,
	Threads: 4,
	SaltLen: 16,
	KeyLen:  32,
}

// Validate rejects parameters that would silently weaken hashing.
func (p Argon2Params) Validate() error {
	switch {
	case p.Time < 1:
		return errors.New("argon2: time must be at least 1")
	case p.Memory < 8*1024:
		return errors.New("argon2: memory must be at least 8192 KiB")
	case p.Threads < 1:
		return errors.New("argon2: threads must be at least 1")
	case p.SaltLen < 16:
		return errors.New("argon2: salt length must be at least 16 bytes")
	case p.KeyLen < 32:
		return errors.New("argon2: key length must be at least 32 bytes")
	}
	return nil
}

// weakerThan reports whether a stored hash was produced with less work than the
// current configuration, which is the signal to transparently re-hash on login.
func (p Argon2Params) weakerThan(current Argon2Params) bool {
	return p.Time < current.Time ||
		p.Memory < current.Memory ||
		p.KeyLen < current.KeyLen
}

// Argon2Hasher hashes and verifies passwords.
type Argon2Hasher struct {
	params Argon2Params
	// dummyHash is verified when no account matches, so that a request for a
	// non-existent email costs the same as one for a real account. Without it,
	// response time alone enumerates the user base.
	dummyHash string
}

// NewArgon2Hasher builds a hasher, failing fast on parameters that would be
// weaker than the floor rather than quietly accepting them.
func NewArgon2Hasher(p Argon2Params) (*Argon2Hasher, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	h := &Argon2Hasher{params: p}

	// The dummy hash is computed once at startup from a random password nobody
	// knows. Computing it per request would double the cost of every failed
	// login; hard-coding it would make the comparison a known constant.
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("argon2: generate dummy secret: %w", err)
	}
	dummy, err := h.Hash(string(secret))
	if err != nil {
		return nil, err
	}
	h.dummyHash = dummy
	return h, nil
}

// Hash produces a PHC-encoded Argon2id hash.
//
// The encoding carries the algorithm, version, parameters and salt alongside
// the digest. That is what makes per-user parameter upgrades possible: the
// verifier reads the cost from the stored value instead of assuming today's.
func (h *Argon2Hasher) Hash(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	salt := make([]byte, h.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2: generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.params.Memory, h.params.Time, h.params.Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyResult reports the outcome of a verification.
type VerifyResult struct {
	OK bool
	// NeedsRehash is true when the password was correct but was stored with
	// weaker parameters than the current configuration. Login is the only
	// moment the plaintext is available, so it is the only moment an upgrade
	// can happen.
	NeedsRehash bool
}

// Verify checks a password against a stored PHC hash.
//
// A malformed stored hash reports failure rather than an error: it is
// indistinguishable from a wrong password to the caller, which keeps corrupted
// rows from becoming an authentication bypass or a distinguishable response.
func (h *Argon2Hasher) Verify(password, encoded string) VerifyResult {
	params, salt, want, err := decodePHC(encoded)
	if err != nil {
		return VerifyResult{}
	}

	got := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return VerifyResult{}
	}
	return VerifyResult{OK: true, NeedsRehash: params.weakerThan(h.params)}
}

// VerifyDummy burns the same amount of work as a real verification. It is
// called when no account matches the supplied identifier so that "unknown
// email" and "wrong password" take indistinguishable time.
//
// The result is discarded on purpose; the call exists only for its cost.
func (h *Argon2Hasher) VerifyDummy() {
	res := h.Verify("this password is never correct", h.dummyHash)
	// Keep the compiler and the linker from optimising the work away.
	runtime.KeepAlive(res)
}

// Params exposes the configured cost, for logging and tests.
func (h *Argon2Hasher) Params() Argon2Params { return h.params }

// ValidatePassword enforces the length policy.
func ValidatePassword(password string) error {
	// Length is counted in bytes, not runes, because the byte length is what
	// bounds the hashing work.
	switch {
	case len(password) < MinPasswordLen:
		return apperr.Invalid("WEAK_PASSWORD",
			fmt.Sprintf("password must be at least %d characters", MinPasswordLen),
			apperr.Detail{Field: "password", Reason: "too short"})
	case len(password) > MaxPasswordLen:
		return apperr.Invalid("WEAK_PASSWORD",
			fmt.Sprintf("password must be at most %d characters", MaxPasswordLen),
			apperr.Detail{Field: "password", Reason: "too long"})
	}
	return nil
}

var errBadPHC = errors.New("argon2: malformed hash")

// decodePHC parses "$argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>".
func decodePHC(encoded string) (p Argon2Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	// A leading "$" yields an empty first field, so a well-formed value has six.
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return p, nil, nil, errBadPHC
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, errBadPHC
	}
	// Refusing an unknown version is required for correctness, not just
	// hygiene: argon2 changed its encoding between versions, so verifying a v16
	// hash with a v19 implementation silently produces the wrong digest.
	if version != argon2.Version {
		return p, nil, nil, errBadPHC
	}

	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return p, nil, nil, errBadPHC
	}

	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return p, nil, nil, errBadPHC
	}
	if key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		return p, nil, nil, errBadPHC
	}
	// The upper bound is what makes the conversions below safe, and it is a
	// sanity check in its own right: a legitimate Argon2 salt or key is tens of
	// bytes, so anything larger is a corrupted or hostile value.
	const maxComponentLen = 1024
	if len(salt) == 0 || len(key) == 0 || len(salt) > maxComponentLen || len(key) > maxComponentLen {
		return p, nil, nil, errBadPHC
	}

	p.SaltLen = uint32(len(salt)) //nolint:gosec // bounded by maxComponentLen above
	p.KeyLen = uint32(len(key))   //nolint:gosec // bounded by maxComponentLen above
	return p, salt, key, nil
}
