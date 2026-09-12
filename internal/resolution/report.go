// Package resolution turns collected evidence into a completed, scoped claim.
// It performs no command execution or shell discovery.
package resolution

import (
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
	PathIndex                             int // One-based process PATH index; zero for shell evidence.
}

type Shell struct {
	Name, Version, Edition string
}

// Report is a completed selection under a named policy, never a promise of
// successful execution. ClaimStrength describes the evidence class, including
// when Selected is nil; nil means no observed candidate under this policy.
type Report struct {
	Command, Scope, Policy, ClaimStrength string
	Shell                                 *Shell
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
		Candidates:        make([]Candidate, 0, len(evidence.Candidates)),
		NoCandidateReason: "No external command candidate was observed under this process policy.",
		Limitations: []string{
			"The invoking shell was not observed; its selected command may differ, including current-directory, empty or quoted PATH entry behavior.",
			"Shell-local aliases, functions, built-ins, cmdlets, and command caches are not inspected.",
			"Filesystem observation failures and skipped candidates are not fully retained; no candidate does not prove the command is unavailable.",
			"Candidate eligibility is a filesystem filter, not proof of successful execution or invoking-user permission.",
		},
	}
	for _, c := range evidence.Candidates {
		r.Candidates = append(r.Candidates, Candidate{Type: "external", Path: c.Path, PathIndex: c.DirectoryIndex + 1})
	}
	return selectFirst(r, "First observed candidate in process-path-order-v1 enumeration order; shell selection is unknown.")
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
// means a scoped selection exists, not that shell truth has been established.
func (r Report) ExitCode() int {
	if r.Selected == nil {
		return 1
	}
	return 0
}
