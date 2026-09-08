// Package apperr defines the application-wide error model.
//
// Errors carry two orthogonal pieces of information:
//
//   - Kind: a coarse classification that transport layers map to protocol-level
//     status codes. Business code assigns it but never learns what it maps to.
//   - Code: a stable, machine-readable string that is part of the public API
//     contract and lets clients branch on specific failures.
//
// See docs/adr/0003-error-model.md.
package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an error. The zero value is KindInternal so that an
// unclassified error is treated in the most conservative way possible:
// a 500 response with no detail leaked to the client.
type Kind uint8

// The set of error kinds. Adding one requires adding a row to the status
// table in internal/httpx/errmap.go; a test enforces that.
const (
	KindInternal Kind = iota
	KindInvalidArgument
	KindNotFound
	KindUnauthorized
	KindForbidden
	KindConflict
	KindRateLimited
)

// String returns the canonical name of the kind, used in logs and metrics.
func (k Kind) String() string {
	switch k {
	case KindInvalidArgument:
		return "invalid_argument"
	case KindNotFound:
		return "not_found"
	case KindUnauthorized:
		return "unauthorized"
	case KindForbidden:
		return "forbidden"
	case KindConflict:
		return "conflict"
	case KindRateLimited:
		return "rate_limited"
	case KindInternal:
		return "internal"
	default:
		// An out-of-range value means a Kind was added without updating this
		// switch. Reporting it as "internal" would hide that; "unknown" makes
		// it visible in logs and lets tests detect the omission.
		return "unknown"
	}
}

// Detail carries field-level information for validation failures.
type Detail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Error is the application error type.
//
// Message must always be safe to show to an end user. Internal context belongs
// in cause, which is reachable through errors.Is/As and logging but is never
// serialized into a response.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Details []Detail

	cause error
}

// CodeInternal is the only code ever returned to clients for KindInternal.
const CodeInternal = "INTERNAL"

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s(%s): %s: %v", e.Kind, e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s(%s): %s", e.Kind, e.Code, e.Message)
}

// Unwrap exposes the internal cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.cause }

// WithDetails returns a copy of e with the given details attached.
func (e *Error) WithDetails(details ...Detail) *Error {
	clone := *e
	clone.Details = append(append([]Detail(nil), e.Details...), details...)
	return &clone
}

// New builds an error without an underlying cause.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Wrap builds an error that keeps cause reachable for logging and errors.Is.
func Wrap(cause error, kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message, cause: cause}
}

// Invalid reports a request the client can fix, optionally naming the fields.
func Invalid(code, message string, details ...Detail) *Error {
	return &Error{Kind: KindInvalidArgument, Code: code, Message: message, Details: details}
}

// NotFound reports a resource that does not exist or is not visible to the caller.
func NotFound(code, message string) *Error {
	return New(KindNotFound, code, message)
}

// Unauthorized reports a missing or unusable credential.
func Unauthorized(code, message string) *Error {
	return New(KindUnauthorized, code, message)
}

// Forbidden reports a valid credential that lacks the required permission.
func Forbidden(code, message string) *Error {
	return New(KindForbidden, code, message)
}

// Conflict reports a request that contradicts the current state of a resource.
func Conflict(code, message string) *Error {
	return New(KindConflict, code, message)
}

// RateLimited reports a caller that has exceeded its quota.
func RateLimited(code, message string) *Error {
	return New(KindRateLimited, code, message)
}

// Internal wraps an unexpected error. The client-facing message is fixed so
// that no internal detail can leak through this path by accident.
func Internal(cause error) *Error {
	return Wrap(cause, KindInternal, CodeInternal, "internal error")
}

// KindOf reports the Kind of the first *Error in err's chain.
// A nil error has no kind and reports KindInternal; callers should check for
// nil before calling. Any non-nil error that is not an *Error is internal,
// which is the safe default.
func KindOf(err error) Kind {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Kind
	}
	return KindInternal
}

// Is reports whether err's chain contains an *Error of the given kind.
func Is(err error, kind Kind) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Kind == kind
	}
	// A non-*Error is only ever internal.
	return kind == KindInternal && err != nil
}

// From extracts the first *Error in err's chain. If there is none it
// synthesizes an internal error wrapping err, so callers always have a
// well-formed value to render.
func From(err error) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}
