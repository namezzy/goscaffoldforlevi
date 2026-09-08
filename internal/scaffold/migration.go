package scaffold

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

// migrationDir is where golang-migrate looks, per sqlc.yaml and the Makefile.
const migrationDir = "migrations"

// migrationVersionLayout is deliberately millisecond-precision, always.
//
// The obvious design — seconds, with more precision added only when two
// migrations land in the same second — silently breaks ordering. A 17-digit
// version is larger than every 14-digit one, so a file created to dodge a
// collision today would sort after a migration created next week. Versions
// must be comparable as integers forever, and the only way to guarantee that
// is to keep the width fixed.
const migrationVersionLayout = "20060102150405.000"

var migrationNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// MigrationOptions describes a migration pair to generate.
type MigrationOptions struct {
	// Name is the snake_case description: "create_orders", "add_email_index".
	Name string

	// Root is the project directory.
	Root string

	// Now is injectable so that tests do not depend on the clock.
	Now time.Time
}

// NewMigration builds the plan for an up/down migration pair.
//
// Both files are always created, even though the down file starts empty of
// intent. golang-migrate cannot roll back what does not exist, and a down
// migration written months later, from memory, against a schema that has moved
// on, is the one nobody trusts to run.
func NewMigration(opts MigrationOptions) (*Plan, error) {
	name := strings.TrimSpace(opts.Name)
	if !migrationNameRE.MatchString(name) {
		return nil, fmt.Errorf("%q is not a valid migration name: use lower-case snake_case, "+
			"for example create_orders or add_orders_status_index", opts.Name)
	}
	if opts.Root == "" {
		opts.Root = "."
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	version, err := nextVersion(path.Join(opts.Root, migrationDir), opts.Now)
	if err != nil {
		return nil, err
	}

	plan := &Plan{Root: opts.Root}
	base := path.Join(migrationDir, version+"_"+name)

	up := fmt.Sprintf(`-- %s (up)
--
-- Write the forward change here. Two rules this project holds to:
--
--   * Additive first. Add a column before anything writes to it, and drop the
--     old one a release later, so the previous version of the application can
--     still run against this schema. A rollback of the code is not a rollback
--     of the database.
--   * Index creation on a live table belongs in its own migration, using
--     CREATE INDEX CONCURRENTLY, which cannot run inside a transaction.
`, name)

	down := fmt.Sprintf(`-- %s (down)
--
-- Reverse exactly what the up migration did, and nothing else.
--
-- Note what running this costs: reversing a change discards whatever the new
-- version wrote through it. That is why forward-only recovery is preferred in
-- production and this file is mainly for local work and for CI.
`, name)

	if err := plan.Add(base+".up.sql", []byte(up)); err != nil {
		return nil, err
	}
	if err := plan.Add(base+".down.sql", []byte(down)); err != nil {
		return nil, err
	}
	return plan, nil
}

// nextVersion returns a version string not already used in dir.
func nextVersion(dir string, now time.Time) (string, error) {
	taken := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if i := strings.IndexByte(e.Name(), '_'); i > 0 {
				taken[e.Name()[:i]] = true
			}
		}
	}
	// A missing directory is not an error: `goscaffold migration` is useful in
	// a project that has not created one yet.

	for attempt := 0; attempt < 1000; attempt++ {
		v := strings.Replace(now.Add(time.Duration(attempt)*time.Millisecond).UTC().Format(migrationVersionLayout), ".", "", 1)
		if !taken[v] {
			return v, nil
		}
	}
	return "", fmt.Errorf("could not find a free migration version in %s after 1000 attempts", dir)
}
