package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestRunCommandJSONPreservesExternalResolutionEvidence(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{
			Command: command,
			Candidates: []resolver.Candidate{
				{Path: `/first/python`, DirectoryIndex: 2},
				{Path: `/second/python`, DirectoryIndex: 5},
			},
		}, nil
	}

	code := run([]string{"python", "--json"}, &stdout, &stderr, "dev", resolve)
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var doc jsonDocument
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if doc.SchemaVersion != 1 || doc.Command != "python" || doc.ResolutionScope != "process-external" {
		t.Fatalf("unexpected JSON document: %#v", doc)
	}
	if doc.Winner == nil || doc.Winner.Path != `/first/python` || doc.Winner.PathIndex != 3 {
		t.Fatalf("winner = %#v", doc.Winner)
	}
	if got, want := len(doc.Candidates), 2; got != want {
		t.Fatalf("candidate count = %d, want %d", got, want)
	}
	if doc.Candidates[1].Path != `/second/python` || doc.Candidates[1].PathIndex != 6 {
		t.Fatalf("second candidate = %#v", doc.Candidates[1])
	}
	if doc.WinnerReason == "" || len(doc.Limitations) == 0 {
		t.Fatalf("JSON document lacks explanation metadata: %#v", doc)
	}
}

func TestRunCommandJSONKeepsMissingResultMachineReadable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{Command: command}, nil
	}

	code := run([]string{"missing", "--json"}, &stdout, &stderr, "dev", resolve)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	var doc jsonDocument
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if doc.Winner != nil {
		t.Fatalf("winner = %#v, want nil", doc.Winner)
	}
	if doc.Candidates == nil || len(doc.Candidates) != 0 {
		t.Fatalf("candidates = %#v, want non-nil empty array", doc.Candidates)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunPowerShellJSONPreservesShellWinnerAndOrder(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	alias := base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{
		"Alias", "wwprobe", "", "", "Get-Date",
	}, "\x1f")))
	application := base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{
		"Application", "wwprobe.cmd", "", `C:\Tools\wwprobe.cmd`, "",
	}, "\x1f")))

	code := Run([]string{"__powershell-json", "wwprobe", "5.1", "Desktop", alias, application}, &stdout, &stderr, "dev")
	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0", code)
	}
	var doc jsonDocument
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if doc.ResolutionScope != "powershell-loaded-session" || doc.Shell == nil || doc.Shell.Version != "5.1" {
		t.Fatalf("unexpected PowerShell JSON metadata: %#v", doc)
	}
	if doc.Winner == nil || doc.Winner.Kind != "alias" || doc.Winner.AliasTarget != "Get-Date" {
		t.Fatalf("winner = %#v", doc.Winner)
	}
	if got, want := len(doc.Candidates), 2; got != want || doc.Candidates[1].Kind != "application" {
		t.Fatalf("candidates = %#v", doc.Candidates)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRejectsUnknownSecondCommandArgument(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	resolve := func(command string) (resolver.Result, error) {
		t.Fatal("resolver should not run for invalid arguments")
		return resolver.Result{}, nil
	}

	code := run([]string{"python", "--bogus"}, &stdout, &stderr, "dev", resolve)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "optional --json") {
		t.Fatalf("stdout = %q stderr = %q", stdout.String(), stderr.String())
	}
}
