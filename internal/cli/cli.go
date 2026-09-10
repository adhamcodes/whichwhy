package cli

import (
	"fmt"
	"io"
)

const usage = `WhichWhy — know exactly which command will run, and why.

Usage:
  whichwhy <command>
  whichwhy path
  whichwhy doctor
  whichwhy init <shell>
  whichwhy --help
  whichwhy --version

WhichWhy is under active development. Command investigation is not available yet.
`

// Run executes the command-line interface and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}

	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "-v", "--version", "version":
		fmt.Fprintf(stdout, "whichwhy %s\n", version)
		return 0
	default:
		fmt.Fprintln(stderr, "whichwhy: command investigation is not implemented yet")
		return 2
	}
}
