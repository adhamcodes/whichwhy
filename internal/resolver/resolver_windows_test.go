//go:build windows

package resolver

import (
	"os"
	"path/filepath"
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
