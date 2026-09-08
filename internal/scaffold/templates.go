package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// templates holds both trees:
//
//   - templates/project/… is a copy of this repository, produced by Sync and
//     verified by TestProjectTemplatesMatchTheRepository. Never edit it by
//     hand; edit the real file and run `make templates`.
//   - templates/module/… is written by hand. It cannot be derived from
//     anything, because it describes a module that does not exist yet. What
//     keeps it honest is the CI job that generates a project, generates a
//     module inside it, and compiles the result.
//
// The `all:` prefix is required: without it the embed skips every path whose
// name begins with a dot, which would silently drop .github, .golangci.yml,
// .env.example and .gitignore from every generated project.
//
//go:embed all:templates
var templates embed.FS

func embeddedProjectTemplates() (map[string][]byte, error) {
	return readTemplateTree("templates/project")
}

func readTemplateTree(root string) (map[string][]byte, error) {
	out := make(map[string][]byte)
	err := fs.WalkDir(templates, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		content, readErr := templates.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		out[p] = content
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read embedded templates: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no templates embedded under %s; run `make templates`", root)
	}
	return out, nil
}

// Sync writes the project template tree derived from repoRoot into dir,
// removing anything there that the repository no longer contains.
//
// It is the write half of the drift mechanism; the test is the read half.
func Sync(repoRoot, dir string) (added, updated, removed []string, err error) {
	want, err := CollectFromRepo(repoRoot)
	if err != nil {
		return nil, nil, nil, err
	}

	have := map[string]bool{}
	if _, statErr := os.Stat(dir); statErr == nil {
		walkErr := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, relErr := filepath.Rel(dir, p)
			if relErr != nil {
				return relErr
			}
			have["templates/project/"+filepath.ToSlash(rel)] = true
			return nil
		})
		if walkErr != nil {
			return nil, nil, nil, walkErr
		}
	}

	for key, content := range want {
		rel := strings.TrimPrefix(key, "templates/project/")
		full := filepath.Join(dir, filepath.FromSlash(rel))
		existing, readErr := os.ReadFile(full) //nolint:gosec // path derived from a caller-supplied directory
		switch {
		case readErr != nil:
			added = append(added, rel)
		case string(existing) == string(content):
			continue
		default:
			updated = append(updated, rel)
		}
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o750); mkErr != nil {
			return nil, nil, nil, mkErr
		}
		if wErr := os.WriteFile(full, content, 0o644); wErr != nil { //nolint:gosec // templates are not secrets
			return nil, nil, nil, wErr
		}
	}

	for key := range have {
		if _, ok := want[key]; ok {
			continue
		}
		rel := strings.TrimPrefix(key, "templates/project/")
		removed = append(removed, rel)
		if rmErr := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); rmErr != nil {
			return nil, nil, nil, rmErr
		}
	}

	sort.Strings(added)
	sort.Strings(updated)
	sort.Strings(removed)
	return added, updated, removed, nil
}
