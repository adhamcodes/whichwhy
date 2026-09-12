package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

const usage = `WhichWhy — inspect command-resolution evidence and its limits.

Usage:
  whichwhy <command>
  whichwhy <command> --json
  whichwhy path
  whichwhy path --json
  whichwhy doctor
  whichwhy doctor --json
  whichwhy init <shell>
  whichwhy --help
  whichwhy --version

External command inspection, PATH diagnostics, and the installation doctor are available. PowerShell session integration is experimental.
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

	if len(args) > 0 && args[0] == "doctor" {
		switch {
		case len(args) == 1:
			return runDoctor(stdout, stderr, version, os.Executable, resolver.ResolveExternal)
		case len(args) == 2 && args[1] == "--json":
			return runDoctorJSON(stdout, stderr, version, os.Executable, resolver.ResolveExternal)
		default:
			fmt.Fprintln(stderr, "whichwhy: doctor accepts optional --json")
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
	case "init":
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
	report := resolution.ProcessExternal(result)
	if jsonOutput {
		return printCommandJSON(stdout, stderr, report)
	}
	return printCommandReport(stdout, report)
}

func printCommandReport(stdout io.Writer, report resolution.Report) int {
	fmt.Fprintf(stdout, "WhichWhy — %s\n\n", report.Command)
	if report.Selected == nil {
		fmt.Fprintln(stdout, report.NoCandidateReason)
	} else {
		if report.Scope == resolution.PowerShellScope {
			fmt.Fprintln(stdout, "POWERSHELL WINNER (LOADED SESSION)")
		} else {
			fmt.Fprintln(stdout, "PROCESS POLICY SELECTED CANDIDATE")
		}
		printCommandCandidate(stdout, *report.Selected)
		if len(report.Alternatives) > 0 {
			fmt.Fprintln(stdout, "\nOTHER CANDIDATES UNDER THIS POLICY")
			for _, candidate := range report.Alternatives {
				printCommandCandidate(stdout, candidate)
			}
		}
		fmt.Fprintf(stdout, "\nWHY\n  %s\n", report.SelectionReason)
	}
	printClaim(stdout, report)
	return report.ExitCode()
}

func printClaim(stdout io.Writer, report resolution.Report) {
	fmt.Fprintf(stdout, "\nRESOLUTION SCOPE\n  %s\nPOLICY\n  %s\nCLAIM STRENGTH\n  %s\n", report.Scope, report.Policy, report.ClaimStrength)
	if report.Shell != nil {
		fmt.Fprintf(stdout, "\nSHELL\n  %s %s (%s)\n", report.Shell.Name, report.Shell.Version, report.Shell.Edition)
	}
	fmt.Fprintln(stdout, "\nCURRENT LIMITS")
	for _, limitation := range report.Limitations {
		fmt.Fprintf(stdout, "  %s\n", limitation)
	}
}
