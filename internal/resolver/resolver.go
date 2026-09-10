package resolver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrEmptyCommand            = errors.New("command name is empty")
	ErrExplicitPathUnsupported = errors.New("explicit command paths are not supported yet")
)

// Candidate is an external command found through the process-visible search path.
type Candidate struct {
	Path           string
	DirectoryIndex int
}

// Result contains the ordered external candidates for one command name.
type Result struct {
	Command    string
	Candidates []Candidate
}

// Winner returns the first external candidate in search order.
func (r Result) Winner() (Candidate, bool) {
	if len(r.Candidates) == 0 {
		return Candidate{}, false
	}
	return r.Candidates[0], true
}

// ResolveExternal finds external command candidates visible to the current process.
// It intentionally does not claim to model shell-local aliases, functions, builtins,
// cmdlets, or command caches.
func ResolveExternal(command string) (Result, error) {
	return resolveExternal(command, os.Getenv("PATH"), os.Getenv("PATHEXT"))
}

func resolveExternal(command, pathValue, pathExtValue string) (Result, error) {
	if command == "" {
		return Result{}, ErrEmptyCommand
	}
	if filepath.Base(command) != command {
		return Result{}, fmt.Errorf("%w: %s", ErrExplicitPathUnsupported, command)
	}

	candidates := findCandidates(command, filepath.SplitList(pathValue), pathExtValue)
	return Result{Command: command, Candidates: candidates}, nil
}

func absolutePath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		return abs
	}
	return filepath.Clean(path)
}
