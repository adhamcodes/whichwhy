package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
)

type envLookup func(string) (string, bool)
type pathInspector func(string) pathdiag.Report

func runPath(stdout, stderr io.Writer, lookup envLookup, inspect pathInspector) int {
	value, ok := lookup("PATH")
	if !ok {
		printError(stderr, "PATH is not set; inspect your shell's PATH configuration")
		return 1
	}
	printPathReport(stdout, inspect(value))
	return 0
}

func printPathReport(stdout io.Writer, r pathdiag.Report) {
	newHuman(stdout).path(r)
}

func pathLabels(e pathdiag.Entry) (string, string, bool) {
	labels, style, problem := []string{}, good, false
	if e.Empty {
		labels = append(labels, "EMPTY")
		problem = true
	}
	switch {
	case e.Error != "":
		labels = append(labels, "ERROR")
		style, problem = bad, true
	case e.Missing:
		labels = append(labels, "MISSING")
		problem = true
	case !e.Directory:
		labels = append(labels, "NOT DIRECTORY")
		problem = true
	default:
		labels = append(labels, "OK")
	}
	if e.DuplicateOf > 0 {
		labels = append(labels, fmt.Sprintf("DUPLICATE #%d", e.DuplicateOf))
		problem = true
	}
	if problem && style != bad {
		style = warn
	}
	return strings.Join(labels, ", "), style, problem
}

func (h humanRenderer) path(r pathdiag.Report) {
	problems := []pathdiag.Entry{}
	for _, e := range r.Entries {
		if _, _, problem := pathLabels(e); problem {
			problems = append(problems, e)
		}
	}
	status, style := "no entry problems observed", good
	if len(problems) > 0 {
		status, style = fmt.Sprintf("%d entries need attention", len(problems)), warn
	}
	if len(r.Entries) == 0 {
		status, style = "empty PATH", warn
	}
	h.heading("PATH", status, style)
	h.section("Summary")
	h.item(fmt.Sprintf("%d entries; %d missing; %d duplicate", len(r.Entries), r.MissingCount, r.DuplicateCount), false, "")
	h.item(fmt.Sprintf("%d empty; %d not-directory; %d errors", r.EmptyCount, r.NotDirectoryCount, r.ErrorCount), true, "")
	h.section("Problems")
	if len(problems) == 0 {
		text := "No entry problems observed."
		if len(r.Entries) == 0 {
			text = "PATH is set but empty. There are no directories to search."
		}
		h.item(text, true, "")
	}
	for i, e := range problems {
		labels, style, _ := pathLabels(e)
		h.item(fmt.Sprintf("PATH #%d: %s", e.Index, labels), i == len(problems)-1, style)
		if e.Error != "" {
			h.line("     ", e.Error, "")
		}
	}
	if len(problems) > 0 {
		h.item("Missing/non-directory entries cannot supply commands; errors leave access uncertain. Empty entries use the current directory; duplicates repeat a directory. Review your shell's PATH configuration before making changes.", true, "")
	}
	h.section("Entries")
	for i, e := range r.Entries {
		labels, style, _ := pathLabels(e)
		h.item(fmt.Sprintf("PATH #%d  %s", e.Index, labels), i == len(r.Entries)-1, style)
		value := e.Value
		if value == "" {
			value = "<empty>"
		}
		h.line("     ", value, "")
		if e.Value != e.EffectiveValue {
			h.line("     ", fmt.Sprintf("raw: %q -> effective: %q", e.Value, e.EffectiveValue), quiet)
		}
		if e.Error != "" {
			h.line("     ", e.Error, "")
		}
	}
	h.section("Scope")
	h.item("PATH entries visible to this process; your shell's search rules may differ. Exact raw PATH is preserved in JSON.", true, quiet)
	h.section("Safe inspection")
	h.item("WhichWhy only inspected PATH. It did not change anything.", true, quiet)
}
