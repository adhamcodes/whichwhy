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
