package user

import (
	"context"

	"github.com/example/go-enterprise/internal/auth"
)

// AccountStore adapts the user repository to the narrow view authentication
// needs.
//
// The adapter lives here, on the user side of the boundary, because this is the
// package that owns user storage. Putting it in auth would mean auth had to
// know how users are persisted, and the dependency would run both ways.
type AccountStore struct {
	repo Repository
}

// NewAccountStore returns an auth.AccountStore backed by the user repository.
func NewAccountStore(repo Repository) *AccountStore { return &AccountStore{repo: repo} }

var _ auth.AccountStore = (*AccountStore)(nil)

// FindByEmail looks up an account for login. The caller has already normalised
// the address; doing it again here would hide a mismatch rather than fix it.
func (s *AccountStore) FindByEmail(ctx context.Context, email string) (auth.Account, error) {
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return auth.Account{}, err
	}
	return toAccount(u), nil
}

// FindByID looks up an account during refresh rotation.
func (s *AccountStore) FindByID(ctx context.Context, userID string) (auth.Account, error) {
	id, err := ParseID(userID)
	if err != nil {
		return auth.Account{}, err
	}
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return auth.Account{}, err
	}
	return toAccount(u), nil
}

// UpdatePasswordHash writes back a hash that was upgraded during login.
//
// It goes through the repository's patch path so that the update passes the
// same constraints as any other write; there is no second way to modify a user.
func (s *AccountStore) UpdatePasswordHash(ctx context.Context, userID, hash string) error {
	id, err := ParseID(userID)
	if err != nil {
		return err
	}
	_, err = s.repo.Update(ctx, id, Patch{PasswordHash: &hash})
	return err
}

func toAccount(u User) auth.Account {
	return auth.Account{
		UserID:       u.ID.String(),
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		Active:       u.CanAuthenticate(),
	}
}
