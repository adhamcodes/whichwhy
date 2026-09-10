package cli

import (
	"fmt"
	"io"

	"github.com/adhamcodes/whichwhy/internal/resolver"
)

const usage = `WhichWhy — know exactly which command will run, and why.

Usage:
  whichwhy <command>
  whichwhy path
  whichwhy doctor
  whichwhy init <shell>
  whichwhy --help
  whichwhy --version

External command inspection is available. Shell-aware resolution is still under development.
`

type externalResolver func(string) (resolver.Result, error)

// Run executes the command-line interface and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	return run(args, stdout, stderr, version, resolver.ResolveExternal)
}

func run(args []string, stdout, stderr io.Writer, version string, resolve externalResolver) int {
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
	case "path", "doctor", "init":
		fmt.Fprintf(stderr, "whichwhy: %s is not implemented yet\n", args[0])
		return 2
	}

	if len(args) != 1 {
		fmt.Fprintln(stderr, "whichwhy: command inspection currently accepts one command name")
		return 2
	}

	result, err := resolve(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "whichwhy: %v\n", err)
		return 2
	}
	return printExternalResult(stdout, result)
}

func printExternalResult(stdout io.Writer, result resolver.Result) int {
	fmt.Fprintf(stdout, "WhichWhy — %s\n\n", result.Command)

	winner, ok := result.Winner()
	if !ok {
		fmt.Fprintln(stdout, "No external command candidate was found in the current search path.")
		fmt.Fprintln(stdout, "\nShell aliases, functions, built-ins, cmdlets, and command caches are not inspected yet.")
		return 1
	}

	fmt.Fprintln(stdout, "EXTERNAL COMMAND WINNER")
	fmt.Fprintf(stdout, "  %s\n", winner.Path)

	if len(result.Candidates) > 1 {
		fmt.Fprintln(stdout, "\nOTHER EXTERNAL CANDIDATES")
		for _, candidate := range result.Candidates[1:] {
			fmt.Fprintf(stdout, "  %s\n", candidate.Path)
		}
	}

	fmt.Fprintln(stdout, "\nWHY")
	fmt.Fprintln(stdout, "  This candidate appears first in the process-visible external command search order.")
	fmt.Fprintln(stdout, "\nCURRENT LIMIT")
	fmt.Fprintln(stdout, "  Shell-local aliases, functions, built-ins, cmdlets, and command caches are not inspected yet.")
	return 0
}
