package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

func TestNormalHumanScopeIsNaturalAndConcise(t *testing.T) {
	process := resolution.ProcessExternal(resolver.Result{Command: "probe", Candidates: []resolver.Candidate{{Path: "/bin/probe"}}})
	shell := resolution.PowerShell(ps.Evidence{Command: "where", Version: "7", Edition: "Core", Matches: []ps.Match{{CommandType: "Alias", Name: "where", AliasTarget: "Where-Object"}}})
	for _, tc := range []struct {
		name   string
		render func(*bytes.Buffer)
		want   []string
	}{
		{"process", func(out *bytes.Buffer) { printCommandReport(out, process) }, []string{"External commands visible to this process", "PATH order", "aliases, functions, built-ins, cached commands", "search rules may choose differently", "does not guarantee execution"}},
		{"shell", func(out *bytes.Buffer) { printCommandReport(out, shell) }, []string{"Current loaded PowerShell session", "Unloaded modules may add commands", "does not guarantee execution or a working alias target"}},
		{"doctor", func(out *bytes.Buffer) {
			printDoctorResult(out, doctorReport{Discovery: doctorDiscoveryCurrent, Resolution: process})
		}, []string{"healthy", "External commands visible to this process", "may choose differently", "does not guarantee execution"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			tc.render(&out)
			text := out.String()
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Fatalf("missing %q: %s", want, text)
				}
			}
			for _, internal := range []string{resolution.ProcessScope, resolution.ProcessPolicy, resolution.PolicyOnly, resolution.PowerShellPolicy, resolution.ShellObserved, "\n  Limits\n", "mode-class", "UID"} {
				if strings.Contains(text, internal) {
					t.Fatalf("normal UX contains %q: %s", internal, text)
				}
			}
			if strings.Count(text, "\n") > 32 || len(strings.Fields(text)) > 165 {
				t.Fatalf("normal report became a limitation wall: %s", text)
			}
		})
	}
}

func TestHumanSummariesKeepUnfamiliarEvidenceAndOriginalJSON(t *testing.T) {
	r := resolution.ProcessExternal(resolver.Result{Command: "probe", Candidates: []resolver.Candidate{{Path: "/first/probe"}, {Path: "/second/probe"}}})
	r.Selected = &r.Candidates[1]
	r.Alternatives = r.Candidates[:1]
	r.SelectionReason = "A supplied explanation that the renderer must not replace."
	r.Limitations = append(r.Limitations, "Result-specific warning: metadata is unavailable.")
	var before, after, human, stderr bytes.Buffer
	printCommandJSON(&before, &stderr, r)
	printCommandReport(&human, r)
	printCommandJSON(&after, &stderr, r)
	if !bytes.Equal(before.Bytes(), after.Bytes()) || stderr.Len() != 0 {
		t.Fatal("human summary changed JSON")
	}
	for _, want := range []string{r.SelectionReason, "Result-specific warning: metadata is unavailable."} {
		if !strings.Contains(human.String(), want) {
			t.Fatalf("unfamiliar evidence hidden: %s", &human)
		}
	}
	if strings.Index(human.String(), "/second/probe") > strings.Index(human.String(), "/first/probe") {
		t.Fatal("renderer reselected the first candidate")
	}
	for _, limit := range r.Limitations {
		if !strings.Contains(before.String(), limit) {
			t.Fatalf("JSON lost %q", limit)
		}
	}
}

func TestConciseReasonsStillExplainUncertainty(t *testing.T) {
	failure := resolver.Observation{Attempt: 1, PathIndex: 1, Path: "/denied/probe", Status: resolver.ObservedError, Error: &resolver.ObservationError{Operation: "stat", Category: "permission-denied", Message: "access denied"}}
	hit := resolver.Observation{Attempt: 2, PathIndex: 2, Path: "/bin/probe", Status: resolver.ObservedCandidate}
	for _, tc := range []struct {
		name         string
		candidates   []resolver.Candidate
		observations []resolver.Observation
		want         []string
	}{
		{"earlier failure", []resolver.Candidate{{Path: hit.Path}}, []resolver.Observation{failure, hit}, []string{"OBSERVED CANDIDATE", "uncertain order", "earlier failed lookup could hide a command that takes priority", "access denied"}},
		{"later failure", []resolver.Candidate{{Path: hit.Path}}, []resolver.Observation{hit, failure}, []string{"SELECTED", "definitive in this PATH search", "inspection incomplete", "access denied"}},
		{"incomplete miss", nil, []resolver.Observation{failure}, []string{"NO DEFINITIVE RESULT", "failed lookups may hide matching commands", "access denied"}},
		{"complete miss", nil, nil, []string{"NOT FOUND", "Your shell may still find it", "Inspection  complete"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			printCommandReport(&out, resolution.ProcessExternal(resolver.Result{Command: "probe", Candidates: tc.candidates, Observations: tc.observations}))
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q: %s", want, &out)
				}
			}
		})
	}
}

func TestPrimaryAnswerFallbackHierarchy(t *testing.T) {
	path := `C:\Tools with spaces\probe.exe`
	r := resolution.ProcessExternal(resolver.Result{Command: "probe", Candidates: []resolver.Candidate{{Path: path}}})
	for _, opts := range []terminalOptions{{true, true, 100}, {true, false, 80}, {}, {false, true, 24}} {
		var out bytes.Buffer
		humanRenderer{out: &out, terminalOptions: opts}.command(r)
		text := stripTestStyles(out.String())
		if !strings.Contains(text, path) {
			t.Fatal("answer lost")
		}
		if opts.unicode && opts.width == 100 {
			if !strings.Contains(text, "╭") {
				t.Fatal("Unicode panel lost")
			}
		} else {
			if !strings.Contains(text, "SELECTED\n\n  > "+path) {
				t.Fatalf("fallback has no primary hierarchy: %s", text)
			}
			if strings.ContainsAny(text, "╭╰│") {
				t.Fatal("fallback forced box drawing")
			}
		}
		if strings.Contains(out.String(), "\x1b") != opts.color {
			t.Fatal("fallback changed ANSI eligibility")
		}
	}
}

func TestPlainPowerShellAnswerPrecedesShadowedCommand(t *testing.T) {
	r := resolution.PowerShell(ps.Evidence{Command: "where", Version: "7", Edition: "Core", Matches: []ps.Match{
		{CommandType: "Alias", Name: "where", AliasTarget: "Where-Object"},
		{CommandType: "Application", Name: "where.exe", Path: `C:\Windows\System32\where.exe`},
	}})
	var out bytes.Buffer
	printCommandReport(&out, r)
	s := out.String()
	previous := -1
	for _, piece := range []string{"[ PowerShell session ]", "SELECTED\n\n  > Alias where -> Where-Object", "\n  Why\n", "Shadowed in this session", "Application where.exe", `C:\Windows\System32\where.exe`, "Current loaded PowerShell session"} {
		position := strings.Index(s, piece)
		if position <= previous {
			t.Fatalf("plain hierarchy lost %q: %s", piece, s)
		}
		previous = position
	}
	if strings.ContainsAny(s, "\x1b╭╰│") {
		t.Fatal("plain report acquired terminal styling")
	}
}

func TestPATHRawEvidenceIsPerEntryInHumanAndWholeInJSON(t *testing.T) {
	r := pathdiag.Report{RawValue: `;"folder;with separator";last;`, Entries: []pathdiag.Entry{
		{Index: 1, Value: "", EffectiveValue: ".", Empty: true, Directory: true},
		{Index: 2, Value: `"folder;with separator"`, EffectiveValue: "folder;with separator", Directory: true},
		{Index: 3, Value: "last", EffectiveValue: "last", Missing: true},
		{Index: 4, Value: "", EffectiveValue: ".", Empty: true, Directory: true, DuplicateOf: 1},
	}, EmptyCount: 2, MissingCount: 1, DuplicateCount: 1}
	var human bytes.Buffer
	printPathReport(&human, r)
	s := human.String()
	if strings.Contains(s, "\n  Raw PATH\n") || strings.Contains(s, fmt.Sprintf("%q", r.RawValue)) {
		t.Fatal("redundant raw PATH wall")
	}
	for _, want := range []string{"PATH #1  EMPTY, OK", "PATH #2  OK", "PATH #3  MISSING", "PATH #4  EMPTY, OK, DUPLICATE #1", `raw: "" -> effective: "."`, fmt.Sprintf("raw: %q -> effective: %q", r.Entries[1].Value, r.Entries[1].EffectiveValue), "Exact raw PATH is preserved in JSON"} {
		if !strings.Contains(s, want) {
			t.Fatalf("lost %q: %s", want, s)
		}
	}
	doc := pathJSONDocument(r)
	if doc.RawValue != r.RawValue || len(doc.Entries) != len(r.Entries) {
		t.Fatal("JSON raw evidence changed")
	}
	for i, entry := range r.Entries {
		if doc.Entries[i].Value != entry.Value || doc.Entries[i].EffectiveValue != entry.EffectiveValue {
			t.Fatal("JSON entry evidence changed")
		}
	}
}
