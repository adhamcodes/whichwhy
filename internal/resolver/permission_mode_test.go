package resolver

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestUnixModeEligibility(t *testing.T) {
	// Each identity independently fixes the applicable class. Enumerating all
	// 512 modes catches fallback, unrelated read/write bits, and odd combinations.
	for _, tc := range []struct {
		name     string
		identity unixIdentity
		mask     os.FileMode
	}{
		{"owner", unixIdentity{uid: 42, gid: 7}, 0o100},
		{"owner-also-in-group", unixIdentity{uid: 42, gid: 84, groups: []int{84}}, 0o100},
		{"effective-group", unixIdentity{uid: 43, gid: 84}, 0o010},
		{"supplementary-group", unixIdentity{uid: 43, gid: 7, groups: []int{9, 84, 10}}, 0o010},
		{"duplicate-groups", unixIdentity{uid: 43, gid: 7, groups: []int{84, 84, 7}}, 0o010},
		{"other-empty-groups", unixIdentity{uid: 43, gid: 7}, 0o001},
		{"other-unmatched-groups", unixIdentity{uid: 43, gid: 7, groups: []int{9, 10}}, 0o001},
		{"root", unixIdentity{uid: 0, gid: 7}, 0o111},
		{"root-matching-group", unixIdentity{uid: 0, gid: 84}, 0o111},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for mode := os.FileMode(0); mode <= 0o777; mode++ {
				want := mode&tc.mask != 0
				for _, extra := range []os.FileMode{0, os.ModeSetuid | os.ModeSetgid | os.ModeSticky} {
					if got := unixModeEligible(mode|extra, 42, 84, tc.identity); got != want {
						t.Fatalf("mode %04o extra %v: got %v want %v", mode, extra, got, want)
					}
				}
			}
		})
	}
	for _, mode := range []os.FileMode{0, 0o100, 0o010, 0o001, 0o111} {
		if got := unixModeEligible(mode, 0, 0, unixIdentity{}); got != (mode != 0) {
			t.Fatalf("root-owned mode %04o: %v", mode, got)
		}
	}
	if !unixModeEligible(0o010, 42, ^uint32(0), unixIdentity{uid: 43, gid: ^uint32(0)}) {
		t.Fatal("full-width numeric GID lost")
	}
}

func TestUnixModeRejectsLegacyAnyExecuteFalsePositive(t *testing.T) {
	for _, mode := range []os.FileMode{0o010, 0o001, 0o011} {
		t.Run(fmt.Sprintf("owner-mode-%04o", mode), func(t *testing.T) {
			if mode&0o111 == 0 {
				t.Fatal("fixture must pass the pre-F5 filter")
			}
			if unixModeEligible(mode, 42, 84, unixIdentity{uid: 42, gid: 84, groups: []int{84}}) {
				t.Fatal("owner denial fell through to another class")
			}
		})
	}
	if unixModeEligible(0o001, 42, 84, unixIdentity{uid: 43, gid: 84}) {
		t.Fatal("group denial fell through to other")
	}
}

func TestLoadUnixIdentity(t *testing.T) {
	groups := []int{91, 84, 84}
	got, err := loadUnixIdentity(42, 7, func() ([]int, error) { return groups, nil })
	if err != nil || !reflect.DeepEqual(got, unixIdentity{uid: 42, gid: 7, groups: groups}) {
		t.Fatalf("identity = %#v, %v", got, err)
	}
	if !unixModeEligible(0o010, 43, 84, got) {
		t.Fatal("collected supplementary membership not used")
	}
	failure := errors.New("controlled group collection failure")
	_, err = loadUnixIdentity(42, 7, func() ([]int, error) { return nil, failure })
	if !errors.Is(err, failure) {
		t.Fatalf("collection failure discarded: %v", err)
	}
}
