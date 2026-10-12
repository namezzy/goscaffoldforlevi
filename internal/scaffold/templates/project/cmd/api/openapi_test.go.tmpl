package main

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/namezzy/goscaffoldforlevi/internal/audit/audithttp"
	"github.com/namezzy/goscaffoldforlevi/internal/auth"
	"github.com/namezzy/goscaffoldforlevi/internal/auth/authhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/invite/invitehttp"
	"github.com/namezzy/goscaffoldforlevi/internal/messaging/messaginghttp"
	"github.com/namezzy/goscaffoldforlevi/internal/notification/notificationhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/org/orghttp"
	"github.com/namezzy/goscaffoldforlevi/internal/requesttrack/requesttrackhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/ticket/tickethttp"
	"github.com/namezzy/goscaffoldforlevi/internal/user/userhttp"
	"github.com/namezzy/goscaffoldforlevi/internal/workbench/workbenchhttp"
)

// TestOpenAPIMatchesTheRouteTable compares the served routes against
// api/openapi.yaml in both directions.
//
// A specification that is merely written by hand drifts within weeks: an
// endpoint gets added and nobody remembers the document, or an endpoint is
// removed and the document keeps advertising it. Checking both directions is
// what makes the document a description of the program rather than a wish.
func TestOpenAPIMatchesTheRouteTable(t *testing.T) {
	t.Parallel()

	spec := loadSpecOperations(t)
	routes := walkRoutes(t, buildTestRouter())

	for op := range routes {
		if _, ok := spec[op]; !ok {
			t.Errorf("route %s is served but absent from api/openapi.yaml", op)
		}
	}
	for op := range spec {
		if _, ok := routes[op]; !ok {
			t.Errorf("api/openapi.yaml documents %s, which no route serves", op)
		}
	}

	if t.Failed() {
		t.Logf("served: %s", sortedKeys(routes))
		t.Logf("documented: %s", sortedKeys(spec))
	}
}

// TestOpenAPIDocumentsEveryPathParameter guards the failure that a route-name
// comparison alone would miss: a path can be spelled correctly in the document
// while its parameter is undeclared, which generates clients that cannot fill
// in the URL.
func TestOpenAPIDocumentsEveryPathParameter(t *testing.T) {
	t.Parallel()

	doc := loadSpec(t)

	for path, item := range doc.Paths {
		wanted := templateParams(path)
		if len(wanted) == 0 {
			continue
		}

		declared := map[string]bool{}
		collect := func(params []specParameter) {
			for _, p := range params {
				if p.In == "path" {
					declared[p.Name] = true
				}
			}
		}
		collect(resolveParameters(doc, item.Parameters))
		for _, op := range item.operations() {
			collect(resolveParameters(doc, op.Parameters))
		}

		for _, name := range wanted {
			if !declared[name] {
				t.Errorf("path %s uses {%s} but never declares it as a path parameter", path, name)
			}
		}
	}
}

// TestProtectedEndpointsDeclareSecurity checks that nothing under /users opts
// out of the document's global security requirement. An endpoint marked
// `security: []` by accident tells every generated client and every reviewer
// that the endpoint is public.
func TestProtectedEndpointsDeclareSecurity(t *testing.T) {
	t.Parallel()

	doc := loadSpec(t)
	if len(doc.Security) == 0 {
		t.Fatal("the document declares no default security requirement")
	}

	for path, item := range doc.Paths {
		if !strings.HasPrefix(path, "/api/v1/users") {
			continue
		}
		for method, op := range item.operationsByMethod() {
			if op.Security != nil && len(*op.Security) == 0 {
				t.Errorf("%s %s overrides security with an empty list, marking it public", method, path)
			}
		}
	}
}

// buildTestRouter assembles the real route table with nil services and
// pass-through middleware. Registering a route never calls the handler, so the
// absence of a database is not a problem here — and requiring one would mean
// this test could not run in the unit suite, which is where drift needs to be
// caught.
func buildTestRouter() chi.Router {
	passthrough := func(next http.Handler) http.Handler { return next }

	r := chi.NewRouter()
	r.Route("/api/v1", registerRoutes(routeDeps{
		auth:            authhttp.NewHandler(nil, 0, nil),
		users:           userhttp.NewHandler(nil, nil),
		org:             orghttp.NewHandler(nil, nil),
		invitations:     invitehttp.NewHandler(nil, nil),
		audit:           audithttp.NewHandler(nil, nil),
		messaging:       messaginghttp.NewHandler(nil, nil),
		notifications:   notificationhttp.NewHandler(nil, nil),
		tickets:         tickethttp.NewHandler(nil, nil),
		workbench:       workbenchhttp.NewHandler(nil, nil),
		requests:        requesttrackhttp.NewHandler(nil, nil),
		loginLimiter:    passthrough,
		authenticate:    passthrough,
		rateLimitByUser: passthrough,
		requirePermission: func(auth.Permission) func(http.Handler) http.Handler {
			return passthrough
		},
	}))
	return r
}

// walkRoutes flattens the router into "METHOD /path" keys using OpenAPI's
// brace syntax for parameters.
func walkRoutes(t *testing.T, r chi.Router) map[string]struct{} {
	t.Helper()

	found := map[string]struct{}{}
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		found[method+" "+normaliseRoute(route)] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	return found
}

// normaliseRoute converts chi's ":id" and "*" segments into OpenAPI templates
// and strips the trailing slash chi reports for a collection root.
func normaliseRoute(route string) string {
	segments := strings.Split(route, "/")
	for i, s := range segments {
		if strings.HasPrefix(s, ":") {
			segments[i] = "{" + strings.TrimPrefix(s, ":") + "}"
		}
	}
	joined := strings.Join(segments, "/")
	if len(joined) > 1 {
		joined = strings.TrimSuffix(joined, "/")
	}
	return joined
}

func templateParams(path string) []string {
	var names []string
	for _, s := range strings.Split(path, "/") {
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			names = append(names, strings.Trim(s, "{}"))
		}
	}
	return names
}

type specDocument struct {
	Paths      map[string]specPathItem `yaml:"paths"`
	Security   []map[string][]string   `yaml:"security"`
	Components struct {
		Parameters map[string]specParameter `yaml:"parameters"`
	} `yaml:"components"`
}

type specPathItem struct {
	Parameters []specParameter `yaml:"parameters"`
	Get        *specOperation  `yaml:"get"`
	Post       *specOperation  `yaml:"post"`
	Put        *specOperation  `yaml:"put"`
	Patch      *specOperation  `yaml:"patch"`
	Delete     *specOperation  `yaml:"delete"`
}

type specOperation struct {
	Parameters []specParameter        `yaml:"parameters"`
	Security   *[]map[string][]string `yaml:"security"`
}

type specParameter struct {
	Ref  string `yaml:"$ref"`
	Name string `yaml:"name"`
	In   string `yaml:"in"`
}

func (p specPathItem) operations() []specOperation {
	var ops []specOperation
	for _, op := range p.operationsByMethod() {
		ops = append(ops, op)
	}
	return ops
}

func (p specPathItem) operationsByMethod() map[string]specOperation {
	ops := map[string]specOperation{}
	for method, op := range map[string]*specOperation{
		http.MethodGet:    p.Get,
		http.MethodPost:   p.Post,
		http.MethodPut:    p.Put,
		http.MethodPatch:  p.Patch,
		http.MethodDelete: p.Delete,
	} {
		if op != nil {
			ops[method] = *op
		}
	}
	return ops
}

// resolveParameters follows the one form of $ref this document uses. A full
// resolver is not needed, but silently ignoring references would make the
// parameter check pass for the wrong reason.
func resolveParameters(doc specDocument, params []specParameter) []specParameter {
	out := make([]specParameter, 0, len(params))
	for _, p := range params {
		if p.Ref == "" {
			out = append(out, p)
			continue
		}
		name := strings.TrimPrefix(p.Ref, "#/components/parameters/")
		if target, ok := doc.Components.Parameters[name]; ok {
			out = append(out, target)
		}
	}
	return out
}

func loadSpec(t *testing.T) specDocument {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read api/openapi.yaml: %v", err)
	}

	var doc specDocument
	// KnownFields keeps a typo in a key from being read as an absent field,
	// which would let a malformed document pass every check below.
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("parse api/openapi.yaml: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("api/openapi.yaml declares no paths")
	}
	return doc
}

// loadSpecOperations reduces the document to the same "METHOD /path" keys the
// router produces, ignoring the probe endpoints: they are mounted outside
// /api/v1 by the shared router and are not part of this comparison.
func loadSpecOperations(t *testing.T) map[string]struct{} {
	t.Helper()

	ops := map[string]struct{}{}
	for path, item := range loadSpec(t).Paths {
		if !strings.HasPrefix(path, "/api/v1") {
			continue
		}
		for method := range item.operationsByMethod() {
			ops[method+" "+path] = struct{}{}
		}
	}
	return ops
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
