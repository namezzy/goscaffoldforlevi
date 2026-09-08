package scaffold_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/namezzy/goscaffoldforlevi/internal/scaffold"
)

func TestParseNameDerivesEveryForm(t *testing.T) {
	tests := map[string]scaffold.Name{
		"order": {
			Snake: "order", Plural: "orders", Pascal: "Order", Camel: "order", Screaming: "ORDER",
		},
		"order_item": {
			Snake: "order_item", Plural: "order_items", Pascal: "OrderItem", Camel: "orderItem", Screaming: "ORDER_ITEM",
		},
		// Only the last word is pluralised: "orders_item" would be wrong.
		"invoice_category": {
			Snake: "invoice_category", Plural: "invoice_categories", Pascal: "InvoiceCategory",
			Camel: "invoiceCategory", Screaming: "INVOICE_CATEGORY",
		},
		"box": {
			Snake: "box", Plural: "boxes", Pascal: "Box", Camel: "box", Screaming: "BOX",
		},
		// A vowel before the y keeps the plain -s: "days", not "daies".
		"holiday": {
			Snake: "holiday", Plural: "holidays", Pascal: "Holiday", Camel: "holiday", Screaming: "HOLIDAY",
		},
		// Already plural, and left alone rather than doubled into "addresseses".
		"address": {
			Snake: "address", Plural: "addresses", Pascal: "Address", Camel: "address", Screaming: "ADDRESS",
		},
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := scaffold.ParseName(input)
			if err != nil {
				t.Fatalf("ParseName(%q): %v", input, err)
			}
			if got != want {
				t.Errorf("ParseName(%q)\n got %+v\nwant %+v", input, got, want)
			}
		})
	}
}

func TestParseNameRejectsAmbiguousInput(t *testing.T) {
	// Each of these has an obvious intended meaning and no obvious correct
	// conversion. Guessing produces the wrong package name silently, so the
	// generator refuses and says what it wants instead.
	for _, input := range []string{"", "Order", "orderItem", "order-item", "order__item", "_order", "order_", "1order", "order item"} {
		t.Run(input, func(t *testing.T) {
			if _, err := scaffold.ParseName(input); err == nil {
				t.Errorf("ParseName(%q) was accepted; it should be refused", input)
			}
		})
	}
}

func TestMigrationVersionsAreFixedWidthAndOrdered(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "migrations"), 0o755); err != nil {
		t.Fatal(err)
	}

	at := func(s string) time.Time {
		ts, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}

	first := mustMigration(t, scaffold.MigrationOptions{
		Name: "create_orders", Root: root, Now: at("2026-09-09T12:00:00.123Z"),
	})
	if err := first.Write(); err != nil {
		t.Fatalf("write: %v", err)
	}

	later := mustMigration(t, scaffold.MigrationOptions{
		Name: "add_orders_index", Root: root, Now: at("2026-09-10T12:00:00.000Z"),
	})

	v1, v2 := versionOf(t, first), versionOf(t, later)

	// Fixed width is the property that matters. If a collision were resolved by
	// adding precision, a 17-digit version would sort after every 14-digit one
	// — including migrations written weeks later — and golang-migrate would
	// apply them out of order.
	if len(v1) != len(v2) {
		t.Fatalf("versions have different widths: %q and %q", v1, v2)
	}
	if v1 >= v2 {
		t.Errorf("version %q should sort before %q", v1, v2)
	}
}

func TestMigrationAvoidsACollisionInTheSameMillisecond(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "migrations"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

	first := mustMigration(t, scaffold.MigrationOptions{Name: "one", Root: root, Now: now})
	if err := first.Write(); err != nil {
		t.Fatal(err)
	}
	second := mustMigration(t, scaffold.MigrationOptions{Name: "two", Root: root, Now: now})
	if err := second.Write(); err != nil {
		t.Fatalf("the second migration in the same millisecond was refused: %v", err)
	}

	if v1, v2 := versionOf(t, first), versionOf(t, second); v1 == v2 {
		t.Errorf("both migrations got version %q; golang-migrate would refuse the pair", v1)
	}
}

func TestMigrationRejectsAnInvalidName(t *testing.T) {
	for _, name := range []string{"", "Create Orders", "create-orders", "9orders"} {
		if _, err := scaffold.NewMigration(scaffold.MigrationOptions{Name: name, Root: t.TempDir()}); err == nil {
			t.Errorf("NewMigration(%q) was accepted", name)
		}
	}
}

func TestWriteRefusesToOverwriteAndChangesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "migrations"), 0o755); err != nil {
		t.Fatal(err)
	}

	plan := mustMigration(t, scaffold.MigrationOptions{Name: "create_orders", Root: root})

	// Put something in the way, with content worth protecting.
	victim := filepath.Join(root, filepath.FromSlash(plan.Sorted()[0].Path))
	const precious = "-- hand written, do not lose\n"
	if err := os.WriteFile(victim, []byte(precious), 0o644); err != nil {
		t.Fatal(err)
	}

	err := plan.Write()
	if err == nil {
		t.Fatal("the write succeeded; it must refuse when a file is in the way")
	}

	var conflict *scaffold.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("got %T, want a *scaffold.ConflictError", err)
	}
	if len(conflict.Paths) != 1 {
		t.Errorf("reported %d conflicts, want 1: %v", len(conflict.Paths), conflict.Paths)
	}

	got, readErr := os.ReadFile(victim)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != precious {
		t.Error("the existing file was modified despite the refusal")
	}

	// The refusal must be total: the other file of the pair must not have been
	// written either. A half-applied generation leaves a directory that is
	// neither the old state nor the new one.
	other := filepath.Join(root, filepath.FromSlash(plan.Sorted()[1].Path))
	if _, err := os.Stat(other); err == nil {
		t.Error("the second file was written even though the plan was refused")
	}
}

func TestConflictsReportsWithoutWriting(t *testing.T) {
	// This is what --dry-run runs. It must not create anything, including the
	// directories it would need.
	root := t.TempDir()
	plan := mustMigration(t, scaffold.MigrationOptions{Name: "create_orders", Root: root})

	if _, err := plan.Conflicts(); err != nil {
		t.Fatalf("conflicts: %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a dry run created %d entries in the output directory", len(entries))
	}
}

func TestNewProjectSubstitutesEveryIdentifier(t *testing.T) {
	plan, err := scaffold.NewProject(scaffold.ProjectOptions{
		Name:   "billing",
		Module: "github.com/acme/billing",
		Output: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	var offenders []string
	for _, f := range plan.Sorted() {
		// The generated README links back to the scaffold it came from; that
		// is the one place the source name is still correct.
		if f.Path == "README.md" {
			continue
		}
		if strings.Contains(string(f.Content), "goscaffoldforlevi") {
			offenders = append(offenders, f.Path)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("these files still name the scaffold instead of the new project: %v", offenders)
	}

	gomod := contentOf(t, plan, "go.mod")
	if !strings.Contains(gomod, "module github.com/acme/billing") {
		t.Errorf("go.mod does not declare the new module path:\n%s", firstLine(gomod))
	}
}

func TestNewProjectRejectsUnusableInput(t *testing.T) {
	tests := map[string]scaffold.ProjectOptions{
		"no name":            {Module: "github.com/acme/x"},
		"no module":          {Name: "billing"},
		"module without dot": {Name: "billing", Module: "billing"},
		// The name becomes a container name, a Kubernetes object name and a
		// metrics label. Upper case and underscores are rejected by at least
		// one of those, at deploy time rather than here.
		"upper case name": {Name: "Billing", Module: "github.com/acme/billing"},
		"underscore name": {Name: "my_service", Module: "github.com/acme/x"},
		"trailing dash":   {Name: "billing-", Module: "github.com/acme/x"},
	}

	for name, opts := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := scaffold.NewProject(opts); err == nil {
				t.Errorf("NewProject(%+v) was accepted", opts)
			}
		})
	}
}

func TestNewModuleFollowsTheThreePackageShape(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/acme/billing\n\ngo 1.27.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := scaffold.NewModule(scaffold.ModuleOptions{Name: "invoice_line", Root: root})
	if err != nil {
		t.Fatalf("NewModule: %v", err)
	}

	want := []string{
		"internal/invoice_line/invoice_line.go",
		"internal/invoice_line/service.go",
		"internal/invoice_line/service_test.go",
		"internal/invoice_line/invoice_linehttp/dto.go",
		"internal/invoice_line/invoice_linehttp/handler.go",
		"internal/invoice_line/invoice_linehttp/handler_test.go",
		"internal/invoice_line/invoice_linepg/repository.go",
	}
	got := map[string]bool{}
	for _, f := range plan.Files {
		got[f.Path] = true
	}
	for _, path := range want {
		if !got[path] {
			t.Errorf("missing %s", path)
		}
	}
	if len(plan.Files) != len(want) {
		t.Errorf("planned %d files, want %d", len(plan.Files), len(want))
	}

	// The module path came from go.mod, not from a flag: this is what makes
	// `goscaffold module x` work with no arguments inside a project.
	service := contentOf(t, plan, "internal/invoice_line/service.go")
	if !strings.Contains(service, "github.com/acme/billing/internal/apperr") {
		t.Error("the generated imports do not use the module path from go.mod")
	}

	// The domain package must not know about transport or storage (ADR-0002).
	for _, forbidden := range []string{"net/http", "pgx", "chi"} {
		if strings.Contains(service, forbidden) {
			t.Errorf("the generated service imports %q; the dependency runs inward only", forbidden)
		}
	}
}

func TestNewModuleNeedsAModulePath(t *testing.T) {
	// An empty directory has no go.mod, so there is nothing to import from.
	// Failing here is much better than emitting files with a guessed path.
	if _, err := scaffold.NewModule(scaffold.ModuleOptions{Name: "order", Root: t.TempDir()}); err == nil {
		t.Error("NewModule succeeded outside a project")
	}
}

// ---------------------------------------------------------------- helpers

func mustMigration(t *testing.T, opts scaffold.MigrationOptions) *scaffold.Plan {
	t.Helper()
	plan, err := scaffold.NewMigration(opts)
	if err != nil {
		t.Fatalf("NewMigration(%+v): %v", opts, err)
	}
	return plan
}

func versionOf(t *testing.T, plan *scaffold.Plan) string {
	t.Helper()
	base := filepath.Base(plan.Sorted()[0].Path)
	i := strings.IndexByte(base, '_')
	if i < 0 {
		t.Fatalf("%q has no version prefix", base)
	}
	return base[:i]
}

func contentOf(t *testing.T, plan *scaffold.Plan, path string) string {
	t.Helper()
	for _, f := range plan.Files {
		if f.Path == path {
			return string(f.Content)
		}
	}
	t.Fatalf("the plan has no %s", path)
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
