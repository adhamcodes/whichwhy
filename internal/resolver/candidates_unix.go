//go:build !windows

package resolver

import (
	"os"
	"path/filepath"
)

func findCandidates(command string, directories []string, _ string) []Candidate {
	candidates := make([]Candidate, 0)

	for index, directory := range directories {
		if directory == "" {
			directory = "."
		}

		path := filepath.Join(directory, command)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			continue
		}

		candidates = append(candidates, Candidate{
			Path:           absolutePath(path),
			DirectoryIndex: index,
		})
	}

	return candidates
}
