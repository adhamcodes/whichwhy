//go:build windows

package resolver

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

var defaultPathExt = []string{".COM", ".EXE", ".BAT", ".CMD"}

func findCandidates(command string, entries []processpath.Entry, pathExtValue string) (candidateEvidence, error) {
	return findWindowsCandidates(command, entries, pathExtValue, os.Stat)
}

func findWindowsCandidates(command string, entries []processpath.Entry, pathExtValue string, stat statFunc) (candidateEvidence, error) {
	return collectCandidates(entries, windowsCandidateNames(command, pathExtValue), stat,
		func(os.FileInfo) (bool, error) { return true, nil }, filepath.Abs)
}

func windowsCandidateNames(command, pathExtValue string) []string {
	extensions := parsePathExt(pathExtValue)
	if len(extensions) == 0 {
		extensions = defaultPathExt
	}

	suffix := filepath.Ext(command)
	for _, extension := range extensions {
		if strings.EqualFold(suffix, extension) {
			// Explicit executable suffixes stay literal under this process policy.
			return []string{command}
		}
	}

	names := make([]string, 0, len(extensions)+1)
	if suffix != "" {
		// Preserve literal dotted candidates, but a dot alone must not suppress
		// PATHEXT expansion. Extensionless literals remain outside this policy.
		names = append(names, command)
	}
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
