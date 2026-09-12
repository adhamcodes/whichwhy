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
	for _, name := range []string{"path-only", "leading-empty-entry", "empty-path"} {
		t.Run(name, func(t *testing.T) {
			cwd, pathDir := t.TempDir(), t.TempDir()
			t.Chdir(cwd)
			const probe = "wwpolicyoraclefixture"
			for dir, label := range map[string]string{cwd: "cwd", pathDir: "path"} {
				body := "@echo off\r\necho " + label + "\r\necho executed>oracle.marker\r\n"
				if err := os.WriteFile(filepath.Join(dir, probe+".cmd"), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			pathValue, selectedDir := pathDir, pathDir
			wantCount, wantCode := 1, 0
			switch name {
			case "leading-empty-entry":
				pathValue = ";" + pathDir
				selectedDir = cwd
				wantCount = 2
			case "empty-path":
				pathValue = ""
				wantCount, wantCode = 0, 1
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
				if doc.Selected != nil || doc.NoCandidateReason == "" || !strings.Contains(human.String(), doc.NoCandidateReason) {
					t.Fatalf("missing policy result overclaims: %s / %s", &human, &machine)
				}
			} else {
				if doc.Selected == nil || !strings.EqualFold(doc.Selected.Path, want) || doc.Selected.PathIndex != 1 || !strings.Contains(human.String(), doc.Selected.Path) {
					t.Fatalf("unexpected selection: %s / %s", &human, &machine)
				}
				if wantCount == 2 && (!strings.EqualFold(doc.Candidates[1].Path, filepath.Join(pathDir, probe+".CMD")) || doc.Candidates[1].PathIndex != 2) {
					t.Fatalf("candidate order changed: %s", &machine)
				}
			}
			if !strings.Contains(human.String(), "shell was not observed") {
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
			if err != nil || strings.TrimSpace(string(output)) != "cwd" {
				t.Fatalf("cmd oracle: %v %q", err, output)
			}
			if _, err := os.Stat("oracle.marker"); err != nil {
				t.Fatalf("oracle fixture did not run: %v", err)
			}
			t.Logf("policy candidates=%d; cmd executed cwd fixture; case=%s", len(doc.Candidates), name)
		})
	}
}
