package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

const usage = `WhichWhy — know exactly which command will run, and why.

Usage:
  whichwhy <command>
  whichwhy <command> --json
  whichwhy path
  whichwhy path --json
  whichwhy doctor
  whichwhy init <shell>
  whichwhy --help
  whichwhy --version

External command inspection and PATH diagnostics are available. PowerShell session integration is experimental.
`

type externalResolver func(string) (resolver.Result, error)

// Run executes the command-line interface and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	if len(args) > 0 {
		switch args[0] {
		case "__powershell":
			return runPowerShellEvidence(args[1:], stdout, stderr, false)
		case "__powershell-json":
			return runPowerShellEvidence(args[1:], stdout, stderr, true)
		}
	}

	if len(args) == 2 && args[0] == "init" && strings.EqualFold(args[1], "powershell") {
		executable, err := os.Executable()
		if err != nil {
			fmt.Fprintf(stderr, "whichwhy: locate executable: %v\n", err)
			return 2
		}
		fmt.Fprint(stdout, ps.InitScript(executable))
		return 0
	}

	if len(args) > 0 && args[0] == "path" {
		switch {
		case len(args) == 1:
			return runPath(stdout, stderr, os.LookupEnv, pathdiag.Inspect)
		case len(args) == 2 && args[1] == "--json":
			return runPathJSON(stdout, stderr, os.LookupEnv, pathdiag.Inspect)
		default:
			fmt.Fprintln(stderr, "whichwhy: path accepts optional --json")
			return 2
		}
	}

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
	case "doctor", "init":
		fmt.Fprintf(stderr, "whichwhy: %s is not implemented yet\n", args[0])
		return 2
	}

	jsonOutput := len(args) == 2 && args[1] == "--json"
	if len(args) != 1 && !jsonOutput {
		fmt.Fprintln(stderr, "whichwhy: command inspection accepts one command name and optional --json")
		return 2
	}

	result, err := resolve(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "whichwhy: %v\n", err)
		return 2
	}
	if jsonOutput {
		return printExternalJSON(stdout, stderr, result)
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
