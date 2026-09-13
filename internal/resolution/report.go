// Package resolution turns collected evidence into a completed, scoped claim.
// It performs no command execution or shell discovery.
package resolution

import (
	"runtime"

	"github.com/adhamcodes/whichwhy/internal/processpath"
	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

const (
	ProcessScope     = "process-external"
	ProcessPolicy    = "process-path-order-v1"
	PolicyOnly       = "policy-only"
	PowerShellScope  = "powershell-loaded-session"
	PowerShellPolicy = "powershell-loaded-session-order-v1"
	ShellObserved    = "shell-observed"
)

// Candidate retains the identity supplied by the evidence collector.
type Candidate struct {
	Type, Name, Path, Source, AliasTarget string
	PathIndex                             int // One-based ProcessPath entry index; zero for shell evidence.
}

type Shell struct {
	Name, Version, Edition string
}

const (
	InspectionComplete   = "complete"
	InspectionIncomplete = "incomplete"
	SelectionDefinitive  = "definitive"
	SelectionUncertain   = "precedence-uncertain"
	SelectionNone        = "no-candidate"
)

// ProcessInspection describes collection completeness, independently of the
// evidence class and of whether failures could precede the selected candidate.
type ProcessInspection struct {
	Completeness string                 `json:"completeness"`
	Observations []resolver.Observation `json:"observations"`
}

// Report is a completed selection under a named policy, never a promise of
// successful execution. ClaimStrength describes the evidence class, including
// when Selected is nil; nil means no observed candidate under this policy.
type Report struct {
	Command, Scope, Policy, ClaimStrength string
	Shell                                 *Shell
	ProcessPath                           *processpath.Path  // Nil for shell-observed evidence.
	Inspection                            *ProcessInspection // Nil for shell-observed evidence.
	SelectionStatus                       string             // Process selection only; definitive within this policy.
	Selected                              *Candidate
	Candidates                            []Candidate
	Alternatives                          []Candidate
	SelectionReason                       string
	NoCandidateReason                     string
	Limitations                           []string
}

// ProcessExternal applies process-path-order-v1 to the collector's ordered
// candidates. The exact enumeration policy is defined in docs/resolution-policy.md.
func ProcessExternal(evidence resolver.Result) Report {
	r := Report{
		Command: evidence.Command, Scope: ProcessScope, Policy: ProcessPolicy,
		ClaimStrength:     PolicyOnly,
		ProcessPath:       &evidence.Path,
		Inspection:        &ProcessInspection{Completeness: InspectionComplete, Observations: append([]resolver.Observation{}, evidence.Observations...)},
		SelectionStatus:   SelectionNone,
		Candidates:        make([]Candidate, 0, len(evidence.Candidates)),
		NoCandidateReason: "No external command candidate was observed under this process policy.",
		Limitations: []string{
			"The invoking shell was not observed; its selected command may differ, including current-directory, empty or quoted PATH entry behavior.",
			"Shell-local aliases, functions, built-ins, cmdlets, and command caches are not inspected.",
			"Filesystem observations are not atomic; no candidate does not prove the command is unavailable.",
			"Candidate eligibility is a filesystem filter, not proof of successful execution or invoking-user permission.",
		},
	}
	if runtime.GOOS != "windows" {
		r.Limitations = append(r.Limitations,
			"Unix eligibility uses effective UID, effective GID and supplementary groups with owner/group/other mode-class precedence; UID 0 requires some execute bit. Symlinks use target metadata.",
			"Mode-class eligibility does not evaluate ACLs, noexec mounts, Linux capabilities or distinct filesystem IDs, macOS extended group membership, or other platform restrictions; actual access may differ in either direction.",
			"Identity and filesystem observations are not atomic; later credential, permission or file changes and interpreter/shebang failures can prevent execution.",
		)
	}
	for _, c := range evidence.Candidates {
		r.Candidates = append(r.Candidates, Candidate{Type: "external", Path: c.Path, PathIndex: c.DirectoryIndex + 1})
	}
	r = selectFirst(r, "First eligible candidate in process-path-order-v1 enumeration order; no earlier attempt was unresolved. Shell selection is unknown.")
	seenCandidate, earlierFailure := false, false
	for _, o := range evidence.Observations {
		if o.Incomplete() {
			r.Inspection.Completeness = InspectionIncomplete
		}
		if o.Status == resolver.ObservedError && !seenCandidate {
			earlierFailure = true
		}
		if o.Status == resolver.ObservedCandidate {
			seenCandidate = true
		}
	}
	if r.Selected != nil {
		r.SelectionStatus = SelectionDefinitive
		if earlierFailure {
			r.SelectionStatus = SelectionUncertain
			r.SelectionReason = "Selected among observed eligible candidates only; an earlier unresolved attempt could precede this candidate. Process-policy precedence is uncertain; shell selection is unknown."
		}
	} else if r.Inspection.Completeness == InspectionIncomplete {
		r.NoCandidateReason = "No external command candidate was observed; inspection was incomplete, so failed attempts may conceal eligible candidates."
	}
	return r
}

// PowerShell retains the order observed by passive discovery in the active
// loaded session. It does not emulate PowerShell precedence or module loading.
func PowerShell(evidence ps.Evidence) Report {
	r := Report{
		Command: evidence.Command, Scope: PowerShellScope, Policy: PowerShellPolicy,
		ClaimStrength:     ShellObserved,
		Shell:             &Shell{Name: "PowerShell", Version: evidence.Version, Edition: evidence.Edition},
		Candidates:        make([]Candidate, 0, len(evidence.Matches)),
		NoCandidateReason: "No command match was found in the current loaded PowerShell session.",
		Limitations: []string{
			"Unloaded module auto-loading is not modeled yet.",
			"An unloaded module may still be auto-loaded when PowerShell invokes a command.",
			"Observed command discovery does not establish that invocation or an alias target will succeed.",
		},
	}
	for _, m := range evidence.Matches {
		r.Candidates = append(r.Candidates, Candidate{Type: m.CommandType, Name: m.Name, Path: m.Path, Source: m.Source, AliasTarget: m.AliasTarget})
	}
	return selectFirst(r, "PowerShell itself reported this match first for the current loaded session.")
}

func selectFirst(r Report, reason string) Report {
	r.Alternatives = []Candidate{}
	if len(r.Candidates) > 0 {
		selected := r.Candidates[0]
		r.Selected = &selected
		r.Alternatives = r.Candidates[1:]
		r.SelectionReason = reason
		r.NoCandidateReason = ""
	}
	return r
}

// ExitCode preserves the CLI's found/no-observed-candidate distinction. Success
// means a scoped selection among observed candidates exists, including when its
// precedence is uncertain. Retained failures are not fatal operational errors.
func (r Report) ExitCode() int {
	if r.Selected == nil {
		return 1
	}
	return 0
}
