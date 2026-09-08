package scaffold

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Module template tokens. They are chosen so that no token is a prefix of
// another in a way a left-to-right replacer could confuse: __PKG__ cannot match
// inside __PKGS__, because the character after "__PKG" there is "S", not "_".
const (
	tokModule = "__MODULE_PATH__"
	tokPkg    = "__PKG__"
	tokPkgs   = "__PKGS__"
	tokType   = "__TYPE__"
	tokCode   = "__CODE__"
	tokVar    = "__VAR__"
)

// ModuleOptions describes a business module to generate.
type ModuleOptions struct {
	// Name is the entity in lower snake_case singular: "order", "order_item".
	Name string

	// Root is the project directory to generate into.
	Root string

	// ModulePath is the project's Go module path. When empty it is read from
	// go.mod under Root, which is what makes `goscaffold module order` work
	// with no flags inside a generated project.
	ModulePath string
}

var modulePathRE = regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`)

// ReadModulePath extracts the module path from the go.mod under root.
func ReadModulePath(root string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // path derived from a caller-supplied root
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w (run this inside a project, or pass --module)", err)
	}
	m := modulePathRE.FindSubmatch(raw)
	if m == nil {
		return "", fmt.Errorf("go.mod has no module line")
	}
	return string(m[1]), nil
}

// NewModule builds the plan for a business module: the domain package, its
// service and ports, an HTTP adapter, a PostgreSQL adapter, and tests.
//
// The shape follows ADR-0002 exactly — three packages, with the dependency
// running inward — because a generator that emits a different shape from the
// one the project uses is worse than no generator: it teaches the wrong
// pattern, at scale, to whoever runs it next.
func NewModule(opts ModuleOptions) (*Plan, error) {
	name, err := ParseName(opts.Name)
	if err != nil {
		return nil, err
	}
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.ModulePath == "" {
		opts.ModulePath, err = ReadModulePath(opts.Root)
		if err != nil {
			return nil, err
		}
	}

	tmpls, err := readTemplateTree("templates/module")
	if err != nil {
		return nil, err
	}

	replace := strings.NewReplacer(
		tokModule, opts.ModulePath,
		tokPkgs, name.Plural,
		tokPkg, name.Snake,
		tokType, name.Pascal,
		tokCode, name.Screaming,
		tokVar, name.Camel,
	)

	// Where each template lands. Keeping this table next to the templates
	// rather than encoding the destination in the file name means a reader can
	// see the whole generated layout in one place.
	destinations := map[string]string{
		"templates/module/entity.go.tmpl":       path.Join("internal", name.Snake, name.Snake+".go"),
		"templates/module/service.go.tmpl":      path.Join("internal", name.Snake, "service.go"),
		"templates/module/service_test.go.tmpl": path.Join("internal", name.Snake, "service_test.go"),
		"templates/module/repository.go.tmpl":   path.Join("internal", name.Snake, name.Snake+"pg", "repository.go"),
		"templates/module/dto.go.tmpl":          path.Join("internal", name.Snake, name.Snake+"http", "dto.go"),
		"templates/module/handler.go.tmpl":      path.Join("internal", name.Snake, name.Snake+"http", "handler.go"),
		"templates/module/handler_test.go.tmpl": path.Join("internal", name.Snake, name.Snake+"http", "handler_test.go"),
	}

	plan := &Plan{Root: opts.Root}
	for tmplPath, content := range tmpls {
		dest, ok := destinations[tmplPath]
		if !ok {
			return nil, fmt.Errorf("module template %s has no destination; add one to the table in module.go", tmplPath)
		}
		if err := plan.Add(dest, []byte(replace.Replace(string(content)))); err != nil {
			return nil, err
		}
	}
	if len(plan.Files) != len(destinations) {
		return nil, fmt.Errorf("expected %d module files, planned %d", len(destinations), len(plan.Files))
	}

	return plan, nil
}

// ModuleNextSteps is what the generator cannot do for the user.
//
// Every item is a change to a file that already exists, and this command
// refuses to modify existing files. Printing the list is not a lesser option:
// an editor that rewrites hand-written wiring is how a generator becomes
// something people are afraid to run.
func ModuleNextSteps(name Name) []string {
	return []string{
		fmt.Sprintf("Register the permissions in internal/auth/policy.go: %q, %q, %q, %q.",
			name.Snake+":create", name.Snake+":read", name.Snake+":list", name.Snake+":delete"),
		fmt.Sprintf("Until you do, every %s endpoint answers 403 — the policy map is the only "+
			"source of permissions and an unregistered one is never granted.", name.Snake),
		fmt.Sprintf("Create the table:  goscaffold migration create_%s", name.Plural),
		fmt.Sprintf("Write the queries in queries/%s.sql, run `make sqlc`, then replace the "+
			"NOT_IMPLEMENTED bodies in internal/%s/%spg/repository.go.", name.Snake, name.Snake, name.Snake),
		fmt.Sprintf("Add internal/%s/*.go to the business-layer depguard rule in .golangci.yml. "+
			"That rule enumerates its packages explicitly, so a new one starts out unprotected: "+
			"nothing would stop this package from importing net/http or pgx.", name.Snake),
		fmt.Sprintf("Wire it in cmd/api/features.go and mount /%s in the route table.", name.Plural),
		fmt.Sprintf("Document the endpoints in api/openapi.yaml — cmd/api/openapi_test.go fails " +
			"until the router and the specification agree, in both directions."),
	}
}
