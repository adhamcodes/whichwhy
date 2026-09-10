package resolver

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveExternalPreservesDirectoryOrder(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	command := "whichwhy-resolver-test"

	writeTestCommand(t, first, command)
	writeTestCommand(t, second, command)

	pathValue := strings.Join([]string{first, second}, string(os.PathListSeparator))
	result, err := resolveExternal(command, pathValue, testPathExt())
	if err != nil {
		t.Fatalf("resolveExternal() error = %v", err)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(result.Candidates))
	}

	wantFirst := absolutePath(filepath.Join(first, testCommandFilename(command)))
	wantSecond := absolutePath(filepath.Join(second, testCommandFilename(command)))
	if result.Candidates[0].Path != wantFirst {
		t.Fatalf("first candidate = %q, want %q", result.Candidates[0].Path, wantFirst)
	}
	if result.Candidates[1].Path != wantSecond {
		t.Fatalf("second candidate = %q, want %q", result.Candidates[1].Path, wantSecond)
	}

	winner, ok := result.Winner()
	if !ok {
		t.Fatal("Winner() reported no winner")
	}
	if winner.Path != wantFirst {
		t.Fatalf("winner = %q, want %q", winner.Path, wantFirst)
	}
}

func TestResolveExternalReturnsNoCandidatesWhenCommandIsMissing(t *testing.T) {
	result, err := resolveExternal("definitely-not-present", t.TempDir(), testPathExt())
	if err != nil {
		t.Fatalf("resolveExternal() error = %v", err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("candidate count = %d, want 0", len(result.Candidates))
	}
	if _, ok := result.Winner(); ok {
		t.Fatal("Winner() reported a winner for a missing command")
	}
}

func TestResolveExternalRejectsEmptyCommand(t *testing.T) {
	_, err := resolveExternal("", t.TempDir(), testPathExt())
	if !errors.Is(err, ErrEmptyCommand) {
		t.Fatalf("error = %v, want ErrEmptyCommand", err)
	}
}

func TestResolveExternalRejectsExplicitPathForNow(t *testing.T) {
	command := filepath.Join("somewhere", "tool")
	_, err := resolveExternal(command, t.TempDir(), testPathExt())
	if !errors.Is(err, ErrExplicitPathUnsupported) {
		t.Fatalf("error = %v, want ErrExplicitPathUnsupported", err)
	}
}

func TestUnixResolverSkipsNonExecutableFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable permission test")
	}

	directory := t.TempDir()
	command := "whichwhy-not-executable"
	path := filepath.Join(directory, command)
	if err := os.WriteFile(path, []byte("not executable"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	result, err := resolveExternal(command, directory, "")
	if err != nil {
		t.Fatalf("resolveExternal() error = %v", err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("candidate count = %d, want 0", len(result.Candidates))
	}
}

func writeTestCommand(t *testing.T, directory, command string) {
	t.Helper()
	path := filepath.Join(directory, testCommandFilename(command))
	if err := os.WriteFile(path, []byte("test"), 0o755); err != nil {
		t.Fatalf("write test command: %v", err)
	}
}

func testCommandFilename(command string) string {
	if runtime.GOOS == "windows" {
		return command + ".EXE"
	}
	return command
}

func testPathExt() string {
	if runtime.GOOS == "windows" {
		return ".EXE"
	}
	return ""
}
