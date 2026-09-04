// Package user holds the user domain: the entity, its invariants, and the
// service that enforces them. It knows nothing about HTTP or SQL — those live
// in userhttp and userpg respectively, which depend on this package and never
// the other way around.
package user

import (
	"time"

	"github.com/google/uuid"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
)

// ID is a distinct type rather than a bare uuid.UUID so that passing a session
// ID or a tenant ID where a user ID is expected fails to compile instead of
// failing at runtime with a puzzling "not found".
type ID uuid.UUID

// NewID mints a UUIDv7. Generation lives here, not in handlers: an entity must
// have its identity before it reaches the database (ADR-0006), and centralising
// it means the version can be changed in one place.
func NewID() ID { return ID(uuid.Must(uuid.NewV7())) }

// ParseID converts external input into an ID. It rejects anything that is not a
// UUID, so handlers never have to decide what a malformed ID means.
func ParseID(s string) (ID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return ID{}, apperr.Invalid("INVALID_USER_ID", "user id must be a UUID")
	}
	return ID(u), nil
}

func (id ID) String() string { return uuid.UUID(id).String() }

// UUID exposes the underlying value for the persistence layer.
func (id ID) UUID() uuid.UUID { return uuid.UUID(id) }

// IsZero reports whether the ID was never assigned.
func (id ID) IsZero() bool { return uuid.UUID(id) == uuid.Nil }

// MarshalText makes ID render as a plain string in JSON rather than as an array
// of sixteen numbers, which is the default encoding of a [16]byte.
func (id ID) MarshalText() ([]byte, error) { return uuid.UUID(id).MarshalText() }

// UnmarshalText parses the textual form written by MarshalText.
func (id *ID) UnmarshalText(b []byte) error {
	var u uuid.UUID
	if err := u.UnmarshalText(b); err != nil {
		return apperr.Invalid("INVALID_USER_ID", "user id must be a UUID")
	}
	*id = ID(u)
	return nil
}

// Status controls whether a user may authenticate. Disabling is preferred to
// deleting because it is reversible and preserves referential history.
type Status string

// The closed set of account statuses.
const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// Valid reports whether s is one of the defined statuses.
func (s Status) Valid() bool { return s == StatusActive || s == StatusDisabled }

func (s Status) String() string { return string(s) }

// User is the domain entity.
//
// PasswordHash is part of the entity because authentication needs it, but it
// must never leave the process: the HTTP layer maps User onto a response type
// that has no such field, so widening the entity cannot widen the API.
type User struct {
	ID           ID
	Email        string
	PasswordHash string
	Name         string
	Status       Status
	Role         auth.Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CanAuthenticate reports whether this user is allowed to obtain tokens.
func (u User) CanAuthenticate() bool { return u.Status == StatusActive }

// Patch describes a partial update. A nil field means "leave unchanged", which
// is the distinction PATCH requires and that a struct of plain values cannot
// express: there would be no way to tell "set name to empty" apart from "do not
// touch name".
type Patch struct {
	Name         *string
	Status       *Status
	Role         *auth.Role
	PasswordHash *string
}

// IsEmpty reports whether the patch would change nothing.
func (p Patch) IsEmpty() bool {
	return p.Name == nil && p.Status == nil && p.Role == nil && p.PasswordHash == nil
}

// ChangesPrivileges reports whether applying the patch would alter what the
// target user is able to do. Such a change can never be authorised by
// ownership alone, only by an explicit permission.
func (p Patch) ChangesPrivileges() bool { return p.Role != nil || p.Status != nil }

// Page is one slice of a listing together with the totals a client needs in
// order to render pagination controls.
type Page struct {
	Items      []User
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}
