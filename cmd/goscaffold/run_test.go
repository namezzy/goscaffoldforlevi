package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every test here generates into t.TempDir(). A generator test that writes
// into the repository is one interrupted run away from leaving files behind
// that the next `git status` has to explain.

func exec(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// TestFlagsAfterTheNameAreParsed pins the documented invocation.
//
// The standard flag package stops at the first non-flag argument, so
// `new demo --module X` silently left --module unparsed until the name was
// removed first. The failure was a usage error on a correct command line.
func TestFlagsAfterTheNameAreParsed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")

	code, stdout, stderr := exec(t, "new", "demo", "--module", "github.com/example/demo", "--output", dir)
	if code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("go.mod was not written: %v", err)
	}
}

func TestFlagsBeforeTheNameAlsoWork(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")

	if code, stdout, stderr := exec(t, "new", "--module", "github.com/example/demo", "--output", dir, "demo"); code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "demo")

	code, stdout, stderr := exec(t, "new", "demo", "--module", "github.com/example/demo", "--output", out, "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "would be written") {
		t.Errorf("the dry run did not describe its plan:\n%s", stdout)
	}
	if !strings.Contains(stdout, "go.mod") {
		t.Errorf("the dry run did not list go.mod:\n%s", stdout)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("the dry run created the output directory")
	}
}

func TestExistingFilesAreNeverOverwritten(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const precious = "module github.com/someone/else\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(precious), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := exec(t, "new", "demo", "--module", "github.com/example/demo", "--output", dir)
	if code == 0 {
		t.Fatal("the command succeeded; it must refuse to overwrite")
	}
	if !strings.Contains(stderr, "go.mod") {
		t.Errorf("the error does not name the file in the way:\n%s", stderr)
	}

	got, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != precious {
		t.Fatal("the existing go.mod was overwritten")
	}
}

func TestModuleAndMigrationGenerateInsideAProject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	if code, _, stderr := exec(t, "new", "demo", "--module", "github.com/example/demo", "--output", dir); code != 0 {
		t.Fatalf("new: %s", stderr)
	}

	code, stdout, stderr := exec(t, "module", "order", "--dir", dir)
	if code != 0 {
		t.Fatalf("module: exit %d, %s", code, stderr)
	}
	// The next steps are the whole reason this command does not edit existing
	// files. Losing them would leave the user with a module wired to nothing.
	for _, expected := range []string{"policy.go", "features.go", "openapi.yaml", "depguard"} {
		if !strings.Contains(stdout, expected) {
			t.Errorf("the next steps do not mention %s:\n%s", expected, stdout)
		}
	}

	if code, _, stderr := exec(t, "migration", "create_orders", "--dir", dir); code != 0 {
		t.Fatalf("migration: exit %d, %s", code, stderr)
	}

	entries, err := filepath.Glob(filepath.Join(dir, "migrations", "*_create_orders.*.sql"))
	if err != nil || len(entries) != 2 {
		t.Errorf("expected an up/down pair, found %v (%v)", entries, err)
	}
}

func TestUnknownCommandsAndBadNamesFail(t *testing.T) {
	tests := map[string][]string{
		"no arguments":     {},
		"unknown command":  {"frobnicate"},
		"module with case": {"module", "OrderItem", "--dir", t.TempDir()},
		"migration dashes": {"migration", "create-orders", "--dir", t.TempDir()},
		"new without name": {"new", "--module", "github.com/example/demo"},
	}

	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			if code, _, _ := exec(t, args...); code == 0 {
				t.Errorf("%v exited 0; it should fail", args)
			}
		})
	}
}
