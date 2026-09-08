package scaffold

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// The identifiers this repository uses for itself. They are replaced when a
// project is generated, in this order, by a single left-to-right pass — so a
// target module that itself contains "goscaffold" is not mangled by the later
// rules.
const (
	sourceModule   = "github.com/namezzy/goscaffoldforlevi"
	sourceName     = "goscaffoldforlevi"
	sourcePrefix   = "goscaffold"
	templateSuffix = ".tmpl"
)

// projectIncludes lists what a generated project inherits from this
// repository. Anything not under one of these paths is left behind.
//
// The list is deliberately explicit rather than "everything except". A new
// file in this repository does not silently become part of every project
// generated afterwards; someone has to decide that it should.
var projectIncludes = []string{
	"cmd/api",
	"cmd/worker",
	"cmd/adminctl",
	"internal",
	"api/openapi.yaml",
	"api/bruno",
	"migrations",
	"queries",
	"deployments",
	".github/workflows",
	"docs/adr",
	"docs/architecture.md",
	"test/blackbox",
	"Dockerfile",
	".dockerignore",
	"docker-compose.yml",
	"Makefile",
	".golangci.yml",
	".gitignore",
	".env.example",
	"sqlc.yaml",
	"go.mod",
	"go.sum",
}

// projectExcludes carves out what must not travel.
var projectExcludes = []string{
	// The generator cannot be part of what it generates: the templates would
	// then have to contain themselves.
	"internal/scaffold",
	"cmd/goscaffold",

	// This workflow runs the scaffold CLI, which a generated project does not
	// contain. Copying it would hand every new project a job that fails on its
	// first push.
	".github/workflows/scaffold.yml",
}

// junk never belongs in a template, whatever directory it turns up in.
var junk = map[string]bool{
	".DS_Store":   true,
	"__pycache__": true,
	"tok.json":    true,
}

// TemplatePath maps a repository path to its template path.
//
// The suffix is what keeps the Go toolchain out of the template tree: without
// it, every embedded .go file would be compiled as part of this module and
// every embedded _test.go would run.
func TemplatePath(repoPath string) string {
	return path.Join("templates", "project", filepath.ToSlash(repoPath)) + templateSuffix
}

// repoPathOf is the inverse of TemplatePath.
func repoPathOf(templatePath string) (string, bool) {
	const prefix = "templates/project/"
	if !strings.HasPrefix(templatePath, prefix) || !strings.HasSuffix(templatePath, templateSuffix) {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(templatePath, prefix), templateSuffix), true
}

// CollectFromRepo reads the repository at root and returns the template tree it
// should produce, keyed by template path.
//
// This function, and the test that compares its output against the committed
// templates, are the entire answer to template drift: the templates are a
// derived artefact of this repository, exactly as the sqlc output is, and a
// stale copy fails the build rather than waiting to surprise someone.
func CollectFromRepo(root string) (map[string][]byte, error) {
	tracked, err := trackedFiles(root)
	if err != nil {
		return nil, err
	}

	out := make(map[string][]byte)

	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if junk[d.Name()] || d.Name() == ".git" {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// Prune anything that cannot contain an included file, so the walk
			// does not descend into bin/ or node_modules-sized trees.
			if isExcluded(rel) || !mayContainIncluded(rel) {
				return fs.SkipDir
			}
			return nil
		}
		// Only what git tracks. This is not a convenience: api/bruno/.env is
		// ignored by git and holds a real credential, and an earlier version of
		// this function copied it into the templates — where the .tmpl suffix
		// meant .gitignore no longer matched it, and it was committed. The
		// manifest cannot be trusted to enumerate every ignored file that might
		// ever appear beneath an included directory; git already knows.
		if !tracked[rel] {
			return nil
		}
		if !isIncluded(rel) {
			return nil
		}
		content, readErr := os.ReadFile(p) //nolint:gosec // paths come from walking a caller-supplied root
		if readErr != nil {
			return fmt.Errorf("read %s: %w", rel, readErr)
		}
		out[TemplatePath(rel)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no files collected from %s: is it the repository root?", root)
	}
	return out, nil
}

// trackedFiles lists the repository's tracked paths, relative to root.
func trackedFiles(root string) (map[string]bool, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z") //nolint:gosec // root is the caller's repository path
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list tracked files in %s: %w (%s)", root, err, strings.TrimSpace(stderr.String()))
	}

	tracked := make(map[string]bool)
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			tracked[name] = true
		}
	}
	if len(tracked) == 0 {
		return nil, fmt.Errorf("git reported no tracked files in %s", root)
	}
	return tracked, nil
}

func isExcluded(rel string) bool {
	for _, ex := range projectExcludes {
		if rel == ex || strings.HasPrefix(rel, ex+"/") {
			return true
		}
	}
	return false
}

func isIncluded(rel string) bool {
	if isExcluded(rel) {
		return false
	}
	for _, in := range projectIncludes {
		if rel == in || strings.HasPrefix(rel, in+"/") {
			return true
		}
	}
	return false
}

// mayContainIncluded reports whether a directory is on the path to something
// included, so that the walk can prune everything else.
func mayContainIncluded(dir string) bool {
	for _, in := range projectIncludes {
		if strings.HasPrefix(in+"/", dir+"/") || strings.HasPrefix(dir+"/", in+"/") {
			return true
		}
	}
	return false
}

// ProjectOptions describes a project to generate.
type ProjectOptions struct {
	// Name is the service name: the value that replaces this repository's own
	// name in container names, labels and APP_NAME.
	Name string

	// Module is the Go module path of the new project.
	Module string

	// Output is the directory to write into.
	Output string
}

// Validate checks the options and fills in what can be derived.
func (o *ProjectOptions) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("a project name is required")
	}
	if !validServiceName(o.Name) {
		return fmt.Errorf("%q is not a usable service name: use lower-case letters, digits and dashes, "+
			"because it becomes a container name, a Kubernetes object name and a metrics label", o.Name)
	}
	if o.Module == "" {
		return fmt.Errorf("--module is required, for example --module github.com/acme/%s", o.Name)
	}
	if strings.Contains(o.Module, " ") || !strings.Contains(o.Module, "/") {
		return fmt.Errorf("%q does not look like a module path; expected something like github.com/acme/%s",
			o.Module, o.Name)
	}
	if o.Output == "" {
		o.Output = o.Name
	}
	return nil
}

func validServiceName(s string) bool {
	if s == "" || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}

// renderReadme produces the generated project's own README.
//
// This repository's README is not copied: it describes a scaffold, including
// its build phases and its own verification status, none of which is true of
// the project generated from it. Shipping it would hand every new project a
// document that is wrong from the first line.
func renderReadme(opts ProjectOptions) ([]byte, error) {
	raw, err := templates.ReadFile("templates/newproject/README.md.tmpl")
	if err != nil {
		return nil, fmt.Errorf("read the README template: %w", err)
	}
	return []byte(strings.NewReplacer("__PROJECT_NAME__", opts.Name).Replace(string(raw))), nil
}

// NewProject builds the plan for a new project.
func NewProject(opts ProjectOptions) (*Plan, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	templates, err := embeddedProjectTemplates()
	if err != nil {
		return nil, err
	}

	replace := strings.NewReplacer(
		sourceModule, opts.Module,
		sourceName, opts.Name,
		sourcePrefix, opts.Name,
	)

	plan := &Plan{Root: opts.Output}
	for templatePath, content := range templates {
		repoPath, ok := repoPathOf(templatePath)
		if !ok {
			return nil, fmt.Errorf("unexpected template path %q", templatePath)
		}
		if err := plan.Add(repoPath, []byte(replace.Replace(string(content)))); err != nil {
			return nil, err
		}
	}

	readme, err := renderReadme(opts)
	if err != nil {
		return nil, err
	}
	if err := plan.Add("README.md", readme); err != nil {
		return nil, err
	}

	return plan, nil
}
