//go:build unix

package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func findCandidates(command string, entries []processpath.Entry, _ string) ([]Candidate, error) {
	return findUnixCandidates(command, entries, func() (unixIdentity, error) {
		return loadUnixIdentity(os.Geteuid(), os.Getegid(), os.Getgroups)
	})
}

func findUnixCandidates(command string, entries []processpath.Entry, identitySource func() (unixIdentity, error)) ([]Candidate, error) {
	candidates := make([]Candidate, 0)
	if len(entries) == 0 {
		return candidates, nil
	}
	identity, err := identitySource()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		directory := entry.EffectiveValue()

		path := filepath.Join(directory, command)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		eligible, err := unixFileEligible(info, identity)
		if err != nil {
			return nil, fmt.Errorf("Unix candidate eligibility for %q: %w", path, err)
		}
		if !eligible {
			continue
		}

		candidates = append(candidates, Candidate{
			Path:           absolutePath(path),
			DirectoryIndex: entry.Index - 1,
		})
	}

	return candidates, nil
}

func unixFileEligible(info os.FileInfo, identity unixIdentity) (bool, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return false, fmt.Errorf("file owner UID/GID unavailable")
	}
	return unixModeEligible(info.Mode(), stat.Uid, stat.Gid, identity), nil
}
