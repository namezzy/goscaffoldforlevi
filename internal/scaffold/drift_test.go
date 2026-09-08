package scaffold

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestProjectTemplatesMatchTheRepository is the whole answer to template drift.
//
// The templates under templates/project are a copy of this repository. A copy
// that is only maintained by hand diverges — not dramatically, but one forgotten
// file at a time, until a project generated from it is subtly not this project
// any more and nobody can say when that started. So the copy is derived, and
// this test is what makes "derived" true rather than aspirational: change a
// source file without running `make templates` and the build fails here.
//
// It is the same mechanism as `make sqlc-check`, applied to a different
// generated artefact, and for the same reason.
func TestProjectTemplatesMatchTheRepository(t *testing.T) {
	root := repositoryRoot(t)

	want, err := CollectFromRepo(root)
	if err != nil {
		t.Fatalf("collect from %s: %v", root, err)
	}
	have, err := embeddedProjectTemplates()
	if err != nil {
		t.Fatalf("read embedded templates: %v", err)
	}

	var missing, extra, changed []string

	for key, content := range want {
		embedded, ok := have[key]
		switch {
		case !ok:
			missing = append(missing, key)
		case string(embedded) != string(content):
			changed = append(changed, key)
		}
	}
	for key := range have {
		if _, ok := want[key]; !ok {
			extra = append(extra, key)
		}
	}

	if len(missing)+len(extra)+len(changed) == 0 {
		return
	}

	sort.Strings(missing)
	sort.Strings(extra)
	sort.Strings(changed)

	var b strings.Builder
	b.WriteString("the embedded project templates no longer match this repository.\n")
	b.WriteString("Run `make templates` and commit the result.\n\n")
	report(&b, "missing from the templates (a new or edited source file)", changed)
	report(&b, "not embedded at all", missing)
	report(&b, "embedded but no longer in the repository", extra)
	t.Fatal(b.String())
}

func report(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString(title + ":\n")
	for i, item := range items {
		if i == 10 {
			b.WriteString("  … and " + itoa(len(items)-10) + " more\n")
			break
		}
		b.WriteString("  " + strings.TrimPrefix(item, "templates/project/") + "\n")
	}
	b.WriteString("\n")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// TestEveryTemplateHasTheSuffix guards the one property that keeps the template
// tree from being compiled as part of this module. Without the suffix, every
// embedded .go file would join the build and every embedded _test.go would run
// — as a second, conflicting copy of the whole project.
func TestEveryTemplateHasTheSuffix(t *testing.T) {
	have, err := embeddedProjectTemplates()
	if err != nil {
		t.Fatalf("read embedded templates: %v", err)
	}
	for key := range have {
		if !strings.HasSuffix(key, templateSuffix) {
			t.Errorf("%s does not end in %s and would be compiled as part of this module", key, templateSuffix)
		}
	}
}

// repositoryRoot walks up until it finds the go.mod, so the test works
// wherever it is run from.
func repositoryRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
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
