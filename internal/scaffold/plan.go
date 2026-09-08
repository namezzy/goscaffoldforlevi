package scaffold

import (
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File is one file a generator intends to write.
type File struct {
	// Path is relative to the output directory, always with forward slashes.
	Path string

	// Content is the fully substituted bytes.
	Content []byte

	// Mode is the permission bits. Scripts need the executable bit; everything
	// else is 0o644.
	Mode fs.FileMode
}

// Plan is a complete set of writes, assembled before anything touches the
// disk.
//
// The two-phase shape is the whole safety mechanism. A generator that writes as
// it goes discovers a conflict halfway through and leaves a directory that is
// neither the old state nor the new one, which is the worst outcome available:
// the user cannot re-run it and cannot easily undo it. Building the plan first
// makes "would this collide?" answerable before the first byte is written, and
// makes --dry-run the same code path as a real run rather than a second
// implementation of it.
type Plan struct {
	Root  string
	Files []File
}

// Add appends a file, rejecting duplicates within the plan itself.
//
// Go source is reformatted on the way in. Substitution changes the width of
// identifiers, so a template whose struct tags and comments line up around
// __TYPE__ stops lining up around Order — and the generated project then fails
// its own `make check` on the first run, which is a poor introduction. Running
// the formatter here also turns a template that is not valid Go into an error
// at generation time rather than a confusing compile failure later.
func (p *Plan) Add(path string, content []byte) error {
	path = filepath.ToSlash(path)
	for _, f := range p.Files {
		if f.Path == path {
			return fmt.Errorf("the generator produced %s twice", path)
		}
	}
	if strings.HasSuffix(path, ".go") {
		formatted, err := format.Source(content)
		if err != nil {
			return fmt.Errorf("the generated %s is not valid Go: %w", path, err)
		}
		content = formatted
	}
	p.Files = append(p.Files, File{Path: path, Content: content, Mode: 0o644})
	return nil
}

// Sorted returns the files in path order, so that output is deterministic and
// two runs can be compared.
func (p *Plan) Sorted() []File {
	files := append([]File(nil), p.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

// ConflictError reports files that already exist on disk.
//
// It lists all of them rather than stopping at the first, because the useful
// question is "what is in my way?" and answering it one file per run turns a
// single decision into a dozen.
type ConflictError struct {
	Root  string
	Paths []string
}

func (e *ConflictError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "refusing to overwrite %d existing file", len(e.Paths))
	if len(e.Paths) != 1 {
		b.WriteString("s")
	}
	fmt.Fprintf(&b, " in %s:\n", e.Root)
	for _, p := range e.Paths {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	b.WriteString("\nMove or delete them, or generate into a different directory.")
	return b.String()
}

// Conflicts reports which of the planned files already exist.
func (p *Plan) Conflicts() ([]string, error) {
	var found []string
	for _, f := range p.Sorted() {
		full := filepath.Join(p.Root, filepath.FromSlash(f.Path))
		switch _, err := os.Lstat(full); {
		case err == nil:
			found = append(found, f.Path)
		case errors.Is(err, fs.ErrNotExist):
		default:
			return nil, fmt.Errorf("check %s: %w", full, err)
		}
	}
	return found, nil
}

// Write commits the plan.
//
// It refuses if anything is in the way. There is no --force: this command is
// most often run against a directory someone has already put work into, and an
// overwrite there is unrecoverable, while the cost of refusing is that the user
// deletes a file and runs it again. The seam for adding one later is exactly
// this check — nothing below it inspects existing state.
func (p *Plan) Write() error {
	conflicts, err := p.Conflicts()
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return &ConflictError{Root: p.Root, Paths: conflicts}
	}

	for _, f := range p.Sorted() {
		full := filepath.Join(p.Root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(full), err)
		}
		// O_EXCL rather than a plain create. The conflict check above races
		// with anything else touching the directory, and this is the only
		// check that cannot be raced: the file system decides.
		file, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, f.Mode) //nolint:gosec // the path is the caller's chosen output directory
		if err != nil {
			return fmt.Errorf("create %s: %w", full, err)
		}
		if _, err := file.Write(f.Content); err != nil {
			_ = file.Close()
			return fmt.Errorf("write %s: %w", full, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close %s: %w", full, err)
		}
	}
	return nil
}
