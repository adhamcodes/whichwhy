package pathdiag

import (
	"os"
	"path/filepath"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

// Entry is one parsed PATH segment and its directory diagnostics. Value keeps
// raw text; EffectiveValue is the actual Stat operand (including "." for empty
// interpreted entries). Empty is independent of Directory/Missing/Error.
type Entry struct {
	Index          int
	Value          string
	EffectiveValue string
	Directory      bool
	Missing        bool
	Empty          bool
	DuplicateOf    int
	Error          string
}

// Report is the ordered diagnostic view of the process-visible PATH.
type Report struct {
	RawValue          string
	Entries           []Entry
	MissingCount      int
	DuplicateCount    int
	EmptyCount        int
	NotDirectoryCount int
	ErrorCount        int
}

// Inspect examines PATH entries without modifying the environment or filesystem.
func Inspect(value string) Report {
	return InspectPath(processpath.Parse(value))
}

// InspectPath consumes the same parsed evidence as candidate discovery without
// reparsing, filtering, or renumbering entries. Only directory metadata is read.
func InspectPath(path processpath.Path) Report {
	report := Report{RawValue: path.Raw, Entries: make([]Entry, 0, len(path.Entries))}
	firstSeen := make(map[string]int, len(path.Entries))

	for _, part := range path.Entries {
		entry := Entry{Index: part.Index, Value: part.Raw, EffectiveValue: part.EffectiveValue()}
		if part.Value == "" {
			entry.Empty = true
			report.EmptyCount++
		}

		key := filepath.Clean(entry.EffectiveValue)
		if absolute, err := filepath.Abs(entry.EffectiveValue); err == nil {
			key = absolute
		}
		key = normalizeForComparison(key)
		if first, ok := firstSeen[key]; ok {
			entry.DuplicateOf = first
			report.DuplicateCount++
		} else {
			firstSeen[key] = entry.Index
		}

		info, err := os.Stat(entry.EffectiveValue)
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
