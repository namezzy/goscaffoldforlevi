// Command goscaffold generates projects, modules and migrations for this
// scaffold.
//
// It is not a low-code platform and is not meant to grow into one. It writes
// the files a person would otherwise copy by hand, and stops.
package main

import (
	"os"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
