package cli

import (
	"io"

	"github.com/adhamcodes/whichwhy/internal/resolution"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

func runPowerShellEvidence(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) < 3 {
		printError(stderr, "incomplete PowerShell evidence: expected base64 command, version, edition, and optional records")
		return 2
	}

	evidence, err := ps.DecodeEvidence(args[0], args[1], args[2], args[3:])
	if err != nil {
		printError(stderr, err.Error())
		return 2
	}
	report := resolution.PowerShell(evidence)
	if jsonOutput {
		return printCommandJSON(stdout, stderr, report)
	}
	return printCommandReport(stdout, report)
}
