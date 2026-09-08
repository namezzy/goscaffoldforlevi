package httpx_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/httpx"
)

// TestDecodeJSONRequiresAJSONContentType closes a gap found by posting a JSON
// body labelled text/plain and watching it be accepted.
//
// The document says these endpoints consume application/json, and a decoder
// that accepts anything makes that a wish. It also matters for the browser:
// text/plain is one of the three content types a cross-origin form can send
// without a preflight, so accepting it is what makes a form-based cross-site
// request reach the handler at all.
func TestDecodeJSONRequiresAJSONContentType(t *testing.T) {
	t.Parallel()

	const body = `{"name":"ok"}`

	accepted := []string{
		"application/json",
		"application/json; charset=utf-8",
		"application/json;charset=UTF-8",
		"APPLICATION/JSON",
		"application/vnd.example.v1+json",
	}
	for _, ct := range accepted {
		t.Run("accepts "+ct, func(t *testing.T) {
			t.Parallel()

			var got struct {
				Name string `json:"name"`
			}
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			req.Header.Set("Content-Type", ct)
			if err := httpx.DecodeJSON(req, &got); err != nil {
				t.Fatalf("DecodeJSON with %q: %v", ct, err)
			}
			if got.Name != "ok" {
				t.Errorf("name = %q, want %q", got.Name, "ok")
			}
		})
	}

	rejected := []string{
		"text/plain",
		"application/x-www-form-urlencoded",
		"multipart/form-data; boundary=x",
		"text/plain; charset=utf-8",
		"application/xml",
		"", // a body with no declared type is not a JSON body
	}
	for _, ct := range rejected {
		t.Run("rejects "+ct, func(t *testing.T) {
			t.Parallel()

			var got struct {
				Name string `json:"name"`
			}
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			if ct != "" {
				req.Header.Set("Content-Type", ct)
			}
			err := httpx.DecodeJSON(req, &got)
			if err == nil {
				t.Fatalf("DecodeJSON accepted a %q body", ct)
			}
			var appErr *apperr.Error
			if !errors.As(err, &appErr) {
				t.Fatalf("error is not an *apperr.Error: %v", err)
			}
			if appErr.Code != "UNSUPPORTED_MEDIA_TYPE" {
				t.Errorf("code = %q, want UNSUPPORTED_MEDIA_TYPE", appErr.Code)
			}
			if got.Name != "" {
				t.Error("the body was decoded despite the wrong content type")
			}
		})
	}
}
