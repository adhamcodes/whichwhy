package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
)

func TestRunPathPrintsDiagnostics(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	lookup := func(name string) (string, bool) {
		if name != "PATH" {
			t.Fatalf("lookup name = %q, want PATH", name)
		}
		return "ignored", true
	}
	inspect := func(value string) pathdiag.Report {
		if value != "ignored" {
			t.Fatalf("inspect value = %q, want ignored", value)
		}
		return pathdiag.Report{
			Entries: []pathdiag.Entry{
				{Index: 1, Value: `/first`, Directory: true},
				{Index: 2, Value: `/missing`, Missing: true},
				{Index: 3, Value: `/first`, Directory: true, DuplicateOf: 1},
			},
			MissingCount:   1,
			DuplicateCount: 1,
		}
	}

	code := runPath(&stdout, &stderr, lookup, inspect)
	if code != 0 {
		t.Fatalf("runPath() exit code = %d, want 0", code)
	}
	output := stdout.String()
	for _, want := range []string{"PATH ENTRIES", "MISSING", "DUPLICATE #1", "SUMMARY", "did not change anything"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunPathReportsUnsetPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runPath(&stdout, &stderr, func(string) (string, bool) { return "", false }, pathdiag.Inspect)
	if code != 1 {
		t.Fatalf("runPath() exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "PATH is not set") {
		t.Fatalf("stderr = %q, want unset PATH message", stderr.String())
	}
}
