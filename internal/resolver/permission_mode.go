package resolver

import (
	"fmt"
	"os"
)

// unixIdentity is the effective process identity, not an account-database lookup.
// Keep collection and mode evaluation portable so synthetic identities can be
// tested even on a Windows development host.
type unixIdentity struct {
	uid, gid uint32
	groups   []int
}

func loadUnixIdentity(euid, egid int, getgroups func() ([]int, error)) (unixIdentity, error) {
	groups, err := getgroups()
	if err != nil {
		return unixIdentity{}, fmt.Errorf("collect Unix supplementary groups: %w", err)
	}
	return unixIdentity{uid: uint32(euid), gid: uint32(egid), groups: groups}, nil
}

// unixModeEligible applies only the documented mode-class policy. It is not an
// access/exec probe: ACLs, mount policy, and special credentials are not modeled.
// Callers exclude directories and follow symlinks before supplying target facts.
func unixModeEligible(mode os.FileMode, owner, group uint32, identity unixIdentity) bool {
	if identity.uid == 0 {
		// Traditional superuser mode policy on Linux and macOS. Actual kernel
		// privileges (including Linux capabilities) can differ from this model.
		return mode.Perm()&0o111 != 0
	}
	if identity.uid == owner {
		return mode.Perm()&0o100 != 0
	}
	if identity.gid == group {
		return mode.Perm()&0o010 != 0
	}
	for _, gid := range identity.groups {
		if uint32(gid) == group {
			return mode.Perm()&0o010 != 0
		}
	}
	return mode.Perm()&0o001 != 0
}
