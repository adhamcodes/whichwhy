//go:build !windows

package resolver

import (
	"os"
	"path/filepath"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func findCandidates(command string, entries []processpath.Entry, _ string) []Candidate {
	candidates := make([]Candidate, 0)

	for _, entry := range entries {
		directory := entry.EffectiveValue()

		path := filepath.Join(directory, command)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			continue
		}

		candidates = append(candidates, Candidate{
			Path:           absolutePath(path),
			DirectoryIndex: entry.Index - 1,
		})
	}

	return candidates
}
