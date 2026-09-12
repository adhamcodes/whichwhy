package cli

import (
	"fmt"
	"io"

	"github.com/adhamcodes/whichwhy/internal/resolution"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

func runPowerShellEvidence(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "whichwhy: incomplete PowerShell evidence")
		return 2
	}

	evidence, err := ps.DecodeEvidence(args[0], args[1], args[2], args[3:])
	if err != nil {
		fmt.Fprintf(stderr, "whichwhy: %v\n", err)
		return 2
	}
	report := resolution.PowerShell(evidence)
	if jsonOutput {
		return printCommandJSON(stdout, stderr, report)
	}
	return printCommandReport(stdout, report)
}

func printCommandCandidate(stdout io.Writer, match resolution.Candidate) {
	switch match.Type {
	case "external":
		fmt.Fprintf(stdout, "  %s\n", match.Path)
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
		fmt.Fprintf(stdout, "  %s %s\n", match.Type, match.Name)
		if match.Path != "" {
			fmt.Fprintf(stdout, "    %s\n", match.Path)
		}
	default:
		fmt.Fprintf(stdout, "  %s %s\n", match.Type, match.Name)
	}
}
