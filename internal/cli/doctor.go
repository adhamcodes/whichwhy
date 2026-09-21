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
	doctorDiscoveryUncertain doctorDiscovery = "uncertain"
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
	case resolved.SelectionStatus == resolution.SelectionUncertain || (resolved.Selected == nil && resolved.Inspection.Completeness == resolution.InspectionIncomplete):
		report.Discovery = doctorDiscoveryUncertain
		report.Status = 1
	case resolved.Selected == nil:
		report.Discovery = doctorDiscoveryMissing
		report.Status = 1
	case sameExecutable(current, resolved.Selected.Path):
		report.Discovery = doctorDiscoveryCurrent
	default:
		report.Discovery = doctorDiscoveryDifferent
		report.Status = 1
	}
	if len(candidates) > 1 || resolved.Inspection.Completeness == resolution.InspectionIncomplete {
		report.Status = 1
	}
	return report, nil
}

func runDoctor(stdout, stderr io.Writer, version string, executable executableLocator, resolve externalResolver) int {
	report, err := inspectDoctor(version, executable, resolve)
	if err != nil {
		printError(stderr, "failure: "+err.Error()+"; verify the running executable and PATH access, then retry")
		return 2
	}
	return printDoctorResult(stdout, report)
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
