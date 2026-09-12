package resolver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

var (
	ErrEmptyCommand            = errors.New("command name is empty")
	ErrExplicitPathUnsupported = errors.New("explicit command paths are not supported yet")
)

// Candidate is an external command found through the process-visible search path.
type Candidate struct {
	Path           string
	DirectoryIndex int // Zero-based offset into Result.Path.Entries; never compacted.
}

// Result contains ordered process-visible external evidence, not a shell winner.
type Result struct {
	Command    string
	Path       processpath.Path
	Candidates []Candidate
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

	path := processpath.Parse(pathValue)
	candidates := distinctCandidatePaths(findCandidates(command, path.Entries, pathExtValue))
	return Result{Command: command, Path: path, Candidates: candidates}, nil
}

// distinctCandidatePaths removes repeated references to the same visible path while
// preserving the first search-order occurrence. It deliberately does not resolve
// symlinks: two different visible paths can still be meaningful resolution evidence.
func distinctCandidatePaths(candidates []Candidate) []Candidate {
	distinct := make([]Candidate, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		key := candidatePathKey(candidate.Path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		distinct = append(distinct, candidate)
	}
	return distinct
}

func candidatePathKey(path string) string {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(key)
	}
	return key
}

func absolutePath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		return abs
	}
	return filepath.Clean(path)
}
