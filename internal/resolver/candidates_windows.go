//go:build windows

package resolver

import (
	"os"
	"path/filepath"
	"strings"
)

var defaultPathExt = []string{".COM", ".EXE", ".BAT", ".CMD"}

func findCandidates(command string, directories []string, pathExtValue string) []Candidate {
	candidates := make([]Candidate, 0)
	names := windowsCandidateNames(command, pathExtValue)

	for index, directory := range directories {
		if directory == "" {
			directory = "."
		}

		for _, name := range names {
			path := filepath.Join(directory, name)
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}

			candidates = append(candidates, Candidate{
				Path:           absolutePath(path),
				DirectoryIndex: index,
			})
		}
	}

	return candidates
}

func windowsCandidateNames(command, pathExtValue string) []string {
	if filepath.Ext(command) != "" {
		return []string{command}
	}

	extensions := parsePathExt(pathExtValue)
	if len(extensions) == 0 {
		extensions = defaultPathExt
	}

	names := make([]string, 0, len(extensions))
	seen := make(map[string]struct{}, len(extensions))
	for _, extension := range extensions {
		name := command + extension
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	return names
}

func parsePathExt(value string) []string {
	parts := strings.Split(value, ";")
	extensions := make([]string, 0, len(parts))
	for _, part := range parts {
		extension := strings.TrimSpace(part)
		if extension == "" {
			continue
		}
		if !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		extensions = append(extensions, extension)
	}
	return extensions
}
