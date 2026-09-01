package user

import (
	"context"
	"log/slog"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/validate"
)

// Repository is the persistence port.
//
// It is defined here, next to its consumer, and lists exactly the operations
// this service performs. A generic Repository[T] with Find/Save/Delete would
// hide which of those the service actually needs, force every implementation to
// support operations nobody calls, and make the compiler useless as a guide
// when a query has to change.
type Repository interface {
	Create(ctx context.Context, u User) (User, error)
	GetByID(ctx context.Context, id ID) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	List(ctx context.Context, limit, offset int32) ([]User, error)
	Count(ctx context.Context) (int64, error)
	Update(ctx context.Context, id ID, p Patch) (User, error)
	Delete(ctx context.Context, id ID) error
}

// PasswordHasher is the hashing port. Declaring it here rather than importing a
// concrete hasher keeps the service's tests fast: a real Argon2id hash costs
// 64 MiB and tens of milliseconds per call.
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// Pagination bounds (ADR-0004). The maximum exists so that a client cannot turn
// one request into a full table scan by asking for a page of a million.
const (
	DefaultPageSize = 20
	MaxPageSize     = 100

	// maxOffset is the largest OFFSET this service will issue. It is far above
	// any realistic table size and far below the int32 limit, which is what
	// makes the conversion in List provably safe.
	maxOffset = 1 << 30
)

// Service holds the user use cases.
type Service struct {
	repo   Repository
	hasher PasswordHasher
	log    *slog.Logger
}

// NewService wires the service.
func NewService(repo Repository, hasher PasswordHasher, log *slog.Logger) *Service {
	return &Service{repo: repo, hasher: hasher, log: log}
}

// CreateInput is the validated input for registration.
type CreateInput struct {
	Email    string
	Name     string
	Password string
	Role     auth.Role
}

// Create registers a user.
//
// Uniqueness is not pre-checked with a SELECT. Between the check and the insert
// there is a window in which another request can insert the same email, and
// under concurrent signup it is reliably lost. The unique index decides, and
// the repository turns its violation into a conflict.
func (s *Service) Create(ctx context.Context, in CreateInput) (User, error) {
	email := validate.NormalizeEmail(in.Email)

	var v validate.Violations
	v.Required("email", email)
	v.Email("email", email)
	v.Required("name", in.Name)
	v.RuneLen("name", in.Name, 1, 100)
	v.NoControlChars("name", in.Name)
	if !in.Role.Valid() {
		v.Add("role", "must be one of: admin, user")
	}
	if v.Any() {
		return User{}, v.Err("INVALID_ARGUMENT", "invalid user")
	}
	// The password is validated by the hasher, which owns the policy, but it is
	// checked before hashing so that an over-long password is rejected without
	// paying for the hash.
	if err := auth.ValidatePassword(in.Password); err != nil {
		return User{}, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return User{}, apperr.Internal(err)
	}

	return s.repo.Create(ctx, User{
		ID:           NewID(),
		Email:        email,
		PasswordHash: hash,
		Name:         in.Name,
		Status:       StatusActive,
		Role:         in.Role,
	})
}

// Get returns one user.
//
// Authorization is checked here rather than in middleware because it depends on
// who the record belongs to, and that is only knowable from the request's
// target — see auth.AuthorizeSelfOrPermission.
func (s *Service) Get(ctx context.Context, id ID) (User, error) {
	if err := auth.AuthorizeSelfOrPermission(ctx, id.String(), auth.PermUserRead); err != nil {
		return User{}, err
	}
	return s.repo.GetByID(ctx, id)
}

// List returns a page of users. There is no self-service variant: listing is an
// administrative capability, so it is a plain permission check.
func (s *Service) List(ctx context.Context, page, pageSize int) (Page, error) {
	if err := auth.Authorize(ctx, auth.PermUserList); err != nil {
		return Page{}, err
	}

	page, pageSize = normalizePaging(page, pageSize)

	total, err := s.repo.Count(ctx)
	if err != nil {
		return Page{}, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	// The conversions below are safe because both values have just been
	// clamped: pageSize is at most MaxPageSize, and page is bounded so that the
	// offset cannot exceed maxOffset.
	//
	// The bound is checked by division rather than by comparing
	// page*pageSize against the limit. That multiplication overflows for a
	// large page — page=math.MaxInt with pageSize=20 wraps to -20, which is
	// below any limit — so the guard against the overflow was itself the
	// overflow, and a negative OFFSET reached PostgreSQL. Dividing cannot
	// overflow, because pageSize is at least 1 here.
	if page > maxOffset/pageSize {
		return Page{
			Items: []User{}, Page: page, PageSize: pageSize,
			Total: total, TotalPages: totalPages,
		}, nil
	}
	offset := int32((page - 1) * pageSize)                  //nolint:gosec // bounded above
	items, err := s.repo.List(ctx, int32(pageSize), offset) //nolint:gosec // <= MaxPageSize
	if err != nil {
		return Page{}, err
	}

	return Page{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// normalizePaging clamps client input instead of rejecting it. A page size of
// 1000 is a reasonable thing to ask for and an unreasonable thing to serve; an
// error would be pedantic, while silently serving it would be a denial of
// service.
func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}
	return page, pageSize
}

// Update applies a partial update.
func (s *Service) Update(ctx context.Context, id ID, p Patch) (User, error) {
	// Ownership is enough to change your own name, but never enough to change
	// your own role or status: that would be self-service privilege escalation.
	// Those fields require the permission that covers editing anyone.
	if p.ChangesPrivileges() {
		if err := auth.Authorize(ctx, auth.PermUserUpdate); err != nil {
			return User{}, err
		}
	} else if err := auth.AuthorizeSelfOrPermission(ctx, id.String(), auth.PermUserUpdate); err != nil {
		return User{}, err
	}

	if p.IsEmpty() {
		return User{}, apperr.Invalid("EMPTY_PATCH", "the request changes nothing")
	}

	var v validate.Violations
	if p.Name != nil {
		v.Required("name", *p.Name)
		v.RuneLen("name", *p.Name, 1, 100)
		v.NoControlChars("name", *p.Name)
	}
	if p.Status != nil && !p.Status.Valid() {
		v.Add("status", "must be one of: active, disabled")
	}
	if p.Role != nil && !p.Role.Valid() {
		v.Add("role", "must be one of: admin, user")
	}
	if v.Any() {
		return User{}, v.Err("INVALID_ARGUMENT", "invalid user patch")
	}

	return s.repo.Update(ctx, id, p)
}

// ChangePassword sets a new password.
//
// It is separate from Update because it has a different authorization rule and
// a different input: the plaintext must never travel through the generic patch
// path, where it could end up logged alongside the other fields.
func (s *Service) ChangePassword(ctx context.Context, id ID, newPassword string) error {
	if err := auth.AuthorizeSelfOrPermission(ctx, id.String(), auth.PermUserUpdate); err != nil {
		return err
	}
	if err := auth.ValidatePassword(newPassword); err != nil {
		return err
	}

	hash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return apperr.Internal(err)
	}
	_, err = s.repo.Update(ctx, id, Patch{PasswordHash: &hash})
	return err
}

// Delete removes a user. Unlike read and update, this is not available through
// ownership: self-deletion is a product decision that has not been made, and
// defaulting to allowing it would be irreversible.
func (s *Service) Delete(ctx context.Context, id ID) error {
	if err := auth.Authorize(ctx, auth.PermUserDelete); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}
