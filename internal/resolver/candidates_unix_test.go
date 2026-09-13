//go:build unix

package resolver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func TestUnixCandidateTargetModes(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if stat := info.Sys().(*syscall.Stat_t); stat.Uid != uint32(os.Geteuid()) {
		if os.Getenv("WHICHWHY_REQUIRE_ORACLES") == "1" {
			t.Fatalf("required fixture owner %d differs from effective UID %d", stat.Uid, os.Geteuid())
		}
		t.Skipf("fixture owner %d differs from effective UID %d", stat.Uid, os.Geteuid())
	}
	for _, mode := range []os.FileMode{0o100, 0o010, 0o001, 0o011, 0o111, 0} {
		t.Run(fmt.Sprintf("%04o", mode), func(t *testing.T) {
			if err := os.Chmod(target, mode); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(target)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("fixture mode not preserved: %v, %v", info, err)
			}
			want := mode&0o100 != 0
			if os.Geteuid() == 0 {
				want = mode&0o111 != 0
			}
			for _, command := range []string{"target", "link"} {
				r, err := resolveExternal(command, dir, "")
				if err != nil || (len(r.Candidates) == 1) != want {
					t.Fatalf("%s candidates=%#v want eligible=%v error=%v", command, r.Candidates, want, err)
				}
				if want && (r.Candidates[0].Path != filepath.Join(dir, command) || r.Candidates[0].DirectoryIndex != 0) {
					t.Fatalf("candidate spelling/index changed: %#v", r.Candidates)
				}
				status := ObservedIneligible
				if want {
					status = ObservedCandidate
				}
				if len(r.Observations) != 1 || r.Observations[0].Status != status || r.Observations[0].Incomplete() {
					t.Fatalf("eligibility observation lost: %#v", r)
				}
			}
		})
	}
	for _, command := range []string{"directory", "broken"} {
		path := filepath.Join(dir, command)
		if command == "directory" {
			err = os.Mkdir(path, 0o755)
		} else {
			err = os.Symlink(filepath.Join(dir, "missing"), path)
		}
		if err != nil {
			t.Fatal(err)
		}
		r, err := resolveExternal(command, dir, "")
		if err != nil || len(r.Candidates) != 0 {
			t.Fatalf("%s: %#v %v", command, r, err)
		}
		status := ObservedDirectory
		if command == "broken" {
			status = ObservedNotFound
		}
		if len(r.Observations) != 1 || r.Observations[0].Status != status || r.Observations[0].Incomplete() {
			t.Fatalf("target observation lost: %#v", r)
		}
	}
}

func TestUnixCollectedEffectiveIdentity(t *testing.T) {
	got, err := loadUnixIdentity(os.Geteuid(), os.Getegid(), os.Getgroups)
	groups, groupErr := os.Getgroups()
	if err != nil || groupErr != nil || got.uid != uint32(os.Geteuid()) || got.gid != uint32(os.Getegid()) || !reflect.DeepEqual(got.groups, groups) {
		t.Fatalf("effective identity=%#v groups=%v errors=%v/%v", got, groups, err, groupErr)
	}
}

func TestUnixCollectionFailureIsOperationalError(t *testing.T) {
	failure := errors.New("controlled identity failure")
	source := func() (unixIdentity, error) { return unixIdentity{}, failure }
	candidates, err := findUnixCandidates("probe", processpath.Parse(t.TempDir()).Entries, source, os.Stat)
	if !errors.Is(err, failure) || candidates.Candidates != nil || candidates.Observations != nil {
		t.Fatalf("identity failure became a completed search: %v %v", candidates, err)
	}
	candidates, err = findUnixCandidates("probe", nil, source, os.Stat)
	if err != nil || candidates.Candidates == nil || len(candidates.Candidates) != 0 || candidates.Observations == nil {
		t.Fatalf("empty PATH unnecessarily requires identity: %v %v", candidates, err)
	}
	info, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []any{nil, (*syscall.Stat_t)(nil), "not Unix ownership"} {
		eligible, err := unixFileEligible(missingOwnership{FileInfo: info, metadata: metadata}, unixIdentity{})
		if err == nil || eligible {
			t.Fatalf("unavailable ownership became a definitive eligibility result: %v %v", eligible, err)
		}
	}
}

type missingOwnership struct {
	os.FileInfo
	metadata any
}

func (info missingOwnership) Sys() any { return info.metadata }

func TestUnixObservationFailuresAndEligibility(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "probe")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	source := func() (unixIdentity, error) { return unixIdentity{uid: 1000, gid: 1000}, nil }
	for _, metadata := range []any{nil, (*syscall.Stat_t)(nil), "invalid"} {
		r, err := findUnixCandidates("probe", processpath.Parse(dir).Entries, source, func(string) (os.FileInfo, error) { return missingOwnership{FileInfo: info, metadata: metadata}, nil })
		if err == nil || r.Candidates != nil || r.Observations != nil {
			t.Fatalf("ownership failure downgraded: %#v %v", r, err)
		}
	}
	entries := processpath.Parse("failed:ineligible:eligible:failed").Entries
	eligibleInfo := missingOwnership{FileInfo: observedMode{FileInfo: info, mode: 0o100}, metadata: &syscall.Stat_t{Uid: 1000, Gid: 1000}}
	r, err := findUnixCandidates("probe", entries, source, func(path string) (os.FileInfo, error) {
		switch filepath.Dir(path) {
		case "failed":
			return nil, &os.PathError{Op: "stat", Path: path, Err: syscall.EACCES}
		case "ineligible":
			return missingOwnership{FileInfo: observedMode{FileInfo: info, mode: 0o011}, metadata: &syscall.Stat_t{Uid: 1000, Gid: 1000}}, nil
		default:
			return eligibleInfo, nil
		}
	})
	if err != nil || len(r.Candidates) != 1 || r.Candidates[0].DirectoryIndex != 2 || len(r.Observations) != 4 {
		t.Fatalf("%#v %v", r, err)
	}
	for i, status := range []ObservationStatus{ObservedError, ObservedIneligible, ObservedCandidate, ObservedError} {
		if r.Observations[i].Status != status || r.Observations[i].PathIndex != i+1 {
			t.Fatalf("%#v", r.Observations)
		}
	}
	// Real Stat follows the symlink; looping targets are failures, missing targets
	// are negatives. No lstat-based guess about link identity is introduced.
	if err := os.Symlink("loop", filepath.Join(dir, "loop")); err != nil {
		t.Fatal(err)
	}
	loop, err := resolveExternal("loop", dir, "")
	if err != nil || len(loop.Observations) != 1 || loop.Observations[0].Status != ObservedError || !loop.Observations[0].Incomplete() {
		t.Fatalf("loop failure lost: %#v %v", loop, err)
	}
	notDir, err := resolveExternal("probe", file, "")
	if err != nil || len(notDir.Observations) != 1 || notDir.Observations[0].Status != ObservedNotDirectory || notDir.Observations[0].Incomplete() {
		t.Fatalf("ENOTDIR lost: %#v %v", notDir, err)
	}
}

type observedMode struct {
	os.FileInfo
	mode os.FileMode
}

func (i observedMode) Mode() os.FileMode { return i.mode }
