// Package httpx holds the HTTP transport: server, router, middleware, and the
// translation between the application error model and HTTP.
//
// It is the only package permitted to know the Kind -> status code mapping.
package httpx

import (
	"net/http"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
)

// statusByKind is the single source of truth for translating an error Kind to
// an HTTP status code.
//
// Keeping this table in one file is the point of the Kind abstraction: if
// status codes were chosen at each handler, "which errors return 409?" would
// become an archaeology exercise. Adding a Kind without adding a row here is
// caught by TestStatusByKindIsExhaustive.
//
// 503 and 504 are absent on purpose. They describe the state of the transport
// (dependency unavailable, deadline exceeded), not a classification of a
// business error, and are written directly by the health handler and the
// timeout middleware.
var statusByKind = map[apperr.Kind]int{
	apperr.KindInternal:        http.StatusInternalServerError,
	apperr.KindInvalidArgument: http.StatusBadRequest,
	apperr.KindNotFound:        http.StatusNotFound,
	apperr.KindUnauthorized:    http.StatusUnauthorized,
	apperr.KindForbidden:       http.StatusForbidden,
	apperr.KindConflict:        http.StatusConflict,
	apperr.KindRateLimited:     http.StatusTooManyRequests,
}

// StatusOf returns the HTTP status for err.
//
// An unclassified error yields 500: the safe default is to reveal nothing.
func StatusOf(err error) int {
	if status, ok := statusByKind[apperr.KindOf(err)]; ok {
		return status
	}
	return http.StatusInternalServerError
}
