package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolution"
)

type envLookup func(string) (string, bool)
type pathInspector func(string) pathdiag.Report

func runPath(stdout, stderr io.Writer, lookup envLookup, inspect pathInspector) int {
	value, ok := lookup("PATH")
	if !ok {
		fmt.Fprintln(stderr, "whichwhy: PATH is not set")
		return 1
	}

	printPathReport(stdout, inspect(value))
	return 0
}

func printPathReport(stdout io.Writer, report pathdiag.Report) {
	fmt.Fprintln(stdout, "WhichWhy — PATH")
	fmt.Fprintf(stdout, "\nPROCESS PARSING\n  %s (Go platform-native PATH parsing; shell search may differ).\n", resolution.ProcessPolicy)
	fmt.Fprintf(stdout, "\nRAW PATH\n  %q\n", report.RawValue)
	fmt.Fprintln(stdout, "\nPATH ENTRIES")

	for _, entry := range report.Entries {
		labels := make([]string, 0, 2)
		if entry.Empty {
			labels = append(labels, "EMPTY")
		}
		switch {
		case entry.Error != "":
			labels = append(labels, "ERROR")
		case entry.Missing:
			labels = append(labels, "MISSING")
		case !entry.Directory:
			labels = append(labels, "NOT DIRECTORY")
		default:
			labels = append(labels, "OK")
		}

		if entry.DuplicateOf > 0 {
			labels = append(labels, fmt.Sprintf("DUPLICATE #%d", entry.DuplicateOf))
		}

		value := entry.Value
		if value == "" {
			value = "<empty>"
		}
		fmt.Fprintf(stdout, "  %2d. %-24s %s\n", entry.Index, strings.Join(labels, ", "), value)
		if entry.Value != entry.EffectiveValue {
			fmt.Fprintf(stdout, "      raw: %q -> effective: %q\n", entry.Value, entry.EffectiveValue)
		}
		if entry.Error != "" {
			fmt.Fprintf(stdout, "      %s\n", entry.Error)
		}
	}

	fmt.Fprintln(stdout, "\nSUMMARY")
	fmt.Fprintf(stdout, "  %d entries · %d missing · %d duplicate · %d empty · %d not-directory · %d errors\n",
		len(report.Entries), report.MissingCount, report.DuplicateCount, report.EmptyCount, report.NotDirectoryCount, report.ErrorCount)
	fmt.Fprintln(stdout, "\nWHY IT MATTERS")
	fmt.Fprintln(stdout, "  The process policy inspects entries in order; empty entries use the current directory. Missing, duplicate, or non-directory entries can make command resolution harder to understand.")
	fmt.Fprintln(stdout, "\nSAFETY")
	fmt.Fprintln(stdout, "  WhichWhy only inspected PATH. It did not change anything.")
}
