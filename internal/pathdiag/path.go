package pathdiag

import (
	"os"
	"path/filepath"
	"strings"
)

// Entry is one PATH segment and the diagnostics observed for it.
type Entry struct {
	Index       int
	Value       string
	Directory   bool
	Missing     bool
	Empty       bool
	DuplicateOf int
	Error       string
}

// Report is the ordered diagnostic view of the process-visible PATH.
type Report struct {
	Entries           []Entry
	MissingCount      int
	DuplicateCount    int
	EmptyCount        int
	NotDirectoryCount int
	ErrorCount        int
}

// Inspect examines PATH entries without modifying the environment or filesystem.
func Inspect(value string) Report {
	parts := strings.Split(value, string(os.PathListSeparator))
	report := Report{Entries: make([]Entry, 0, len(parts))}
	firstSeen := make(map[string]int, len(parts))

	for i, raw := range parts {
		entry := Entry{Index: i + 1, Value: raw}
		if raw == "" {
			entry.Empty = true
			report.EmptyCount++
			report.Entries = append(report.Entries, entry)
			continue
		}

		key := normalizeForComparison(filepath.Clean(raw))
		if first, ok := firstSeen[key]; ok {
			entry.DuplicateOf = first
			report.DuplicateCount++
		} else {
			firstSeen[key] = entry.Index
		}

		info, err := os.Stat(raw)
		switch {
		case err == nil:
			entry.Directory = info.IsDir()
			if !entry.Directory {
				report.NotDirectoryCount++
			}
		case os.IsNotExist(err):
			entry.Missing = true
			report.MissingCount++
		default:
			entry.Error = err.Error()
			report.ErrorCount++
		}

		report.Entries = append(report.Entries, entry)
	}

	return report
}
