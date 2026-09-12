//go:build windows

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestWindowsQuotedPATHCorrelation(t *testing.T) {
	root := t.TempDir()
	quoted := filepath.Join(root, "quoted;directory")
	last := filepath.Join(root, "last")
	for _, dir := range []string{quoted, last} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "wwpathfixture.cmd"), []byte("@echo off\r\necho executed>inspected.marker\r\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	value := `"` + quoted + `";` + last
	t.Setenv("PATH", value)
	t.Setenv("PATHEXT", ".CMD")
	// Lock the original audit failure into the fixture: the old diagnostics
	// split this into three entries, while the R1 resolver sees two.
	if len(strings.Split(value, ";")) != 3 || len(filepath.SplitList(value)) != 2 {
		t.Fatal("fixture no longer reproduces the old 2-versus-3 parser mismatch")
	}
	evidence, err := resolver.ResolveExternal("wwpathfixture")
	if err != nil {
		t.Fatal(err)
	}
	report := resolution.ProcessExternal(evidence)
	diagnostics := pathdiag.Inspect(value)
	if len(diagnostics.Entries) != 2 || len(report.Candidates) != 2 {
		t.Fatalf("resolver candidates=%d; diagnostic entries=%d; want 2 each", len(report.Candidates), len(diagnostics.Entries))
	}
	for i, candidate := range report.Candidates {
		entry := diagnostics.Entries[i]
		if candidate.PathIndex != entry.Index || !entry.Directory {
			t.Fatalf("candidate %#v does not correlate with directory entry %#v", candidate, entry)
		}
	}
	assertPATHCorrelation(t, value, []int{1, 2})
	if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
		t.Fatalf("inspection executed fixture or marker check failed: %v", err)
	}
}

func TestWindowsQuotedPATHEmptyRelativeDuplicates(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, dir := range []string{root, filepath.Join(root, "relative;dir"), filepath.Join(root, "last")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writePATHFixture(t, dir)
	}
	// The quoted empty, literal empty, and dot all denote cwd. Raw quotes and
	// relative spelling must survive even when effective directories duplicate.
	value := `"relative;dir";"";;.;"` + filepath.Join(root, "relative;dir") + `";last;"last";`
	d := assertPATHCorrelation(t, value, []int{1, 2, 6})
	wantRaw := []string{`"relative;dir"`, `""`, "", ".", `"` + filepath.Join(root, "relative;dir") + `"`, "last", `"last"`, ""}
	wantEffective := []string{"relative;dir", ".", ".", ".", filepath.Join(root, "relative;dir"), "last", "last", "."}
	wantDuplicate := []int{0, 0, 2, 2, 1, 0, 6, 2}
	if len(d.Entries) != 8 || d.EmptyCount != 3 || d.DuplicateCount != 5 {
		t.Fatalf("unexpected diagnostics: %#v", d)
	}
	for i, e := range d.Entries {
		if e.Value != wantRaw[i] || e.EffectiveValue != wantEffective[i] || e.DuplicateOf != wantDuplicate[i] {
			t.Fatalf("entry #%d: %#v", i+1, e)
		}
	}
}

func TestWindowsPATHDriveRelativeCorrelation(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	dir := filepath.Join(root, "relative")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePATHFixture(t, dir)
	driveRelative := filepath.VolumeName(root) + "relative"
	value := driveRelative + ";" + dir + ";relative"
	d := assertPATHCorrelation(t, value, []int{1})
	if len(d.Entries) != 3 || d.Entries[0].Value != driveRelative || d.Entries[0].EffectiveValue != driveRelative || d.Entries[1].DuplicateOf != 1 || d.Entries[2].DuplicateOf != 1 {
		t.Fatalf("drive-relative identity lost: %#v", d)
	}
}
