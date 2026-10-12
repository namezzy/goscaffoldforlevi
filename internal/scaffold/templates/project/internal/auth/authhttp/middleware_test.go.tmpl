package authhttp_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authhttp"
)

// stubVerifier accepts exactly one token. Using a stub rather than the real
// signer keeps these tests about the middleware: token validation itself is
// covered in the authjwt package, and duplicating it here would mean two places
// to change when the claims change.
type stubVerifier struct {
	token  string
	claims auth.AccessClaims
}

func (s stubVerifier) Verify(token string) (auth.AccessClaims, error) {
	if token != s.token {
		return auth.AccessClaims{}, auth.ErrInvalidToken()
	}
	return s.claims, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestAuthenticateRejectsAnythingButAValidBearerToken covers the header parsing
// as a table, because every accepted-but-wrong shape here is a request reaching
// a handler without a caller.
func TestAuthenticateRejectsAnythingButAValidBearerToken(t *testing.T) {
	t.Parallel()

	const good = "good-token"
	verifier := stubVerifier{token: good, claims: auth.AccessClaims{Subject: "u1", Role: auth.RoleReadOnly}}

	cases := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"scheme only", "Bearer", http.StatusUnauthorized},
		{"empty credentials", "Bearer   ", http.StatusUnauthorized},
		{"wrong scheme", "Basic " + good, http.StatusUnauthorized},
		{"raw token without a scheme", good, http.StatusUnauthorized},
		{"unknown token", "Bearer nonsense", http.StatusUnauthorized},
		// RFC 7235 makes the scheme case-insensitive, and clients send both.
		{"lowercase scheme", "bearer " + good, http.StatusOK},
		{"canonical", "Bearer " + good, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reached := false
			h := authhttp.Authenticate(verifier, discardLogger())(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					reached = true
					w.WriteHeader(http.StatusOK)
				}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if reached != (tc.wantStatus == http.StatusOK) {
				t.Errorf("handler reached = %v, which does not match status %d", reached, rec.Code)
			}
		})
	}
}

func TestAuthenticateAttachesThePrincipal(t *testing.T) {
	t.Parallel()

	claims := auth.AccessClaims{Subject: "user-123", Role: auth.RoleSystemAdmin, TokenID: "jti-1"}
	verifier := stubVerifier{token: "t", claims: claims}

	var got auth.Principal
	h := authhttp.Authenticate(verifier, discardLogger())(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got, _ = auth.PrincipalFrom(r.Context())
		}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer t")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got.UserID != claims.Subject || got.Role != claims.Role || got.TokenID != claims.TokenID {
		t.Errorf("principal = %+v, want it to carry the claims", got)
	}
}

// TestRejectionsDoNotExplainWhy checks that every rejected token produces the
// identical response. A message that distinguishes "expired" from "bad
// signature" tells someone probing the endpoint how far their forgery got.
//
// The shipped message, "invalid or expired token", names both causes precisely
// so that it can name neither.
func TestRejectionsDoNotExplainWhy(t *testing.T) {
	t.Parallel()

	verifier := stubVerifier{token: "t"}
	h := authhttp.Authenticate(verifier, discardLogger())(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	reject := func(header string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		var parsed struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("decode: %v (%s)", err, rec.Body)
		}
		return rec.Code, parsed.Error.Code + ": " + parsed.Error.Message
	}

	headers := []string{"Bearer forged", "Bearer expired-looking-token", "Bearer " + strings.Repeat("a", 512)}
	wantStatus, wantBody := reject(headers[0])
	for _, header := range headers[1:] {
		status, body := reject(header)
		if status != wantStatus || body != wantBody {
			t.Errorf("header %q gave (%d, %q), want (%d, %q)", header, status, body, wantStatus, wantBody)
		}
	}

	// Nothing in the response may echo the credential back to the caller, which
	// would put it into any proxy log that records response bodies.
	if strings.Contains(wantBody, "forged") {
		t.Errorf("the rejection echoes the token: %s", wantBody)
	}
}

func TestRequirePermission(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		principal *auth.Principal
		want      int
	}{
		{"unauthenticated is 401", nil, http.StatusUnauthorized},
		// RoleOperator, not RoleReadOnly: ReadOnly holds PermUserList, which
		// would defeat the point of this case.
		{"wrong role is 403", &auth.Principal{UserID: "u", Role: auth.RoleOperator}, http.StatusForbidden},
		{"right role passes", &auth.Principal{UserID: "u", Role: auth.RoleSystemAdmin}, http.StatusOK},
		// An unrecognised role must be treated as having no permissions rather
		// than as an error to investigate later.
		{"unknown role is 403", &auth.Principal{UserID: "u", Role: auth.Role("superuser")}, http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := authhttp.RequirePermission(auth.PermUserList, discardLogger())(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.principal != nil {
				req = req.WithContext(auth.ContextWithPrincipal(req.Context(), *tc.principal))
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}
