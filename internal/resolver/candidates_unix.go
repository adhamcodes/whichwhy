//go:build unix

package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func findCandidates(command string, entries []processpath.Entry, _ string) (candidateEvidence, error) {
	return findUnixCandidates(command, entries, func() (unixIdentity, error) {
		return loadUnixIdentity(os.Geteuid(), os.Getegid(), os.Getgroups)
	}, os.Stat)
}

func findUnixCandidates(command string, entries []processpath.Entry, identitySource func() (unixIdentity, error), stat statFunc) (candidateEvidence, error) {
	if len(entries) == 0 {
		return candidateEvidence{Candidates: []Candidate{}, Observations: []Observation{}}, nil
	}
	identity, err := identitySource()
	if err != nil {
		return candidateEvidence{}, err
	}
	return collectCandidates(entries, []string{command}, stat,
		func(info os.FileInfo) (bool, error) { return unixFileEligible(info, identity) }, filepath.Abs)
}

func unixFileEligible(info os.FileInfo, identity unixIdentity) (bool, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return false, fmt.Errorf("file owner UID/GID unavailable")
	}
	return unixModeEligible(info.Mode(), stat.Uid, stat.Gid, identity), nil
}
