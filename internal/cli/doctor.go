package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type executableLocator func() (string, error)

func runDoctor(stdout, stderr io.Writer, version string, executable executableLocator, resolve externalResolver) int {
	current, err := executable()
	if err != nil {
		fmt.Fprintf(stderr, "whichwhy: locate running executable: %v\n", err)
		return 2
	}
	current = filepath.Clean(current)

	result, err := resolve("whichwhy")
	if err != nil {
		fmt.Fprintf(stderr, "whichwhy: inspect command discovery: %v\n", err)
		return 2
	}

	fmt.Fprintln(stdout, "WhichWhy — doctor")
	fmt.Fprintf(stdout, "\nVERSION\n  %s\n", version)
	fmt.Fprintf(stdout, "\nPLATFORM\n  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(stdout, "\nRUNNING EXECUTABLE\n  %s\n", current)
	fmt.Fprintln(stdout, "\nCOMMAND DISCOVERY")

	status := 0
	winner, ok := result.Winner()
	switch {
	case !ok:
		fmt.Fprintln(stdout, "  WARNING  'whichwhy' is not discoverable through the process-visible PATH.")
		status = 1
	case sameExecutable(current, winner.Path):
		fmt.Fprintln(stdout, "  OK       PATH resolves 'whichwhy' to this running executable.")
	default:
		fmt.Fprintln(stdout, "  WARNING  PATH resolves 'whichwhy' to a different executable.")
		fmt.Fprintf(stdout, "           PATH winner: %s\n", winner.Path)
		fmt.Fprintf(stdout, "           Running:     %s\n", current)
		status = 1
	}

	if len(result.Candidates) > 1 {
		fmt.Fprintln(stdout, "\nOTHER WHICHWHY CANDIDATES")
		for _, candidate := range result.Candidates[1:] {
			fmt.Fprintf(stdout, "  %s\n", candidate.Path)
		}
		status = 1
	}

	fmt.Fprintln(stdout, "\nSUMMARY")
	if status == 0 {
		fmt.Fprintln(stdout, "  OK — command discovery is consistent with the executable that is running.")
	} else {
		fmt.Fprintln(stdout, "  WARNING — WhichWhy is running, but its command discovery may be incomplete or ambiguous.")
	}

	fmt.Fprintln(stdout, "\nSAFETY")
	fmt.Fprintln(stdout, "  Doctor only inspected the running executable and command search results. It changed nothing.")
	return status
}

func sameExecutable(a, b string) bool {
	if aInfo, aErr := os.Stat(a); aErr == nil {
		if bInfo, bErr := os.Stat(b); bErr == nil && os.SameFile(aInfo, bInfo) {
			return true
		}
	}

	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
