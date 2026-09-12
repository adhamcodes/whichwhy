package resolution

import (
	"reflect"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

func TestEvidenceClassesAndSelection(t *testing.T) {
	process := ProcessExternal(resolver.Result{Command: "probe", Candidates: []resolver.Candidate{
		{Path: "/first/probe", DirectoryIndex: 2}, {Path: "/second/probe", DirectoryIndex: 5},
	}})
	shell := PowerShell(ps.Evidence{Command: "probe", Version: "5.1", Edition: "Desktop", Matches: []ps.Match{
		{CommandType: "Alias", Name: "probe", AliasTarget: "Get-Date"}, {CommandType: "Application", Name: "probe", Path: "/second/probe"},
	}})
	if process.Scope != "process-external" || process.Policy != "process-path-order-v1" || process.ClaimStrength != "policy-only" || process.Shell != nil {
		t.Fatalf("process claim = %#v", process)
	}
	if shell.Scope != "powershell-loaded-session" || shell.Policy != "powershell-loaded-session-order-v1" || shell.ClaimStrength != "shell-observed" || shell.Shell.Version != "5.1" {
		t.Fatalf("shell claim = %#v", shell)
	}
	if shell.ProcessPath != nil {
		t.Fatal("shell evidence acquired process PATH parsing")
	}
	if process.Selected.Path != "/first/probe" || process.Selected.PathIndex != 3 || process.Alternatives[0].PathIndex != 6 {
		t.Fatalf("process order/identity changed: %#v", process)
	}
	if shell.Selected.Type != "Alias" || shell.Selected.AliasTarget != "Get-Date" || shell.Alternatives[0].Path != "/second/probe" {
		t.Fatalf("shell order/identity changed: %#v", shell)
	}
	for _, r := range []Report{process, shell} {
		if !reflect.DeepEqual(*r.Selected, r.Candidates[0]) || !reflect.DeepEqual(r.Alternatives, r.Candidates[1:]) || r.SelectionReason == "" || r.NoCandidateReason != "" || len(r.Limitations) == 0 || r.ExitCode() != 0 {
			t.Fatalf("inconsistent completed report: %#v", r)
		}
	}
}

func TestNoCandidatesDoesNotUpgradeClaim(t *testing.T) {
	for _, r := range []Report{ProcessExternal(resolver.Result{Command: "missing"}), PowerShell(ps.Evidence{Command: "missing", Version: "7"})} {
		if r.Selected != nil || len(r.Candidates) != 0 || r.Candidates == nil || len(r.Alternatives) != 0 || r.SelectionReason != "" || r.NoCandidateReason == "" || len(r.Limitations) == 0 || r.ExitCode() != 1 {
			t.Fatalf("inconsistent no-candidate report: %#v", r)
		}
		if r.Scope == ProcessScope && r.ClaimStrength != "policy-only" || r.Scope == PowerShellScope && r.ClaimStrength != "shell-observed" {
			t.Fatalf("missing result changed evidence class: %#v", r)
		}
	}
}
