// Package processpath preserves the PATH evidence used by process-path-order-v1.
// Its parsing matches Go's native filepath.SplitList, not any shell's search rules.
package processpath

import (
	"os"
	"runtime"
	"strings"
)

// Path is one process PATH value and its ordered, unfiltered entry sequence.
type Path struct {
	Raw     string
	Entries []Entry
}

// Entry retains the original segment and the value interpreted by the process
// policy. Index is one-based and never changes when entries fail inspection or
// refer to the same directory. Relative values are not made absolute here.
type Entry struct {
	Index int
	Raw   string
	Value string
}

// EffectiveValue is the directory operand used for filesystem inspection.
func (e Entry) EffectiveValue() string {
	if e.Value == "" {
		return "."
	}
	return e.Value
}

func Parse(value string) Path {
	return parse(value, byte(os.PathListSeparator), runtime.GOOS == "windows")
}

// Keep raw spans while applying SplitList semantics. A single scan defines both
// raw and interpreted entry identities; native differential tests guard drift.
// Windows quotes toggle delimiter protection, including unmatched quotes; all
// double quotes are removed from the interpreted value. Unix quotes are literal.
func parse(value string, separator byte, windowsQuotes bool) Path {
	p := Path{Raw: value, Entries: []Entry{}}
	if value == "" {
		return p
	}
	appendEntry := func(raw string) {
		interpreted := raw
		if windowsQuotes {
			interpreted = strings.ReplaceAll(raw, `"`, "")
		}
		p.Entries = append(p.Entries, Entry{Index: len(p.Entries) + 1, Raw: raw, Value: interpreted})
	}
	start, quoted := 0, false
	for i := 0; i < len(value); i++ {
		if windowsQuotes && value[i] == '"' {
			quoted = !quoted
		} else if value[i] == separator && !quoted {
			appendEntry(value[start:i])
			start = i + 1
		}
	}
	appendEntry(value[start:])
	return p
}
