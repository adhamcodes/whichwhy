package cli

import (
	"fmt"
	"io"

	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

func runPowerShellEvidence(args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "whichwhy: incomplete PowerShell evidence")
		return 2
	}

	evidence, err := ps.DecodeEvidence(args[0], args[1], args[2], args[3:])
	if err != nil {
		fmt.Fprintf(stderr, "whichwhy: %v\n", err)
		return 2
	}

	return printPowerShellEvidence(stdout, evidence)
}

func printPowerShellEvidence(stdout io.Writer, evidence ps.Evidence) int {
	fmt.Fprintf(stdout, "WhichWhy — %s\n\n", evidence.Command)

	winner, ok := evidence.Winner()
	if !ok {
		fmt.Fprintln(stdout, "No command match was found in the current loaded PowerShell session.")
		fmt.Fprintln(stdout, "\nCURRENT LIMIT")
		fmt.Fprintln(stdout, "  An unloaded module may still be auto-loaded when PowerShell invokes a command.")
		fmt.Fprintln(stdout, "  That case is not modeled yet.")
		return 1
	}

	fmt.Fprintln(stdout, "POWERSHELL WINNER")
	printPowerShellMatch(stdout, winner)

	if len(evidence.Matches) > 1 {
		fmt.Fprintln(stdout, "\nOTHER POWERSHELL MATCHES")
		for _, match := range evidence.Matches[1:] {
			printPowerShellMatch(stdout, match)
		}
	}

	fmt.Fprintln(stdout, "\nWHY")
	fmt.Fprintln(stdout, "  PowerShell itself reported this match first for the current loaded session.")
	fmt.Fprintf(stdout, "\nSHELL\n  PowerShell %s (%s)\n", evidence.Version, evidence.Edition)
	fmt.Fprintln(stdout, "\nCURRENT LIMIT")
	fmt.Fprintln(stdout, "  Unloaded module auto-loading is not modeled yet.")
	return 0
}

func printPowerShellMatch(stdout io.Writer, match ps.Match) {
	switch match.CommandType {
	case "Alias":
		if match.AliasTarget != "" {
			fmt.Fprintf(stdout, "  Alias %s -> %s\n", match.Name, match.AliasTarget)
			return
		}
		fmt.Fprintf(stdout, "  Alias %s\n", match.Name)
	case "Cmdlet":
		if match.Source != "" {
			fmt.Fprintf(stdout, "  Cmdlet %s [%s]\n", match.Name, match.Source)
			return
		}
		fmt.Fprintf(stdout, "  Cmdlet %s\n", match.Name)
	case "Application", "ExternalScript":
		fmt.Fprintf(stdout, "  %s %s\n", match.CommandType, match.Name)
		if match.Path != "" {
			fmt.Fprintf(stdout, "    %s\n", match.Path)
		}
	default:
		fmt.Fprintf(stdout, "  %s %s\n", match.CommandType, match.Name)
	}
}
