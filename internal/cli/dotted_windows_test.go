//go:build windows

package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

// Only cmd's oracle executes these disposable fixtures. PowerShell discovery and
// both product presentations must leave the marker absent. The tiny native
// fixture makes .EXE and literal dotted files real executables, not fake binaries.
func TestWindowsDottedPATHEXTOracles(t *testing.T) {
	cmdExe := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	shells := make(map[string]string)
	for _, name := range []string{"powershell.exe", "pwsh.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			shells[name] = path
		}
	}
	buildDir := t.TempDir()
	source := filepath.Join(buildDir, "probe.go")
	const body = `package main
import ("fmt"; "os"; "path/filepath")
func main() {
 path, err := os.Executable(); if err != nil { panic(err) }
 name := filepath.Base(path)
 if err := os.WriteFile("oracle.marker", []byte(name), 0600); err != nil { panic(err) }
 fmt.Println(name)
}`
	if err := os.WriteFile(source, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(buildDir, "probe.exe")
	if output, err := exec.Command("go", "build", "-o", binaryPath, source).CombinedOutput(); err != nil {
		t.Fatalf("build controlled native fixture: %v: %s", err, output)
	}
	native, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, command, pathExt string
		files, policy, ps      []string
		cmd                    string // Empty means cmd must fail without a marker.
	}{
		{"extensionless", "wwprobe", ".EXE;.CMD", []string{"wwprobe.exe", "wwprobe.cmd"}, []string{"wwprobe.exe", "wwprobe.cmd"}, []string{"wwprobe.exe", "wwprobe.cmd"}, "wwprobe.exe"},
		{"dotted-cmd-only", "wwtool.v1", ".EXE;.CMD", []string{"wwtool.v1.cmd"}, []string{"wwtool.v1.cmd"}, []string{"wwtool.v1.cmd"}, "wwtool.v1.cmd"},
		{"dotted-order", "wwprobe.v1", ".EXE;.CMD", []string{"wwprobe.v1.cmd", "wwprobe.v1.exe"}, []string{"wwprobe.v1.exe", "wwprobe.v1.cmd"}, []string{"wwprobe.v1.exe", "wwprobe.v1.cmd"}, "wwprobe.v1.exe"},
		{"reversed-duplicates", "wwprobe.v1", ".CMD;.EXE;.cmd", []string{"wwprobe.v1.exe", "wwprobe.v1.cmd"}, []string{"wwprobe.v1.cmd", "wwprobe.v1.exe"}, []string{"wwprobe.v1.cmd", "wwprobe.v1.exe"}, "wwprobe.v1.cmd"},
		{"recognized", "wwprobe.cmd", ".EXE;.CMD", []string{"wwprobe.cmd", "wwprobe.cmd.exe", "wwprobe.cmd.cmd"}, []string{"wwprobe.cmd"}, []string{"wwprobe.cmd", "wwprobe.cmd.exe", "wwprobe.cmd.cmd"}, "wwprobe.cmd"},
		{"recognized-missing", "wwprobe.cmd", ".EXE;.CMD", []string{"wwprobe.cmd.exe", "wwprobe.cmd.cmd"}, nil, []string{"wwprobe.cmd.exe", "wwprobe.cmd.cmd"}, "wwprobe.cmd.exe"},
		{"recognized-case", "wwprobe.CmD", ".EXE;.CMD", []string{"wwprobe.cmd", "wwprobe.cmd.exe", "wwprobe.cmd.cmd"}, []string{"wwprobe.cmd"}, []string{"wwprobe.cmd", "wwprobe.cmd.exe", "wwprobe.cmd.cmd"}, "wwprobe.cmd"},
		{"multi-dot", "wwprobe.alpha.v1", ".EXE;.CMD", []string{"wwprobe.alpha.v1.cmd", "wwprobe.alpha.v1.exe"}, []string{"wwprobe.alpha.v1.exe", "wwprobe.alpha.v1.cmd"}, []string{"wwprobe.alpha.v1.exe", "wwprobe.alpha.v1.cmd"}, "wwprobe.alpha.v1.exe"},
		{"literal-dotted-first", "wwprobe.v1", ".EXE;.CMD", []string{"wwprobe.v1", "wwprobe.v1.exe", "wwprobe.v1.cmd"}, []string{"wwprobe.v1", "wwprobe.v1.exe", "wwprobe.v1.cmd"}, []string{"wwprobe.v1", "wwprobe.v1.exe", "wwprobe.v1.cmd"}, "wwprobe.v1"},
		{"literal-dotted-only", "wwprobe.v1", ".EXE;.CMD", []string{"wwprobe.v1"}, []string{"wwprobe.v1"}, []string{"wwprobe.v1"}, "wwprobe.v1"},
		{"literal-extensionless", "wwprobe", ".EXE;.CMD", []string{"wwprobe", "wwprobe.exe", "wwprobe.cmd"}, []string{"wwprobe.exe", "wwprobe.cmd"}, []string{"wwprobe.exe", "wwprobe.cmd", "wwprobe"}, "wwprobe.exe"},
		{"literal-extensionless-only", "wwprobe", ".EXE;.CMD", []string{"wwprobe"}, nil, []string{"wwprobe"}, ""},
		{"trailing-dot", "wwprobe.", ".CMD", []string{"wwprobe.cmd", "wwprobe..cmd"}, []string{"wwprobe..cmd"}, []string{"wwprobe..cmd"}, "wwprobe..cmd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "path")
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, name := range tc.files {
				content := native
				if strings.HasSuffix(name, ".cmd") {
					content = []byte("@echo off\r\necho " + name + "\r\necho " + name + ">oracle.marker\r\n")
				}
				if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(root)
			t.Setenv("PATH", dir)
			t.Setenv("PATHEXT", tc.pathExt)
			var human, machine, stderr bytes.Buffer
			wantCode := 0
			if len(tc.policy) == 0 {
				wantCode = 1
			}
			if code := Run([]string{tc.command}, &human, &stderr, "test"); code != wantCode {
				t.Fatalf("human exit %d, want %d: %s", code, wantCode, &stderr)
			}
			if code := Run([]string{tc.command, "--json"}, &machine, &stderr, "test"); code != wantCode {
				t.Fatalf("JSON exit %d, want %d: %s", code, wantCode, &stderr)
			}
			var doc jsonDocument
			if err := json.Unmarshal(machine.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Policy != "process-path-order-v1" || doc.ResolutionScope != "process-external" || doc.ClaimStrength != "policy-only" || len(doc.Limitations) == 0 || len(doc.Candidates) != len(tc.policy) {
				t.Fatalf("unexpected policy result: %s", &machine)
			}
			for i, c := range doc.Candidates {
				if !strings.EqualFold(c.Path, filepath.Join(dir, tc.policy[i])) || c.PathIndex != 1 || !strings.Contains(human.String(), c.Path) {
					t.Fatalf("candidate %d differs from policy: %s / %s", i, &machine, &human)
				}
			}
			if len(tc.policy) > 0 {
				if doc.Selected == nil || !strings.EqualFold(doc.Selected.Path, filepath.Join(dir, tc.policy[0])) {
					t.Fatalf("selection differs from policy: %s", &machine)
				}
			} else if doc.Selected != nil || doc.NoCandidateReason == "" {
				t.Fatalf("missing candidate overclaims: %s", &machine)
			}
			assertDottedMarkerAbsent(t)
			for _, name := range []string{"powershell.exe", "pwsh.exe"} {
				t.Run(name, func(t *testing.T) {
					path, ok := shells[name]
					if !ok {
						t.Skipf("%s unavailable", name)
					}
					// A fresh child session; only core discovery, no auto-import or
					// invocation. Fixture command names are fixed safe test literals.
					script := "$PSModuleAutoLoadingPreference='None'; [Console]::WriteLine($PSVersionTable.PSVersion.ToString()); foreach ($c in @(Microsoft.PowerShell.Core\\Get-Command -Name '" + tc.command + "' -All -ListImported -ErrorAction SilentlyContinue)) { [Console]::WriteLine($c.Path) }"
					units := utf16.Encode([]rune(script))
					encoded := make([]byte, len(units)*2)
					for i, unit := range units {
						binary.LittleEndian.PutUint16(encoded[i*2:], unit)
					}
					output, err := exec.Command(path, "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(encoded)).CombinedOutput()
					if err != nil {
						t.Fatalf("PowerShell discovery: %v: %s", err, output)
					}
					lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(string(output)), "\r", ""), "\n")
					if len(lines) != len(tc.ps)+1 {
						t.Fatalf("PowerShell list = %q, want version + %q", lines, tc.ps)
					}
					for i, want := range tc.ps {
						if !strings.EqualFold(lines[i+1], filepath.Join(dir, want)) {
							t.Fatalf("PowerShell list = %q, want %q", lines[1:], tc.ps)
						}
					}
					assertDottedMarkerAbsent(t)
					t.Logf("PowerShell %s: %v; process policy: %v", lines[0], tc.ps, tc.policy)
				})
			}
			oracle := exec.Command(cmdExe, "/D", "/Q", "/C", tc.command)
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(strings.ToLower(entry), "nodefaultcurrentdirectoryinexepath=") {
					oracle.Env = append(oracle.Env, entry)
				}
			}
			output, err := oracle.CombinedOutput()
			if tc.cmd == "" {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
					t.Fatalf("cmd must exit 1 for missing fixture: %v: %s", err, output)
				}
				assertDottedMarkerAbsent(t)
			} else {
				if err != nil || !strings.EqualFold(strings.TrimSpace(string(output)), tc.cmd) {
					t.Fatalf("cmd oracle = %q, %v; want %s", output, err, tc.cmd)
				}
				marker, err := os.ReadFile("oracle.marker")
				if err != nil || !strings.EqualFold(strings.TrimSpace(string(marker)), tc.cmd) {
					t.Fatalf("cmd execution marker = %q, %v", marker, err)
				}
			}
			t.Logf("cmd selected %q; process policy: %v", tc.cmd, tc.policy)
		})
	}
}

func assertDottedMarkerAbsent(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("oracle.marker"); !os.IsNotExist(err) {
		t.Fatalf("inspection executed fixture or marker check failed: %v", err)
	}
}
