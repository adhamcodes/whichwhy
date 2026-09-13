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

}

func TestResolveExternalCollapsesRepeatedIdenticalCandidatePath(t *testing.T) {
	directory := t.TempDir()
	command := "whichwhy-repeated-path-test"
	writeTestCommand(t, directory, command)

	pathValue := strings.Join([]string{directory, directory}, string(os.PathListSeparator))
	result, err := resolveExternal(command, pathValue, testPathExt())
	if err != nil {
		t.Fatalf("resolveExternal() error = %v", err)
	}
	if got, want := len(result.Candidates), 1; got != want {
		t.Fatalf("candidate count = %d, want %d: %#v", got, want, result.Candidates)
	}
	if result.Candidates[0].DirectoryIndex != 0 {
		t.Fatalf("winner directory index = %d, want first occurrence 0", result.Candidates[0].DirectoryIndex)
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

func absolutePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
}
