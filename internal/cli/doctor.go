package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/resolver"
)

type executableLocator func() (string, error)

type doctorDiscovery string

const (
	doctorDiscoveryMissing   doctorDiscovery = "missing"
	doctorDiscoveryCurrent   doctorDiscovery = "current"
	doctorDiscoveryDifferent doctorDiscovery = "different"
)

type doctorReport struct {
	Version           string
	OS                string
	Arch              string
	RunningExecutable string
	Discovery         doctorDiscovery
	Candidates        []resolver.Candidate
	Status            int
}

func inspectDoctor(version string, executable executableLocator, resolve externalResolver) (doctorReport, error) {
	current, err := executable()
	if err != nil {
		return doctorReport{}, fmt.Errorf("locate running executable: %w", err)
	}
	current = filepath.Clean(current)

	result, err := resolve("whichwhy")
	if err != nil {
		return doctorReport{}, fmt.Errorf("inspect command discovery: %w", err)
	}
	candidates := distinctExecutableCandidates(result.Candidates)

	report := doctorReport{
		Version:           version,
		OS:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		RunningExecutable: current,
		Candidates:        candidates,
	}

	switch {
	case len(candidates) == 0:
		report.Discovery = doctorDiscoveryMissing
		report.Status = 1
	case sameExecutable(current, candidates[0].Path):
		report.Discovery = doctorDiscoveryCurrent
	default:
		report.Discovery = doctorDiscoveryDifferent
		report.Status = 1
	}
	if len(candidates) > 1 {
		report.Status = 1
	}
	return report, nil
}

func runDoctor(stdout, stderr io.Writer, version string, executable executableLocator, resolve externalResolver) int {
	report, err := inspectDoctor(version, executable, resolve)
	if err != nil {
		fmt.Fprintf(stderr, "whichwhy: %v\n", err)
		return 2
	}
	return printDoctorResult(stdout, report)
}

func printDoctorResult(stdout io.Writer, report doctorReport) int {
	fmt.Fprintln(stdout, "WhichWhy — doctor")
	fmt.Fprintf(stdout, "\nVERSION\n  %s\n", report.Version)
	fmt.Fprintf(stdout, "\nPLATFORM\n  %s/%s\n", report.OS, report.Arch)
	fmt.Fprintf(stdout, "\nRUNNING EXECUTABLE\n  %s\n", report.RunningExecutable)
	fmt.Fprintln(stdout, "\nCOMMAND DISCOVERY")

	switch report.Discovery {
	case doctorDiscoveryMissing:
		fmt.Fprintln(stdout, "  WARNING  'whichwhy' is not discoverable through the process-visible PATH.")
	case doctorDiscoveryCurrent:
		fmt.Fprintln(stdout, "  OK       PATH resolves 'whichwhy' to this running executable.")
	case doctorDiscoveryDifferent:
		fmt.Fprintln(stdout, "  WARNING  PATH resolves 'whichwhy' to a different executable.")
		fmt.Fprintf(stdout, "           PATH winner: %s\n", report.Candidates[0].Path)
		fmt.Fprintf(stdout, "           Running:     %s\n", report.RunningExecutable)
	}

	if len(report.Candidates) > 1 {
		fmt.Fprintln(stdout, "\nOTHER WHICHWHY CANDIDATES")
		for _, candidate := range report.Candidates[1:] {
			fmt.Fprintf(stdout, "  %s\n", candidate.Path)
		}
	}

	fmt.Fprintln(stdout, "\nSUMMARY")
	if report.Status == 0 {
		fmt.Fprintln(stdout, "  OK — command discovery is consistent with the executable that is running.")
	} else {
		fmt.Fprintln(stdout, "  WARNING — WhichWhy is running, but its command discovery may be incomplete or ambiguous.")
	}

	fmt.Fprintln(stdout, "\nSAFETY")
	fmt.Fprintln(stdout, "  Doctor only inspected the running executable and command search results. It changed nothing.")
	return report.Status
}

func distinctExecutableCandidates(candidates []resolver.Candidate) []resolver.Candidate {
	distinct := make([]resolver.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		repeated := false
		for _, existing := range distinct {
			if sameExecutable(existing.Path, candidate.Path) {
				repeated = true
				break
			}
		}
		if !repeated {
			distinct = append(distinct, candidate)
		}
	}
	return distinct
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
