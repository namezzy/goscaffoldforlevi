// Package bruno holds a Bruno collection for poking at the API by hand.
//
// The test in this file exists because the collection is the kind of artefact
// that fails silently. A request pointing at an endpoint that was renamed or
// removed returns 404, which looks exactly like the service being down, and
// the first instinct is to go and check the service.
package bruno

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// request is one .bru file reduced to the only thing this test cares about.
type request struct {
	file   string
	method string
	path   string
}

// verbs is the closed set of HTTP methods a .bru block can open with. It is a
// set rather than a regexp because the same list is needed when walking the
// OpenAPI document, where a path item also carries non-verb keys such as
// "parameters".
var verbs = map[string]bool{
	"get": true, "post": true, "put": true,
	"patch": true, "delete": true, "head": true, "options": true,
}

// blockRe matches the opening block of a .bru request:
//
//	post {
//	  url: {{baseUrl}}/api/v1/auth/login
var blockRe = regexp.MustCompile(`(?m)^(get|post|put|patch|delete|head|options)\s*\{\s*\r?\n\s*url:\s*(\S+)`)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var varRe = regexp.MustCompile(`\{\{[^}]*\}\}`)

func TestEveryRequestTargetsADocumentedOperation(t *testing.T) {
	documented := documentedOperations(t)
	requests := collectionRequests(t)

	// Without this the test passes by finding nothing, which is the failure
	// mode that matters: someone moves the collection and the guard silently
	// stops guarding.
	if len(requests) == 0 {
		t.Fatal("no .bru requests found; the test would have passed vacuously")
	}

	for _, r := range requests {
		key := r.method + " " + r.path
		if !documented[key] {
			t.Errorf("%s targets %q, which api/openapi.yaml does not document", r.file, key)
		}
	}
}

// documentedOperations reads api/openapi.yaml and returns the set of
// "METHOD /path" strings it declares.
func documentedOperations(t *testing.T) map[string]bool {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}

	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	out := make(map[string]bool)
	for path, item := range doc.Paths {
		for key := range item {
			if verbs[strings.ToLower(key)] {
				out[strings.ToUpper(key)+" "+path] = true
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("openapi.yaml declared no operations; the comparison would be meaningless")
	}
	return out
}

// collectionRequests walks the collection and reduces every .bru request to a
// method and a spec-shaped path.
func collectionRequests(t *testing.T) []request {
	t.Helper()

	var out []request
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// environments/ holds vars, not requests.
		if d.IsDir() && d.Name() == "environments" {
			return filepath.SkipDir
		}
		if d.IsDir() || filepath.Ext(path) != ".bru" {
			return nil
		}
		// folder.bru and collection.bru carry settings, not requests.
		if base := filepath.Base(path); base == "folder.bru" || base == "collection.bru" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		m := blockRe.FindStringSubmatch(string(raw))
		if m == nil {
			t.Errorf("%s has no request block; it cannot be run", path)
			return nil
		}
		out = append(out, request{
			file:   path,
			method: strings.ToUpper(m[1]),
			path:   specPath(m[2]),
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk collection: %v", err)
	}
	return out
}

// specPath turns a Bruno URL into the path shape the OpenAPI document uses.
//
// Both variables and literal UUIDs collapse to {id}. Literals appear in the
// error cases, where the whole point is to send an identifier that resolves to
// nothing, and they must not be mistaken for a separate endpoint.
func specPath(url string) string {
	if i := strings.IndexByte(url, '?'); i >= 0 {
		url = url[:i]
	}
	url = strings.TrimPrefix(url, "{{baseUrl}}")

	segments := strings.Split(url, "/")
	for i, s := range segments {
		switch {
		case varRe.MatchString(s):
			segments[i] = "{id}"
		case uuidRe.MatchString(s):
			segments[i] = "{id}"
		}
	}
	return strings.Join(segments, "/")
}
