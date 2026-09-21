package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Uses only pre-F7 APIs so it can also run against the parent commit. The real
// collector encounters a malformed/overlong path component; no ACL setup or test double is
// needed. Neither human nor JSON inspection may execute the fixture.
func TestFilesystemObservationSilentLossRegression(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writePATHFixture(t, root)
	t.Setenv("PATHEXT", ".CMD")
	badEntry := filepath.Join(root, strings.Repeat("x", 300))
	if runtime.GOOS == "windows" {
		badEntry = filepath.Join(root, "bad<entry")
	}
	for _, tc := range []struct {
		name                    string
		entries                 []string
		completeness, selection string
		code                    int
	}{
		{"complete-miss", []string{filepath.Join(root, "missing")}, "complete", "no-candidate", 1},
		{"incomplete-miss", []string{badEntry}, "incomplete", "no-candidate", 1},
		{"failure-before", []string{badEntry, root}, "incomplete", "precedence-uncertain", 0},
		{"failure-after", []string{root, badEntry}, "incomplete", "definitive", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := strings.Join(tc.entries, string(os.PathListSeparator))
			t.Setenv("PATH", value)
			var human, machine, stderr bytes.Buffer
			if code := Run([]string{"wwpathfixture"}, &human, &stderr, "test"); code != tc.code {
				t.Fatalf("human code=%d %s", code, &stderr)
			}
			if code := Run([]string{"wwpathfixture", "--json"}, &machine, &stderr, "test"); code != tc.code || stderr.Len() != 0 {
				t.Fatalf("JSON code=%d %s", code, &stderr)
			}
			var doc struct {
				Inspection *struct {
					Completeness string `json:"completeness"`
					Observations []struct {
						Attempt   int    `json:"attempt"`
						PathIndex int    `json:"path_index"`
						Status    string `json:"status"`
						Error     *struct {
							Category string `json:"category"`
						} `json:"error"`
					} `json:"observations"`
				} `json:"inspection"`
				Selection         string `json:"selection_status"`
				Reason            string `json:"selection_reason"`
				NoCandidateReason string `json:"no_candidate_reason"`
			}
			if err := json.Unmarshal(machine.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Inspection == nil {
				t.Fatalf("filesystem observation evidence silently lost: %s", &machine)
			}
			if doc.Inspection.Completeness != tc.completeness || doc.Selection != tc.selection || len(doc.Inspection.Observations) != len(tc.entries) {
				t.Fatalf("wrong evidence: %s", &machine)
			}
			for i, o := range doc.Inspection.Observations {
				if o.Attempt != i+1 || o.PathIndex != i+1 {
					t.Fatalf("lost index: %s", &machine)
				}
				if tc.entries[i] == badEntry && (o.Status != "error" || o.Error == nil || o.Error.Category != "other") {
					t.Fatalf("lost failure: %s", &machine)
				}
			}
			for _, text := range []string{tc.completeness, humanSelection(tc.selection), humanReason(doc.Reason), humanReason(doc.NoCandidateReason)} {
				if !strings.Contains(human.String(), text) {
					t.Fatalf("human/JSON disagree: %s / %s", &human, &machine)
				}
			}
			if tc.completeness == "incomplete" && !strings.Contains(human.String(), filepath.Base(badEntry)) {
				t.Fatalf("failure operand hidden: %s", &human)
			}
			if tc.selection == "precedence-uncertain" && strings.Contains(human.String(), "First eligible external command") {
				t.Fatalf("overclaim: %s", &human)
			}
			if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
				t.Fatalf("inspection executed fixture: %v", err)
			}
			if os.Getenv("PATH") != value {
				t.Fatal("inspection mutated PATH")
			}
		})
	}
}
