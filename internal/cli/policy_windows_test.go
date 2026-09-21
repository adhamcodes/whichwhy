//go:build windows

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This oracle executes only the disposable .cmd fixtures written below. The
// product's inspection runs first and must leave the execution marker absent.
func TestWindowsPolicyVersusCmdOracle(t *testing.T) {
	cmdExe := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	if _, err := os.Stat(cmdExe); err != nil {
		t.Fatalf("cmd oracle unavailable: %v", err)
	}
	for _, name := range []string{"normal-path-order", "duplicate-path", "implicit-cwd-difference", "leading-empty-entry", "empty-path"} {
		t.Run(name, func(t *testing.T) {
			cwd, pathDir, lastDir := t.TempDir(), t.TempDir(), t.TempDir()
			t.Chdir(cwd)
			const probe = "wwpolicyoraclefixture"
			for dir, label := range map[string]string{cwd: "cwd", pathDir: "path", lastDir: "last"} {
				if dir == cwd && (name == "normal-path-order" || name == "duplicate-path") {
					continue // No implicit cwd competitor in the PATH-order experiment.
				}
				body := "@echo off\r\necho " + label + "\r\necho " + label + ">oracle.marker\r\n"
				if err := os.WriteFile(filepath.Join(dir, probe+".cmd"), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			pathValue, selectedDir := pathDir, pathDir
			wantCount, wantCode := 1, 0
			oracleLabel, classification := "cwd", "EXPECTED AGREEMENT"
			secondDir, secondIndex := pathDir, 2
			switch name {
			case "normal-path-order", "duplicate-path":
				pathValue = pathDir + ";" + lastDir
				wantCount, oracleLabel, secondDir = 2, "path", lastDir
				if name == "duplicate-path" {
					pathValue = pathDir + ";" + pathDir + ";" + lastDir
					secondIndex = 3
				}
			case "implicit-cwd-difference":
				classification = "EXPECTED DELIBERATE DIFFERENCE"
			case "leading-empty-entry":
				pathValue = ";" + pathDir
				selectedDir = cwd
				wantCount = 2
			case "empty-path":
				pathValue = ""
				wantCount, wantCode = 0, 1
				classification = "EXPECTED DELIBERATE DIFFERENCE"
			}
			t.Setenv("PATH", pathValue)
			t.Setenv("PATHEXT", ".CMD")
			var human, machine, stderr bytes.Buffer
			if code := Run([]string{probe}, &human, &stderr, "test"); code != wantCode {
				t.Fatalf("human exit %d: %s", code, &stderr)
			}
			if code := Run([]string{probe, "--json"}, &machine, &stderr, "test"); code != wantCode {
				t.Fatalf("JSON exit %d: %s", code, &stderr)
			}
			var doc jsonDocument
			if err := json.Unmarshal(machine.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			want := filepath.Join(selectedDir, probe+".CMD")
			if doc.Policy != "process-path-order-v1" || doc.ResolutionScope != "process-external" || doc.ClaimStrength != "policy-only" || len(doc.Limitations) == 0 || len(doc.Candidates) != wantCount {
				t.Fatalf("unexpected policy result: %s", &machine)
			}
			if wantCount == 0 {
				if doc.Selected != nil || doc.NoCandidateReason == "" || !strings.Contains(human.String(), humanReason(doc.NoCandidateReason)) {
					t.Fatalf("missing policy result overclaims: %s / %s", &human, &machine)
				}
			} else {
				if doc.Selected == nil || !strings.EqualFold(doc.Selected.Path, want) || doc.Selected.PathIndex != 1 || !strings.Contains(human.String(), doc.Selected.Path) {
					t.Fatalf("unexpected selection: %s / %s", &human, &machine)
				}
				if wantCount == 2 && (!strings.EqualFold(doc.Candidates[1].Path, filepath.Join(secondDir, probe+".CMD")) || doc.Candidates[1].PathIndex != secondIndex) {
					t.Fatalf("candidate order changed: %s", &machine)
				}
			}
			if !strings.Contains(human.String(), "may choose differently") {
				t.Fatalf("human/JSON disagreement: %s", &human)
			}
			if _, err := os.Stat("oracle.marker"); !os.IsNotExist(err) {
				t.Fatalf("inspection executed fixture or marker check failed: %v", err)
			}
			oracle := exec.Command(cmdExe, "/D", "/Q", "/C", probe)
			// Eliminate inherited opt-out of cmd's default current-directory search.
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(strings.ToLower(entry), "nodefaultcurrentdirectoryinexepath=") {
					oracle.Env = append(oracle.Env, entry)
				}
			}
			output, err := oracle.CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != oracleLabel {
				t.Fatalf("cmd oracle: %v %q", err, output)
			}
			if marker, err := os.ReadFile("oracle.marker"); err != nil || strings.TrimSpace(string(marker)) != oracleLabel {
				t.Fatalf("oracle fixture did not run as expected: %q %v", marker, err)
			}
			t.Logf("%s: policy candidates=%d; cmd executed %s fixture; case=%s", classification, len(doc.Candidates), oracleLabel, name)
		})
	}
}
