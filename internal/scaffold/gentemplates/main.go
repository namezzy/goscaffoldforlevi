// Command gentemplates refreshes the project templates from this repository.
//
// It is the write half of the drift mechanism described in
// internal/scaffold/templates.go. The read half is
// TestProjectTemplatesMatchTheRepository, which fails when the two disagree —
// so forgetting to run this is caught by `make check`, not discovered by
// whoever generates the next project.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/namezzy/goscaffoldforlevi/internal/scaffold"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	dir := filepath.Join(root, "internal", "scaffold", "templates", "project")
	added, updated, removed, err := scaffold.Sync(root, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentemplates: %v\n", err)
		os.Exit(1)
	}

	for _, p := range added {
		fmt.Printf("  + %s\n", p)
	}
	for _, p := range updated {
		fmt.Printf("  ~ %s\n", p)
	}
	for _, p := range removed {
		fmt.Printf("  - %s\n", p)
	}
	fmt.Printf("templates: %d added, %d updated, %d removed\n", len(added), len(updated), len(removed))
}
