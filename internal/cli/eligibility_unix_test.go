//go:build unix

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestUnixEligibilityPATHCorrelation(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, dir := range []string{"denied", "first", "last", "linked"} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if dir != "linked" {
			writePATHFixture(t, dir)
		}
	}
	if err := os.Chmod(filepath.Join("denied", "wwpathfixture"), 0o411); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "first", "wwpathfixture"), filepath.Join("linked", "wwpathfixture")); err != nil {
		t.Fatal(err)
	}
	value := "missing:denied:first:first:linked:last"
	indices := []int{3, 5, 6}
	if os.Geteuid() == 0 {
		indices = []int{2, 3, 5, 6}
	}
	assertPATHCorrelation(t, value, indices)
	raw, err := resolver.ResolveExternal("wwpathfixture")
	if err != nil {
		t.Fatal(err)
	}
	report := resolution.ProcessExternal(raw)
	if report.Scope != "process-external" || report.Policy != "process-path-order-v1" || report.ClaimStrength != "policy-only" {
		t.Fatalf("claim changed: %#v", report)
	}
	limitations := strings.Join(report.Limitations, " ")
	for _, text := range []string{"effective UID", "effective GID", "supplementary groups", "owner/group/other", "UID 0", "Symlinks", "ACLs", "noexec", "not atomic", "interpreter/shebang", "capabilities", "filesystem IDs", "macOS extended group"} {
		if !strings.Contains(limitations, text) {
			t.Fatalf("Unix limitation omitted %q: %s", text, limitations)
		}
	}
}

// Only this test invokes the disposable script it constructs. Inspection must
// leave its marker absent; direct child execution provides the kernel oracle.
func TestUnixOwnerEligibilityOracle(t *testing.T) {
	if os.Geteuid() == 0 {
		unavailableOracle(t, "owner-class denial requires an unprivileged effective UID; root mode policy has separate unit/target coverage")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	name := "wweligibilityfixture"
	path := filepath.Join(dir, name)
	body := "#!/bin/sh\nprintf executed > inspected.marker\nprintf fixture-ok\n"
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		unavailableOracle(t, "filesystem did not assign fixture ownership to the effective test user")
	}
	// Verify that this location and known interpreter permit script execution
	// before interpreting an EACCES result as evidence of mode-class denial.
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	// Even the execution control follows inspection, so an inspection regression
	// cannot be hidden by first creating and then removing the control marker.
	for _, args := range [][]string{{name}, {name, "--json"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, "test"); code != 0 || stderr.Len() != 0 {
			t.Fatalf("control inspection exit=%d: %s %s", code, &stdout, &stderr)
		}
		if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
			t.Fatalf("control inspection executed fixture or marker check failed: %v", err)
		}
	}
	output, err := exec.Command(path).CombinedOutput()
	if err != nil {
		if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) || errors.Is(err, os.ErrNotExist) {
			unavailableOracle(t, "0700 control cannot execute here (mount/platform restriction or unavailable /bin/sh): %v", err)
		}
		t.Fatalf("0700 control failed: %v %s", err, output)
	}
	if string(output) != "fixture-ok" {
		t.Fatalf("unexpected control output: %q", output)
	}
	if marker, err := os.ReadFile("inspected.marker"); err != nil || string(marker) != "executed" {
		t.Fatalf("control execution marker: %q %v", marker, err)
	}
	if err := os.Remove("inspected.marker"); err != nil {
		t.Fatal(err)
	}
	for _, bits := range []os.FileMode{0o100, 0o010, 0o001, 0o011, 0o111, 0} {
		t.Run(fmt.Sprintf("execute-bits-%04o", bits), func(t *testing.T) {
			// Owner read is necessary for /bin/sh to read the script after exec.
			// It does not change execute-class selection. Exact execute-only
			// modes are also tested by TestUnixCandidateTargetModes.
			mode := 0o400 | bits
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != mode {
				t.Fatalf("fixture mode not preserved: %v %v", info, err)
			}
			want := bits&0o100 != 0
			if !want && bits != 0 && info.Mode().Perm()&0o111 == 0 {
				t.Fatal("regression fixture must pass the pre-F5 any-execute-bit filter")
			}
			for _, args := range [][]string{{name}, {name, "--json"}} {
				var stdout, stderr bytes.Buffer
				wantCode := 1
				if want {
					wantCode = 0
				}
				if code := Run(args, &stdout, &stderr, "test"); code != wantCode || stderr.Len() != 0 {
					t.Fatalf("inspection exit=%d want=%d: %s %s", code, wantCode, &stdout, &stderr)
				}
				if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
					t.Fatalf("inspection executed fixture: %v", err)
				}
			}
			output, err := exec.Command(path).CombinedOutput()
			if want {
				if err != nil || string(output) != "fixture-ok" {
					t.Fatalf("controlled execution failed: %v %q", err, output)
				}
				marker, err := os.ReadFile("inspected.marker")
				if err != nil || string(marker) != "executed" {
					t.Fatalf("oracle marker: %q %v", marker, err)
				}
				if err := os.Remove("inspected.marker"); err != nil {
					t.Fatal(err)
				}
			} else {
				if !errors.Is(err, syscall.EACCES) {
					t.Fatalf("controlled exec should fail with EACCES before starting: %v %q", err, output)
				}
				if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
					t.Fatalf("denied oracle wrote a marker: %v", err)
				}
			}
			t.Logf("mode=%04o legacy-eligible=%v user-policy-eligible=%v controlled-exec-success=%v", mode, mode&0o111 != 0, want, err == nil)
		})
	}
}
