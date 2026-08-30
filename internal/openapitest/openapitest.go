// Package openapitest checks real responses against api/openapi.yaml.
//
// The route-level test in cmd/api proves that every documented path is served
// and every served path is documented. It says nothing about response bodies,
// which is where a specification rots quietly: a field gets renamed in a DTO,
// every Go test still passes because they assert on the struct, and the
// document keeps promising the old name to every generated client.
//
// This package closes that gap by comparing the bytes a handler actually
// writes against the schema the document declares. It is only imported from
// tests; nothing links it into a binary.
package openapitest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Spec is the schema section of the OpenAPI document.
type Spec struct {
	schemas map[string]*schema
}

type schema struct {
	Ref        string             `yaml:"$ref"`
	Type       string             `yaml:"type"`
	Required   []string           `yaml:"required"`
	Properties map[string]*schema `yaml:"properties"`
	Items      *schema            `yaml:"items"`
}

type document struct {
	Components struct {
		Schemas map[string]*schema `yaml:"schemas"`
	} `yaml:"components"`
}

// Load reads api/openapi.yaml, locating it by walking up to the module root so
// that callers at any depth can use it without spelling out "../../..".
func Load(t *testing.T) Spec {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read api/openapi.yaml: %v", err)
	}

	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse api/openapi.yaml: %v", err)
	}
	if len(doc.Components.Schemas) == 0 {
		t.Fatal("api/openapi.yaml declares no component schemas")
	}
	return Spec{schemas: doc.Components.Schemas}
}

// AssertMatches checks a response body against a named schema.
//
// Two rules are enforced, in both directions:
//
//   - every property in the body must be documented, which catches a field
//     added to a DTO and forgotten in the document;
//   - every property the document marks required must be present, which
//     catches a field renamed or dropped while the document kept promising it.
//
// A documented optional property may be absent, so a sample error without
// `details` is not a failure.
func (s Spec) AssertMatches(t *testing.T, schemaName string, body []byte) {
	t.Helper()

	root, ok := s.schemas[schemaName]
	if !ok {
		t.Fatalf("api/openapi.yaml has no schema named %q", schemaName)
	}
	s.check(t, schemaName, root, body)
}

func (s Spec) check(t *testing.T, path string, sch *schema, value []byte) {
	t.Helper()

	sch = s.resolve(t, path, sch)

	switch {
	case sch.Items != nil:
		var items []json.RawMessage
		if err := json.Unmarshal(value, &items); err != nil {
			t.Errorf("%s: the document says array, the response is not one: %v", path, err)
			return
		}
		for i, item := range items {
			s.check(t, fmt.Sprintf("%s[%d]", path, i), sch.Items, item)
		}

	case len(sch.Properties) > 0:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(value, &object); err != nil {
			t.Errorf("%s: the document says object, the response is not one: %v", path, err)
			return
		}

		for name, raw := range object {
			property, documented := sch.Properties[name]
			if !documented {
				t.Errorf("%s.%s is in the response but not in api/openapi.yaml (documented: %s)",
					path, name, keysOf(sch.Properties))
				continue
			}
			// A null carries no shape to compare against, and nullability is
			// not modelled here.
			if string(raw) != "null" {
				s.check(t, path+"."+name, property, raw)
			}
		}

		for _, name := range sch.Required {
			if _, present := object[name]; !present {
				t.Errorf("%s.%s is required by api/openapi.yaml but missing from the response",
					path, name)
			}
		}
	}
}

func (s Spec) resolve(t *testing.T, path string, sch *schema) *schema {
	t.Helper()

	for range 10 {
		if sch.Ref == "" {
			return sch
		}
		name := strings.TrimPrefix(sch.Ref, "#/components/schemas/")
		target, ok := s.schemas[name]
		if !ok {
			t.Fatalf("%s: api/openapi.yaml refers to %q, which it does not define", path, sch.Ref)
		}
		sch = target
	}
	t.Fatalf("%s: the $ref chain in api/openapi.yaml does not terminate", path)
	return nil
}

func keysOf(m map[string]*schema) string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the working directory")
		}
		dir = parent
	}
}
