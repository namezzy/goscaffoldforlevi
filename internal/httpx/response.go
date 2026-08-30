package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/example/go-enterprise/internal/apperr"
	"github.com/example/go-enterprise/internal/telemetry"
)

// errorBody is the wire format for failures. Successful responses carry the
// resource directly with no envelope (see ADR-0004); only errors are wrapped,
// because a client needs a predictable place to look when things go wrong.
type errorBody struct {
	Error     errorDetail `json:"error"`
	RequestID string      `json:"request_id,omitempty"`
}

type errorDetail struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details []apperr.Detail `json:"details,omitempty"`
}

// WriteJSON writes v as JSON with the given status.
//
// A marshalling failure after the header is written cannot be turned into an
// error response, so it is logged instead. Passing a nil body writes the
// status with no content, which is what 204 responses need.
func WriteJSON(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, status int, v any) {
	if v == nil {
		w.WriteHeader(status)
		return
	}

	// Marshal before writing the header so that an encoding failure can still
	// produce a 500 instead of a truncated 200.
	buf, err := json.Marshal(v)
	if err != nil {
		logger.ErrorContext(ctx, "marshal response body", slog.Any("error", err))
		WriteError(ctx, w, logger, apperr.Internal(err))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(buf); err != nil {
		// The client went away mid-response. Nothing to do but record it.
		logger.WarnContext(ctx, "write response body", slog.Any("error", err))
	}
}

// WriteError translates err into an HTTP response.
//
// Internal errors are logged with their full cause chain and answered with a
// fixed, information-free body: the request id is the only link between what
// the client sees and what operators can look up.
func WriteError(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, err error) {
	status := StatusOf(err)
	appErr := apperr.From(err)

	body := errorBody{
		Error: errorDetail{
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		},
		RequestID: telemetry.RequestIDFromContext(ctx),
	}

	if status >= http.StatusInternalServerError {
		logger.ErrorContext(ctx, "request failed",
			slog.Int("status", status),
			slog.String("code", appErr.Code),
			slog.Any("error", err),
		)
		body.Error = errorDetail{Code: apperr.CodeInternal, Message: "internal error"}
	} else {
		logger.InfoContext(ctx, "request rejected",
			slog.Int("status", status),
			slog.String("code", appErr.Code),
		)
	}

	buf, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		// errorBody is a fixed struct of serialisable fields; reaching here
		// means Details carried something unexpected. Fall back to a literal.
		logger.ErrorContext(ctx, "marshal error body", slog.Any("error", marshalErr))
		buf = []byte(`{"error":{"code":"INTERNAL","message":"internal error"}}`)
		status = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, writeErr := w.Write(buf); writeErr != nil {
		logger.WarnContext(ctx, "write error body", slog.Any("error", writeErr))
	}
}

// DecodeJSON reads a JSON request body into v.
//
// Unknown fields are rejected: silently ignoring a misspelled field turns a
// client bug into a confusing "the update did nothing" report. The body is
// assumed to be already size-limited by the BodyLimit middleware.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			return apperr.Invalid("REQUEST_BODY_EMPTY", "request body is required")
		case errors.As(err, &maxErr):
			return apperr.Invalid("REQUEST_BODY_TOO_LARGE", "request body is too large")
		default:
			// json error text can echo the payload, so it is not surfaced.
			return apperr.Wrap(err, apperr.KindInvalidArgument,
				"REQUEST_BODY_INVALID", "request body is not valid JSON")
		}
	}

	// A second value means the client sent more than one document.
	if dec.More() {
		return apperr.Invalid("REQUEST_BODY_INVALID", "request body must contain a single JSON object")
	}
	return nil
}
