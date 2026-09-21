package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestProcessPATHEmptyRelativeDuplicateCorrelation(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, dir := range []string{root, filepath.Join(root, "relative")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writePATHFixture(t, dir)
	}
	if err := os.WriteFile("not-directory", []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	separator := string(os.PathListSeparator)
	for _, tc := range []struct {
		name       string
		entries    []string
		indices    []int
		duplicates map[int]int
	}{
		{"empty-path", []string{}, []int{}, nil},
		{"empty-segments", []string{"", "", ".", root, ""}, []int{1}, map[int]int{2: 1, 3: 1, 4: 1, 5: 1}},
		{"relative-first", []string{"missing", "relative", filepath.Join(root, "relative"), "", ".", "relative"}, []int{2, 4}, map[int]int{3: 2, 5: 4, 6: 2}},
		{"absolute-first", []string{filepath.Join(root, "relative"), "relative", "", root}, []int{1, 3}, map[int]int{2: 1, 4: 3}},
		{"skipped-entries", []string{"missing", "not-directory", "relative", "relative"}, []int{3}, map[int]int{4: 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := strings.Join(tc.entries, separator)
			d := assertPATHCorrelation(t, value, tc.indices)
			if len(d.Entries) != len(tc.entries) {
				t.Fatalf("entries=%#v", d.Entries)
			}
			for i, e := range d.Entries {
				if e.Value != tc.entries[i] || e.Empty != (tc.entries[i] == "") || e.DuplicateOf != tc.duplicates[e.Index] {
					t.Fatalf("entry identity changed: %#v", e)
				}
				if e.Empty && (!e.Directory || e.EffectiveValue != ".") {
					t.Fatalf("empty entry not inspected as cwd: %#v", e)
				}
			}
		})
	}
}

func writePATHFixture(t *testing.T, dir string) {
	t.Helper()
	filename, body := "wwpathfixture", "#!/bin/sh\nprintf executed > inspected.marker\n"
	if runtime.GOOS == "windows" {
		filename += ".CMD"
		body = "@echo off\r\necho executed>inspected.marker\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertPATHCorrelation(t *testing.T, value string, indices []int) pathdiag.Report {
	t.Helper()
	t.Setenv("PATH", value)
	t.Setenv("PATHEXT", ".CMD")
	evidence, err := resolver.ResolveExternal("wwpathfixture")
	if err != nil {
		t.Fatal(err)
	}
	r := resolution.ProcessExternal(evidence)
	if r.ProcessPath == nil || !reflect.DeepEqual(*r.ProcessPath, evidence.Path) || r.ProcessPath.Raw != value {
		t.Fatalf("resolution report lost parsed PATH: %#v", r.ProcessPath)
	}
	d := pathdiag.InspectPath(*r.ProcessPath)
	if !reflect.DeepEqual(d, pathdiag.Inspect(value)) {
		t.Fatal("standalone diagnostics disagree with collected PATH")
	}
	if len(r.Candidates) != len(indices) || len(d.Entries) != len(evidence.Path.Entries) {
		t.Fatalf("candidates=%#v entries=%#v", r.Candidates, d.Entries)
	}
	for i, c := range r.Candidates {
		if c.PathIndex != indices[i] {
			t.Fatalf("candidate index=%d want %d", c.PathIndex, indices[i])
		}
		e := d.Entries[c.PathIndex-1]
		want, err := filepath.Abs(filepath.Join(e.EffectiveValue, filepath.Base(c.Path)))
		if err != nil || c.Path != want || e.Index != c.PathIndex || !e.Directory || e.DuplicateOf != 0 {
			t.Fatalf("candidate %#v points at wrong entry %#v: %v", c, e, err)
		}
	}
	assertReportPresentations(t, r)
	var human, machine, stderr bytes.Buffer
	lookup := func(string) (string, bool) { return value, true }
	if runPath(&human, &stderr, lookup, pathdiag.Inspect) != 0 || runPathJSON(&machine, &stderr, lookup, pathdiag.Inspect) != 0 || stderr.Len() != 0 {
		t.Fatalf("PATH presentation failed: %s", &stderr)
	}
	var doc jsonPathDocument
	if err := json.Unmarshal(machine.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc, pathJSONDocument(d)) || doc.RawValue != value || doc.Policy != "process-path-order-v1" || doc.SchemaVersion != 1 {
		t.Fatalf("JSON evidence mismatch: %s", &machine)
	}
	for _, text := range []string{fmt.Sprintf("%q", value), doc.Policy, "shell search may differ", fmt.Sprintf("%d entries; %d missing; %d duplicate", doc.Summary.Entries, doc.Summary.Missing, doc.Summary.Duplicate), fmt.Sprintf("%d empty; %d not-directory; %d errors", doc.Summary.Empty, doc.Summary.NotDirectory, doc.Summary.Errors)} {
		if !strings.Contains(human.String(), text) {
			t.Fatalf("human omitted %q: %s", text, &human)
		}
	}
	for _, e := range doc.Entries {
		shown := e.Value
		if shown == "" {
			shown = "<empty>"
		}
		labels := []string{}
		if e.Empty {
			labels = append(labels, "EMPTY")
		}
		switch {
		case e.Error != "":
			labels = append(labels, "ERROR")
		case e.Missing:
			labels = append(labels, "MISSING")
		case !e.Directory:
			labels = append(labels, "NOT DIRECTORY")
		default:
			labels = append(labels, "OK")
		}
		if e.DuplicateOf > 0 {
			labels = append(labels, fmt.Sprintf("DUPLICATE #%d", e.DuplicateOf))
		}
		if !strings.Contains(human.String(), fmt.Sprintf("PATH #%d  %s\n     %s", e.Index, strings.Join(labels, ", "), shown)) {
			t.Fatalf("human/JSON entry divergence: %#v / %s", e, &human)
		}
		if e.Value != e.EffectiveValue && !strings.Contains(human.String(), fmt.Sprintf("raw: %q -> effective: %q", e.Value, e.EffectiveValue)) {
			t.Fatalf("human omitted interpretation: %#v / %s", e, &human)
		}
	}
	if os.Getenv("PATH") != value {
		t.Fatal("inspection mutated process PATH")
	}
	if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
		t.Fatalf("inspection executed fixture or marker check failed: %v", err)
	}
	return d
}
