//go:build windows

package resolver

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWindowsResolverPreservesPathExtOrderWithinDirectory(t *testing.T) {
	directory := t.TempDir()
	command := "whichwhy-path-ext-test"

	cmdPath := filepath.Join(directory, command+".CMD")
	exePath := filepath.Join(directory, command+".EXE")
	for _, path := range []string{cmdPath, exePath} {
		if err := os.WriteFile(path, []byte("test"), 0o755); err != nil {
			t.Fatalf("write test command %q: %v", path, err)
		}
	}

	result, err := resolveExternal(command, directory, ".CMD;.EXE")
	if err != nil {
		t.Fatalf("resolveExternal() error = %v", err)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(result.Candidates))
	}
	if got, want := result.Candidates[0].Path, absolutePath(cmdPath); got != want {
		t.Fatalf("first candidate = %q, want %q", got, want)
	}
	if got, want := result.Candidates[1].Path, absolutePath(exePath); got != want {
		t.Fatalf("second candidate = %q, want %q", got, want)
	}
}

func TestWindowsCandidateNamesPATHEXT(t *testing.T) {
	for _, tc := range []struct {
		name, command, pathExt string
		want                   []string
	}{
		{"extensionless", "wwtool", ".EXE;.CMD", []string{"wwtool.EXE", "wwtool.CMD"}},
		{"dotted", "wwtool.v1", ".EXE;.CMD", []string{"wwtool.v1", "wwtool.v1.EXE", "wwtool.v1.CMD"}},
		{"multi-dot", "wwtool.alpha.v1", ".CMD;.EXE", []string{"wwtool.alpha.v1", "wwtool.alpha.v1.CMD", "wwtool.alpha.v1.EXE"}},
		{"recognized", "wwtool.cmd", ".EXE;.CMD", []string{"wwtool.cmd"}},
		{"recognized-case", "wwtool.CmD", ".EXE;.cMd", []string{"wwtool.CmD"}},
		{"custom-recognized", "wwtool.v1", ".V1;.CMD", []string{"wwtool.v1"}},
		{"not-in-pathext", "wwtool.cmd", ".EXE", []string{"wwtool.cmd", "wwtool.cmd.EXE"}},
		{"normalize-and-dedup", "wwtool.v1", " ; cmd ;.EXE;.CMD;exe;; ", []string{"wwtool.v1", "wwtool.v1.cmd", "wwtool.v1.EXE"}},
		{"normalized-recognized", "wwtool.CmD", " cmd ;EXE", []string{"wwtool.CmD"}},
		{"default-dotted", "wwtool.v1", " ; ", []string{"wwtool.v1", "wwtool.v1.COM", "wwtool.v1.EXE", "wwtool.v1.BAT", "wwtool.v1.CMD"}},
		{"default-recognized", "wwtool.CmD", "", []string{"wwtool.CmD"}},
		{"default-extensionless", "wwtool", "", []string{"wwtool.COM", "wwtool.EXE", "wwtool.BAT", "wwtool.CMD"}},
		{"trailing-dot", "wwtool.", ".CMD", []string{"wwtool.", "wwtool..CMD"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := windowsCandidateNames(tc.command, tc.pathExt); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("names = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWindowsDottedCommandRegression(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wwtool.v1.cmd")
	if err := os.WriteFile(path, []byte("@echo off\r\necho executed>inspected.marker\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	result, err := resolveExternal("wwtool.v1", dir, ".EXE;.CMD")
	if err != nil || len(result.Candidates) != 1 {
		t.Fatalf("dotted command missed: %#v, %v", result, err)
	}
	if candidatePathKey(result.Candidates[0].Path) != candidatePathKey(path) {
		t.Fatalf("candidate = %#v", result.Candidates[0])
	}
	if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
		t.Fatalf("inspection executed fixture or marker check failed: %v", err)
	}
}
