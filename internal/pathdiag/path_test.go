package pathdiag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectPreservesOrderAndReportsProblems(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	value := strings.Join([]string{dir, missing, dir, "", file}, string(os.PathListSeparator))
	report := Inspect(value)

	if got, want := len(report.Entries), 5; got != want {
		t.Fatalf("entry count = %d, want %d", got, want)
	}
	if !report.Entries[0].Directory {
		t.Fatalf("first entry = %#v, want directory", report.Entries[0])
	}
	if !report.Entries[1].Missing {
		t.Fatalf("second entry = %#v, want missing", report.Entries[1])
	}
	if got, want := report.Entries[2].DuplicateOf, 1; got != want {
		t.Fatalf("third DuplicateOf = %d, want %d", got, want)
	}
	if !report.Entries[3].Empty {
		t.Fatalf("fourth entry = %#v, want empty", report.Entries[3])
	}
	if report.Entries[4].Directory || report.Entries[4].Missing || report.Entries[4].Error != "" {
		t.Fatalf("fifth entry = %#v, want existing non-directory", report.Entries[4])
	}

	if report.MissingCount != 1 || report.DuplicateCount != 1 || report.EmptyCount != 1 || report.NotDirectoryCount != 1 || report.ErrorCount != 0 {
		t.Fatalf("report counts = %#v", report)
	}
}

func TestInspectRetainsEntryObservationFailures(t *testing.T) {
	// A NUL cannot be put into an OS environment, but a supplied inspection
	// value must still retain the filesystem error rather than dropping its index.
	value := "bad\x00entry" + string(os.PathListSeparator) + t.TempDir()
	r := Inspect(value)
	if len(r.Entries) != 2 || r.Entries[0].Index != 1 || r.Entries[0].Value != "bad\x00entry" || r.Entries[0].Error == "" || r.ErrorCount != 1 || r.Entries[1].Index != 2 || !r.Entries[1].Directory {
		t.Fatalf("observation failure lost or shifted: %#v", r)
	}
}
