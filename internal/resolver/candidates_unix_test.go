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
	candidates, err := findUnixCandidates("probe", processpath.Parse(t.TempDir()).Entries, source)
	if !errors.Is(err, failure) || candidates != nil {
		t.Fatalf("identity failure became a completed search: %v %v", candidates, err)
	}
	candidates, err = findUnixCandidates("probe", nil, source)
	if err != nil || candidates == nil || len(candidates) != 0 {
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
