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
		state, status, style = "SELECTED", "definitive within process policy", good
		if r.Scope == resolution.PowerShellScope {
			status = "loaded-session discovery"
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
		h.item(r.NoCandidateReason, true, "")
	} else {
		h.item(r.SelectionReason, true, "")
	}
	if len(r.Alternatives) > 0 {
		title := "Also found (process policy)"
		if r.Scope == resolution.PowerShellScope {
			title = "Shadowed (loaded session)"
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
	h.section("Resolution")
	if r.Inspection != nil {
		h.item("Selection   "+r.SelectionStatus, false, "")
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
	} else if r.Shell != nil {
		h.item("Discovery in the observed loaded session; invocation is not verified.", true, "")
	}
	if r.Shell != nil {
		h.section("Evidence")
		h.item(r.Shell.Name+" "+r.Shell.Version+" ("+r.Shell.Edition+")", false, "")
		h.item("Session: currently loaded commands; module auto-loading is not modeled.", true, quiet)
	}
	h.section("Scope")
	h.item(r.Scope+" / "+r.ClaimStrength, false, quiet)
	h.item("Policy: "+r.Policy, true, quiet)
	h.section("Limits")
	for i, limitation := range r.Limitations {
		h.item(limitation, i == len(r.Limitations)-1, "")
	}
}
