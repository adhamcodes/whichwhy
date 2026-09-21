package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

func TestTerminalCapabilityPolicy(t *testing.T) {
	for _, tc := range []struct {
		name  string
		facts terminalFacts
		env   map[string]string
		want  terminalOptions
	}{
		{"interactive", terminalFacts{true, true, true, 100}, nil, terminalOptions{true, true, 100}},
		{"no color", terminalFacts{true, true, true, 80}, map[string]string{"NO_COLOR": "1"}, terminalOptions{false, true, 80}},
		{"redirected despite terminal env", terminalFacts{false, true, true, 80}, map[string]string{"TERM": "xterm-256color", "FORCE_COLOR": "1"}, terminalOptions{}},
		{"dumb", terminalFacts{true, true, true, 80}, map[string]string{"TERM": "dumb"}, terminalOptions{}},
		{"legacy console", terminalFacts{true, false, false, 80}, nil, terminalOptions{false, false, 80}},
		{"ascii decorations", terminalFacts{true, true, false, 28}, nil, terminalOptions{true, false, 28}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := terminalPolicy(tc.facts, func(k string) string { return tc.env[k] })
			if got != tc.want {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			var out bytes.Buffer
			h := humanRenderer{out: &out, terminalOptions: got}
			h.heading("test", "attention", warn)
			h.section("Result")
			h.panel([]string{"test"})
			h.item("detail", true, quiet)
			if strings.Contains(out.String(), "\x1b") != tc.want.color {
				t.Fatalf("color mismatch: %q", out.String())
			}
			if strings.Contains(out.String(), "└") != tc.want.unicode {
				t.Fatalf("Unicode mismatch: %q", out.String())
			}
		})
	}
	for _, tc := range []struct {
		env  map[string]string
		want bool
	}{
		{map[string]string{"LANG": "en_US.UTF-8"}, true},
		{map[string]string{"LANG": "en_US.UTF-8", "LC_ALL": "C"}, false},
		{map[string]string{"LANG": "C", "LC_CTYPE": "C.utf8"}, true},
		{nil, false},
	} {
		if got := utf8Locale(func(k string) string { return tc.env[k] }); got != tc.want {
			t.Fatalf("locale %#v: %v", tc.env, got)
		}
	}
}

func TestRedirectedOutputUsesPlainPresentation(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("FORCE_COLOR", "1")
	f, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	for _, out := range []*os.File{f, w} {
		if got := detectTerminal(out); got != (terminalOptions{}) {
			t.Fatalf("redirected file styled: %#v", got)
		}
	}
	Run([]string{"--help"}, f, f, "test")
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.ContainsAny(data, "\x1b╭╰│") || !bytes.Contains(data, []byte(tagline)) {
		t.Fatalf("bad redirected help: %q", data)
	}
}

func TestHumanResultStates(t *testing.T) {
	failure := resolver.Observation{Attempt: 1, PathIndex: 1, Path: "/denied/probe", Status: resolver.ObservedError, Error: &resolver.ObservationError{Operation: "stat", Category: "permission-denied", Message: "controlled denial"}}
	candidate := resolver.Observation{Attempt: 2, PathIndex: 2, Path: "/bin/probe", Status: resolver.ObservedCandidate}
	for _, tc := range []struct {
		name         string
		candidates   []resolver.Candidate
		observations []resolver.Observation
		state        string
		code         int
	}{
		{"selected", []resolver.Candidate{{Path: "/bin/probe"}}, []resolver.Observation{candidate}, "SELECTED", 0},
		{"earlier failure", []resolver.Candidate{{Path: "/bin/probe"}}, []resolver.Observation{failure, candidate}, "OBSERVED CANDIDATE", 0},
		{"later failure", []resolver.Candidate{{Path: "/bin/probe"}}, []resolver.Observation{candidate, failure}, "SELECTED", 0},
		{"complete miss", nil, nil, "NOT FOUND", 1},
		{"incomplete miss", nil, []resolver.Observation{failure}, "NO DEFINITIVE RESULT", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := resolution.ProcessExternal(resolver.Result{Command: "probe", Candidates: tc.candidates, Observations: tc.observations})
			for _, opts := range []terminalOptions{{}, {true, true, 90}, {false, true, 30}, {true, false, 28}} {
				var out bytes.Buffer
				h := humanRenderer{out: &out, terminalOptions: opts}
				if code := h.command(r); code != tc.code {
					t.Fatalf("exit %d", code)
				}
				text := stripTestStyles(out.String())
				if !strings.Contains(text, "\n  "+tc.state+"\n") {
					t.Fatalf("wrong state: %s", text)
				}
				for _, state := range []string{"SELECTED", "OBSERVED CANDIDATE", "NOT FOUND", "NO DEFINITIVE RESULT"} {
					if state != tc.state && strings.Contains(text, "\n  "+state+"\n") {
						t.Fatalf("conflicting state: %s", text)
					}
				}
				if len(tc.observations) > 1 || tc.name == "incomplete miss" {
					if !strings.Contains(text, "controlled denial") || !strings.Contains(text, "incomplete") {
						t.Fatalf("lost failure: %s", text)
					}
				}
				if !strings.Contains(strings.Join(strings.Fields(text), " "), "may choose differently") {
					t.Fatalf("lost scope: %s", text)
				}
				first := out.String()
				out.Reset()
				h.command(r)
				if first != out.String() {
					t.Fatal("rendering is nondeterministic")
				}
			}
		})
	}
}

func TestHumanPowerShellSelectionAndMetadata(t *testing.T) {
	r := resolution.PowerShell(ps.Evidence{Command: "where", Version: "7.6.5", Edition: "Core", Matches: []ps.Match{
		{CommandType: "Alias", Name: "where", AliasTarget: "Where-Object", Source: "alias module"},
		{CommandType: "Application", Name: "where.exe", Path: `C:\Windows\System32\where.exe`, Source: "application source"},
	}})
	var out bytes.Buffer
	h := humanRenderer{out: &out, terminalOptions: terminalOptions{true, true, 90}}
	h.command(r)
	s := stripTestStyles(out.String())
	for _, want := range []string{"SELECTED", "Alias where -> Where-Object", "Source: alias module", "Shadowed in this session", `C:\Windows\System32\where.exe`, "Source: application source", "Current loaded PowerShell session", "PowerShell 7.6.5 (Core)", "Safe inspection", "╭"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q: %s", want, s)
		}
	}
	if strings.Index(s, "Alias where") > strings.Index(s, "where.exe") {
		t.Fatal("primary answer buried")
	}
	if strings.Contains(s, "Inspection  complete") || strings.Contains(s, "definitive within process") {
		t.Fatal("invented shell completeness")
	}
}

func TestHumanPATHDiagnosticsRetainEvidence(t *testing.T) {
	r := pathdiag.Report{RawValue: `;"folder with spaces";missing;missing;file;denied`, Entries: []pathdiag.Entry{
		{Index: 1, Value: "", EffectiveValue: ".", Empty: true, Directory: true},
		{Index: 2, Value: `"folder with spaces"`, EffectiveValue: "folder with spaces", Directory: true},
		{Index: 3, Value: "missing", EffectiveValue: "missing", Missing: true},
		{Index: 4, Value: "missing", EffectiveValue: "missing", Missing: true, DuplicateOf: 3},
		{Index: 5, Value: "file", EffectiveValue: "file"},
		{Index: 6, Value: "denied", EffectiveValue: "denied", Error: "permission denied"},
	}, MissingCount: 2, DuplicateCount: 1, EmptyCount: 1, NotDirectoryCount: 1, ErrorCount: 1}
	var out bytes.Buffer
	printPathReport(&out, r)
	s := out.String()
	for _, want := range []string{"5 entries need attention", "6 entries; 2 missing; 1 duplicate", "1 empty; 1 not-directory; 1 errors", "EMPTY, OK", "DUPLICATE #3", "NOT DIRECTORY", "ERROR", "permission denied", `raw: "" -> effective: "."`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q: %s", want, s)
		}
	}
	if !(strings.Index(s, "Summary") < strings.Index(s, "Problems") && strings.Index(s, "Problems") < strings.Index(s, "\n  Entries")) {
		t.Fatal("diagnostics not first")
	}
	for i := 1; i <= 6; i++ {
		if !strings.Contains(s, fmt.Sprintf("PATH #%d", i)) {
			t.Fatal("lost original index")
		}
	}
}

func TestHumanDoctorStatesAndAdvice(t *testing.T) {
	for _, state := range []doctorDiscovery{doctorDiscoveryCurrent, doctorDiscoveryDifferent, doctorDiscoveryMissing, doctorDiscoveryUncertain} {
		r := resolution.ProcessExternal(resolver.Result{Command: "whichwhy", Candidates: []resolver.Candidate{{Path: "/tools/whichwhy"}}})
		status, want := 1, "attention"
		if state == doctorDiscoveryCurrent {
			status, want = 0, "healthy"
		}
		var out bytes.Buffer
		printDoctorResult(&out, doctorReport{Discovery: state, Status: status, Resolution: r, RunningExecutable: "/running/whichwhy"})
		if !strings.Contains(out.String(), "[ "+want+" ]") {
			t.Fatal(out.String())
		}
		if status == 1 && !strings.Contains(out.String(), "Review") {
			t.Fatal("no action guidance")
		}
	}
	var out, stderr bytes.Buffer
	code := runDoctor(&out, &stderr, "dev", func() (string, error) { return "", errors.New("access denied") }, nil)
	if code != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "failure:") || !strings.Contains(stderr.String(), "retry") {
		t.Fatalf("failure state: %d %s %s", code, &out, &stderr)
	}
}

func TestSafeTextAdversarial(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"\x1b[2J\x1b]0;title\x07", `\x1b[2J\x1b]0;title\x07`},
		{"a\r\n\tb\b\x00\x7f\u009b", `a\r\n\tb\x08\x00\x7f\x9b`},
		{"a\u202eb\u2066c\u2028d\u2029\ufeff", `a\u202eb\u2066c\u2028d\u2029\ufeff`},
		{"bad\xff", `bad\xff`},
		{"tags\U000e0001", `tags\U000e0001`},
		{"λ界e\u0301", "λ界e\u0301"},
	} {
		if got := safeText(tc.input); got != tc.want {
			t.Fatalf("safeText(%q)=%q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestAllHumanEvidenceAndErrorsAreSanitized(t *testing.T) {
	attack := "hostile\x1b]52;c;payload\x07\r\n\u202e"
	r := resolution.PowerShell(ps.Evidence{Command: attack, Version: attack, Edition: attack, Matches: []ps.Match{{CommandType: attack, Name: attack, Source: attack, Path: attack, AliasTarget: attack}}})
	r.SelectionReason = attack
	r.Limitations = []string{attack}
	r.Scope = attack
	r.Policy = attack
	r.ClaimStrength = attack
	var out bytes.Buffer
	printCommandReport(&out, r)
	p := resolution.ProcessExternal(resolver.Result{Command: attack, Observations: []resolver.Observation{{Attempt: 1, PathIndex: 1, Path: attack, Status: resolver.ObservedError, Error: &resolver.ObservationError{Operation: attack, Category: attack, Message: attack}}}})
	printCommandReport(&out, p)
	printPathReport(&out, pathdiag.Report{RawValue: attack, Entries: []pathdiag.Entry{{Index: 1, Value: attack, EffectiveValue: attack, Error: attack}}})
	printDoctorResult(&out, doctorReport{Discovery: doctorDiscoveryMissing, Status: 1, Version: attack, OS: attack, Arch: attack, RunningExecutable: attack, Resolution: p})
	printError(&out, attack)
	Run([]string{"--version"}, &out, &out, attack)
	run([]string{"x"}, &out, &out, "", func(string) (resolver.Result, error) { return resolver.Result{}, errors.New(attack) })
	for _, r := range out.String() {
		if (unicode.IsControl(r) && r != '\n') || unicode.Is(unicode.Cf, r) {
			t.Fatalf("unsafe human rune %U", r)
		}
	}
	if !strings.Contains(out.String(), `\x1b]52;c;payload\x07\r\n\u202e`) {
		t.Fatal("sanitizer discarded evidence")
	}
	var machine, stderr bytes.Buffer
	printCommandJSON(&machine, &stderr, r)
	var doc jsonDocument
	if err := json.Unmarshal(machine.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Command != attack || doc.Selected.Path != attack || r.Command != attack {
		t.Fatal("human sanitization changed model/JSON")
	}
	// Styling is added after sanitization, including in the primary answer panel.
	out.Reset()
	humanRenderer{out: &out, terminalOptions: terminalOptions{true, true, 120}}.command(r)
	for _, rune := range stripTestStyles(out.String()) {
		if (unicode.IsControl(rune) && rune != '\n') || unicode.Is(unicode.Cf, rune) {
			t.Fatalf("hostile control survived styled rendering: %U", rune)
		}
	}
}

func TestNarrowAndLongIdentitiesRemainIntact(t *testing.T) {
	long := `C:\folder with spaces\` + strings.Repeat("long-directory-", 40) + `\tool.exe`
	r := resolution.ProcessExternal(resolver.Result{Command: long, Candidates: []resolver.Candidate{{Path: long}, {Path: long + "-other"}}})
	for _, width := range []int{12, 28, 40, 80} {
		var out bytes.Buffer
		h := humanRenderer{out: &out, terminalOptions: terminalOptions{false, true, width}}
		h.command(r)
		if !strings.Contains(out.String(), long+" (PATH #1)") || !strings.Contains(out.String(), long+"-other") {
			t.Fatal("path split or truncated")
		}
		if strings.ContainsAny(out.String(), "╭╰│") {
			t.Fatal("long answer forced into a panel")
		}
	}
	var out bytes.Buffer
	h := humanRenderer{out: &out, terminalOptions: terminalOptions{false, false, 28}}
	text := "These short words should wrap across several lines without losing evidence."
	h.item(text, true, "")
	var words []string
	for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		if len(line) > 28 {
			t.Fatalf("prose did not adapt: %q", line)
		}
		words = append(words, strings.Fields(strings.TrimPrefix(line, "  - "))...)
	}
	if strings.Join(words, " ") != text {
		t.Fatal("wrapping changed prose")
	}
}

func TestJSONNeverInheritsHumanPresentation(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("NO_COLOR", "")
	t.Setenv("PATH", t.TempDir())
	for _, args := range [][]string{{"missing", "--json"}, {"inspect", "missing", "--json"}, {"path", "--json"}, {"doctor", "--json"}, {"__powershell-json", encodeTestCommand("missing"), "7", "Core"}} {
		var out, stderr bytes.Buffer
		code := Run(args, &out, &stderr, "test")
		if code > 1 || stderr.Len() != 0 {
			t.Fatalf("%v: %d %s", args, code, &stderr)
		}
		var doc map[string]any
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if bytes.ContainsAny(out.Bytes(), "\x1b╭╰│") || bytes.Contains(out.Bytes(), []byte("WhichWhy")) {
			t.Fatalf("contaminated JSON: %s", &out)
		}
	}
	// Even rendering a styled report first must leave the original machine model
	// and its serialized document exactly unchanged.
	r := resolution.PowerShell(ps.Evidence{Command: "where", Matches: []ps.Match{{CommandType: "Alias", Name: "where", AliasTarget: "Where-Object"}}})
	before := commandJSONDocument(r)
	var out bytes.Buffer
	humanRenderer{out: &out, terminalOptions: terminalOptions{true, true, 80}}.command(r)
	if !reflect.DeepEqual(before, commandJSONDocument(r)) {
		t.Fatal("presentation mutated machine model")
	}
}

func stripTestStyles(s string) string {
	for _, code := range []string{"0", "2", "31", "32", "33"} {
		s = strings.ReplaceAll(s, "\x1b["+code+"m", "")
	}
	return s
}
