package cli

import (
	"fmt"
	"io"

	"github.com/adhamcodes/whichwhy/internal/resolution"
)

func printCommandReport(stdout io.Writer, report resolution.Report) int {
	return newHuman(stdout).command(report)
}

func (h humanRenderer) command(r resolution.Report) int {
	// Display completed report states; do not inspect Candidates to choose one.
	state, status, style := "NOT FOUND", "no observed match", warn
	if r.Selected != nil {
		state, status, style = "SELECTED", "external PATH search", good
		if r.Scope == resolution.PowerShellScope {
			status = "PowerShell session"
		} else if r.SelectionStatus != resolution.SelectionDefinitive {
			state, status, style = "OBSERVED CANDIDATE", "precedence uncertain", warn
		}
	} else if r.Inspection != nil && r.Inspection.Completeness == resolution.InspectionIncomplete {
		state, status = "NO DEFINITIVE RESULT", "inspection incomplete"
	}
	h.heading(r.Command, status, style)
	if r.Selected != nil && r.Inspection != nil && r.Inspection.Completeness == resolution.InspectionIncomplete {
		h.line("", "[ inspection incomplete ]", warn)
	}
	h.blank()
	h.line("  ", state, style)
	if r.Selected != nil {
		h.panel(candidateLines(*r.Selected))
	}
	h.section("Why")
	if r.Selected == nil {
		h.item(humanReason(r.NoCandidateReason), true, "")
	} else {
		h.item(humanReason(r.SelectionReason), true, "")
	}
	if len(r.Alternatives) > 0 {
		title := "Also found"
		if r.Scope == resolution.PowerShellScope {
			title = "Shadowed in this session"
		}
		h.section(title)
		for i, c := range r.Alternatives {
			lines := candidateLines(c)
			h.dataItem(lines[0], i == len(r.Alternatives)-1, "")
			for _, line := range lines[1:] {
				h.line("     ", line, quiet)
			}
		}
	}
	h.claim(r)
	h.section("Safe inspection")
	h.item("No inspected command was executed. No configuration was changed.", true, quiet)
	return r.ExitCode()
}

func candidateLines(c resolution.Candidate) []string {
	if c.Type == "external" {
		return []string{fmt.Sprintf("%s (PATH #%d)", c.Path, c.PathIndex)}
	}
	lines := []string{c.Type + " " + c.Name}
	if c.AliasTarget != "" {
		lines[0] += " -> " + c.AliasTarget
	}
	if c.Path != "" {
		lines = append(lines, c.Path)
	}
	if c.Source != "" {
		lines = append(lines, "Source: "+c.Source)
	}
	return lines
}

func (h humanRenderer) claim(r resolution.Report) {
	if r.Inspection != nil {
		h.section("Resolution")
		h.item("Selection   "+humanSelection(r.SelectionStatus), false, "")
		style := quiet
		if r.Inspection.Completeness == resolution.InspectionIncomplete {
			style = warn
		}
		h.item("Inspection  "+r.Inspection.Completeness, true, style)
		failures := []string{}
		for _, o := range r.Inspection.Observations {
			if o.Incomplete() {
				failures = append(failures, fmt.Sprintf("Attempt #%d, PATH #%d, %s: %s (%s): %s", o.Attempt, o.PathIndex, o.Path, o.Error.Category, o.Error.Operation, o.Error.Message))
			}
		}
		if len(failures) > 0 {
			h.section("Observation failures")
			for i, failure := range failures {
				h.dataItem(failure, i == len(failures)-1, warn)
			}
		}
	}
	if r.Shell != nil {
		h.section("Evidence")
		h.dataItem(r.Shell.Name+" "+r.Shell.Version+" ("+r.Shell.Edition+")", true, "")
	}
	h.section("Scope")
	switch r.Scope {
	case resolution.ProcessScope:
		h.item("External commands visible to this process, in PATH order. Your shell's aliases, functions, built-ins, cached commands, or search rules may choose differently. This snapshot does not guarantee execution.", true, "")
	case resolution.PowerShellScope:
		h.item("Current loaded PowerShell session. Unloaded modules may add commands when invoked; discovery does not guarantee execution or a working alias target.", true, "")
	default:
		// Unknown evidence classes must remain explicit rather than borrowing a
		// familiar scope's guarantees.
		h.item(r.Scope+" / "+r.ClaimStrength, false, quiet)
		h.item("Policy: "+r.Policy, true, quiet)
	}
	var additional []string
	for _, limitation := range r.Limitations {
		if (r.Scope != resolution.ProcessScope && r.Scope != resolution.PowerShellScope) || !summarizedLimits[limitation] {
			additional = append(additional, limitation)
		}
	}
	if len(additional) > 0 {
		h.section("Additional limits")
		for i, limitation := range additional {
			h.item(limitation, i == len(additional)-1, "")
		}
	}
}

// Translate existing completed explanations, never infer reasons from candidate
// order. Unrecognized explanations pass through, including custom reports.
func humanReason(reason string) string {
	switch reason {
	case "First eligible candidate in process-path-order-v1 enumeration order; no earlier attempt was unresolved. Shell selection is unknown.":
		return "First eligible external command in PATH search order; no earlier lookup was unresolved."
	case "Selected among observed eligible candidates only; an earlier unresolved attempt could precede this candidate. Process-policy precedence is uncertain; shell selection is unknown.":
		return "First among the commands observed, but an earlier failed lookup could hide a command that takes priority."
	case "No external command candidate was observed under this process policy.":
		return "No external command was found in this process's PATH search. Your shell may still find it."
	case "No external command candidate was observed; inspection was incomplete, so failed attempts may conceal eligible candidates.":
		return "No external command was observed. Inspection was incomplete; failed lookups may hide matching commands."
	default:
		return reason
	}
}

func humanSelection(status string) string {
	switch status {
	case resolution.SelectionDefinitive:
		return "definitive in this PATH search"
	case resolution.SelectionUncertain:
		return "uncertain order"
	case resolution.SelectionNone:
		return "none observed"
	default:
		return status
	}
}

// Only these established generic caveats are covered by the concise scope
// notes. New or result-specific limitations stay visible until explicitly
// reviewed; JSON always retains the entire original list.
var summarizedLimits = map[string]bool{
	"The invoking shell was not observed; its selected command may differ, including current-directory, empty or quoted PATH entry behavior.":                                                                                     true,
	"Shell-local aliases, functions, built-ins, cmdlets, and command caches are not inspected.":                                                                                                                                   true,
	"Filesystem observations are not atomic; no candidate does not prove the command is unavailable.":                                                                                                                             true,
	"Candidate eligibility is a filesystem filter, not proof of successful execution or invoking-user permission.":                                                                                                                true,
	"Unix eligibility uses effective UID, effective GID and supplementary groups with owner/group/other mode-class precedence; UID 0 requires some execute bit. Symlinks use target metadata.":                                    true,
	"Mode-class eligibility does not evaluate ACLs, noexec mounts, Linux capabilities or distinct filesystem IDs, macOS extended group membership, or other platform restrictions; actual access may differ in either direction.": true,
	"Identity and filesystem observations are not atomic; later credential, permission or file changes and interpreter/shebang failures can prevent execution.":                                                                   true,
	"Unloaded module auto-loading is not modeled yet.":                                               true,
	"An unloaded module may still be auto-loaded when PowerShell invokes a command.":                 true,
	"Observed command discovery does not establish that invocation or an alias target will succeed.": true,
}
