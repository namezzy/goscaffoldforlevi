package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/namezzy/goscaffoldforlevi/internal/buildinfo"
	"github.com/namezzy/goscaffoldforlevi/internal/scaffold"
)

// The CLI is built on the standard library's flag package.
//
// Cobra and urfave/cli buy shell completion, nested commands and generated
// help. This program has four commands, none of them nested, and its longest
// help text fits on a screen — so the entire benefit would be the generated
// help, in exchange for a dependency tree larger than the rest of this binary.
// ADR-0001 rejected Viper on the same grounds. Revisit if a command ever grows
// subcommands of its own, or if completion becomes a real request rather than
// a nice idea.
const usage = `goscaffold — generate projects, modules and migrations

Usage:
  goscaffold new <name> --module <path> [--output <dir>] [--dry-run]
  goscaffold module <name> [--dir <project>] [--dry-run]
  goscaffold migration <name> [--dir <project>] [--dry-run]
  goscaffold version

Commands:
  new         Create a project: the whole scaffold, with the module path and
              service name substituted throughout.
  module      Create a business module: domain, service, ports, HTTP adapter,
              a PostgreSQL stub and tests, following ADR-0002's three-package
              shape.
  migration   Create an up/down migration pair with a collision-free version.
  version     Print the build stamp.

Every command refuses to overwrite an existing file. Use --dry-run to see what
would be written.
`

// Writes to stdout and stderr have their errors discarded throughout, with
// `_, _ =` rather than silently: there is no useful recovery from a failed
// write to a terminal, and the alternative — propagating it — would put an
// error path on every line of help text.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}

	command, rest := args[0], args[1:]

	var err error
	switch command {
	case "new":
		err = runNew(rest, stdout)
	case "module":
		err = runModule(rest, stdout)
	case "migration":
		err = runMigration(rest, stdout)
	case "version":
		_, _ = fmt.Fprintf(stdout, "goscaffold %s (%s, built %s)\n",
			buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime)
		return 0
	case "help", "-h", "--help":
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", command, usage)
		return 2
	}

	if err != nil {
		_, _ = fmt.Fprintf(stderr, "goscaffold: %v\n", err)
		return 1
	}
	return 0
}

// takeName pulls the leading positional argument out of args.
//
// The standard flag package stops parsing at the first non-flag argument, so
// `new demo --module X` would leave --module unparsed. Every command here
// takes exactly one name, and the documented form puts it first, so it is
// removed before the flags are parsed. A name given after the flags still
// works, through fs.Arg(0).
func takeName(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

// newFlagSet builds a flag set that reports errors through the returned error
// rather than writing to stderr and exiting, so that run stays testable.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func runNew(args []string, stdout io.Writer) error {
	name, args := takeName(args)
	fs := newFlagSet("new")
	module := fs.String("module", "", "the Go module path of the new project")
	output := fs.String("output", "", "where to write it (default: the project name)")
	dryRun := fs.Bool("dry-run", false, "list the files that would be written, and write nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" || fs.NArg() > 1 {
		return fmt.Errorf("usage: goscaffold new <name> --module <path>")
	}

	opts := scaffold.ProjectOptions{Name: name, Module: *module, Output: *output}
	plan, err := scaffold.NewProject(opts)
	if err != nil {
		return err
	}

	if err := commit(plan, *dryRun, stdout); err != nil {
		return err
	}
	if *dryRun {
		return nil
	}

	_, _ = fmt.Fprintf(stdout, "\nCreated %s (%d files).\n\nNext:\n", plan.Root, len(plan.Files))
	for _, step := range []string{
		fmt.Sprintf("cd %s", plan.Root),
		"cp .env.example .env",
		"docker compose up -d && make migrate-up",
		"make run",
	} {
		_, _ = fmt.Fprintf(stdout, "  %s\n", step)
	}
	return nil
}

func runModule(args []string, stdout io.Writer) error {
	name, args := takeName(args)
	fs := newFlagSet("module")
	dir := fs.String("dir", ".", "the project to generate into")
	dryRun := fs.Bool("dry-run", false, "list the files that would be written, and write nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" || fs.NArg() > 1 {
		return fmt.Errorf("usage: goscaffold module <name>")
	}

	parsed, err := scaffold.ParseName(name)
	if err != nil {
		return err
	}

	plan, err := scaffold.NewModule(scaffold.ModuleOptions{Name: name, Root: *dir})
	if err != nil {
		return err
	}
	if err := commit(plan, *dryRun, stdout); err != nil {
		return err
	}
	if *dryRun {
		return nil
	}

	_, _ = fmt.Fprintf(stdout, "\nCreated the %s module. The generator does not edit existing files, so:\n\n", parsed.Snake)
	for _, step := range scaffold.ModuleNextSteps(parsed) {
		_, _ = fmt.Fprintf(stdout, "  - %s\n", step)
	}
	return nil
}

func runMigration(args []string, stdout io.Writer) error {
	name, args := takeName(args)
	fs := newFlagSet("migration")
	dir := fs.String("dir", ".", "the project to generate into")
	dryRun := fs.Bool("dry-run", false, "list the files that would be written, and write nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" || fs.NArg() > 1 {
		return fmt.Errorf("usage: goscaffold migration <name>")
	}

	plan, err := scaffold.NewMigration(scaffold.MigrationOptions{Name: name, Root: *dir})
	if err != nil {
		return err
	}
	return commit(plan, *dryRun, stdout)
}

// commit is the single place a plan becomes files, so that --dry-run exercises
// the same construction as a real run and cannot drift from it.
func commit(plan *scaffold.Plan, dryRun bool, stdout io.Writer) error {
	if dryRun {
		conflicts, err := plan.Conflicts()
		if err != nil {
			return err
		}
		blocked := map[string]bool{}
		for _, c := range conflicts {
			blocked[c] = true
		}

		_, _ = fmt.Fprintf(stdout, "%d file(s) would be written into %s:\n", len(plan.Files), plan.Root)
		for _, f := range plan.Sorted() {
			marker := " "
			if blocked[f.Path] {
				marker = "!"
			}
			_, _ = fmt.Fprintf(stdout, " %s %s\n", marker, filepath.ToSlash(f.Path))
		}
		if len(conflicts) > 0 {
			_, _ = fmt.Fprintf(stdout, "\n%d marked ! already exist. A real run would refuse:\n  %s\n",
				len(conflicts), strings.Join(conflicts, "\n  "))
		}
		return nil
	}
	return plan.Write()
}
