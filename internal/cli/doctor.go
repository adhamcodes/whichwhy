package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/resolution"
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
	Resolution        resolution.Report
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
	result.Candidates = candidates
	resolved := resolution.ProcessExternal(result)

	report := doctorReport{
		Version:           version,
		OS:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		RunningExecutable: current,
		Resolution:        resolved,
	}

	switch {
	case resolved.Selected == nil:
		report.Discovery = doctorDiscoveryMissing
		report.Status = 1
	case sameExecutable(current, resolved.Selected.Path):
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
		fmt.Fprintln(stdout, "  WARNING  No 'whichwhy' candidate was observed under the process policy.")
	case doctorDiscoveryCurrent:
		fmt.Fprintln(stdout, "  OK       The process policy selects this running executable for 'whichwhy'.")
	case doctorDiscoveryDifferent:
		fmt.Fprintln(stdout, "  WARNING  The process policy selects a different executable for 'whichwhy'.")
		fmt.Fprintf(stdout, "           Policy candidate: %s\n", report.Resolution.Selected.Path)
		fmt.Fprintf(stdout, "           Running:     %s\n", report.RunningExecutable)
	}

	if len(report.Resolution.Alternatives) > 0 {
		fmt.Fprintln(stdout, "\nOTHER WHICHWHY CANDIDATES")
		for _, candidate := range report.Resolution.Alternatives {
			fmt.Fprintf(stdout, "  %s\n", candidate.Path)
		}
	}

	fmt.Fprintln(stdout, "\nSUMMARY")
	if report.Status == 0 {
		fmt.Fprintln(stdout, "  OK — command discovery is consistent with the executable that is running.")
	} else {
		fmt.Fprintln(stdout, "  WARNING — WhichWhy is running, but its command discovery may be incomplete or ambiguous.")
	}

	printClaim(stdout, report.Resolution)
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
