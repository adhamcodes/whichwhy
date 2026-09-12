package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

func TestPresentationsConsumeCompletedReport(t *testing.T) {
	for _, report := range []resolution.Report{
		resolution.ProcessExternal(resolver.Result{Command: "probe", Candidates: []resolver.Candidate{{Path: "/first/probe"}, {Path: "/second/probe"}}}),
		resolution.ProcessExternal(resolver.Result{Command: "missing"}),
		resolution.PowerShell(ps.Evidence{Command: "probe", Version: "7", Matches: []ps.Match{{CommandType: "Alias", Name: "probe", AliasTarget: "Get-Date"}}}),
		resolution.PowerShell(ps.Evidence{Command: "missing", Version: "7"}),
	} {
		t.Run(report.Scope+"/"+report.Command, func(t *testing.T) {
			assertReportPresentations(t, report)
		})
	}
}

// A completed report can carry a selection other than the first enumerated
// candidate. Renderers must honor it, not silently reapply today's policy.
func TestPresentationDoesNotRecomputeSelection(t *testing.T) {
	r := resolution.ProcessExternal(resolver.Result{Command: "probe", Candidates: []resolver.Candidate{{Path: "/first/probe"}, {Path: "/second/probe"}}})
	selected := r.Candidates[1]
	r.Selected = &selected
	r.Alternatives = r.Candidates[:1]
	r.SelectionReason = "Controlled completed report supplied by the test."
	assertReportPresentations(t, r)
	var human bytes.Buffer
	printCommandReport(&human, r)
	if strings.Index(human.String(), "/second/probe") > strings.Index(human.String(), "/first/probe") {
		t.Fatal("human renderer independently selected first candidate")
	}
}

func assertReportPresentations(t *testing.T, r resolution.Report) {
	t.Helper()
	var human, machine, stderr bytes.Buffer
	humanCode := printCommandReport(&human, r)
	jsonCode := printCommandJSON(&machine, &stderr, r)
	if humanCode != r.ExitCode() || jsonCode != humanCode || stderr.Len() != 0 {
		t.Fatalf("exit mismatch or error: %d %d %s", humanCode, jsonCode, &stderr)
	}
	var doc jsonDocument
	if err := json.Unmarshal(machine.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Policy != r.Policy || doc.ResolutionScope != r.Scope || doc.ClaimStrength != r.ClaimStrength || !reflect.DeepEqual(doc.Limitations, r.Limitations) || doc.SelectionReason != r.SelectionReason {
		t.Fatalf("JSON changed report claim: %#v", doc)
	}
	if r.Selected == nil {
		if doc.Selected != nil || doc.NoCandidateReason != r.NoCandidateReason {
			t.Fatalf("JSON fabricated selection: %#v", doc)
		}
	} else if doc.Selected == nil || *doc.Selected != candidateJSON(*r.Selected) {
		t.Fatalf("JSON changed selected candidate: %#v", doc)
	}
	for _, text := range append([]string{r.Policy, r.Scope, r.ClaimStrength, r.SelectionReason}, r.Limitations...) {
		if !strings.Contains(human.String(), text) {
			t.Fatalf("human output omitted %q: %s", text, &human)
		}
	}
	if r.Scope == resolution.ProcessScope {
		for _, c := range r.Candidates {
			if !strings.Contains(human.String(), fmt.Sprintf("%s (PATH #%d)", c.Path, c.PathIndex)) {
				t.Fatalf("human output lost candidate PATH identity: %s", &human)
			}
		}
		for _, forbidden := range []string{"WINNER", "will run", "will execute"} {
			if strings.Contains(human.String(), forbidden) {
				t.Fatalf("process output overclaims: %s", &human)
			}
		}
		if doc.Shell != nil || strings.Contains(machine.String(), `"winner"`) {
			t.Fatalf("process JSON implies shell winner: %s", &machine)
		}
	}
}

func TestStandaloneInspectionDoesNotExecuteCandidate(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	name := "wwpassiveprocessfixture"
	filename, body := name, "#!/bin/sh\nprintf executed > inspected.marker\n"
	if runtime.GOOS == "windows" {
		filename += ".cmd"
		body = "@echo off\r\necho executed>inspected.marker\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".CMD")
	for _, args := range [][]string{{name}, {name, "--json"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, "test"); code != 0 {
			t.Fatalf("inspection failed: %d %s", code, &stderr)
		}
		if !strings.Contains(strings.ToLower(stdout.String()), strings.ToLower(filename)) {
			t.Fatalf("fixture not enumerated: %s", &stdout)
		}
		if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
			t.Fatalf("inspection executed fixture or marker check failed: %v", err)
		}
	}
}

func TestDoctorPresentationsCarryProcessClaim(t *testing.T) {
	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{Command: command, Candidates: []resolver.Candidate{{Path: "/policy/whichwhy"}, {Path: "/other/whichwhy"}}}, nil
	}
	r, err := inspectDoctor("test", func() (string, error) { return "/running/whichwhy", nil }, resolve)
	if err != nil {
		t.Fatal(err)
	}
	var human bytes.Buffer
	printDoctorResult(&human, r)
	doc := doctorJSONDocument(r)
	if doc.CommandDiscovery.PathSelected != r.Resolution.Selected.Path || doc.CommandDiscovery.Policy != "process-path-order-v1" || doc.CommandDiscovery.ClaimStrength != "policy-only" || doc.CommandDiscovery.ResolutionScope != "process-external" || !reflect.DeepEqual(doc.Limitations, r.Resolution.Limitations) {
		t.Fatalf("doctor JSON lost scoped report: %#v", doc)
	}
	for _, text := range append([]string{r.Resolution.Selected.Path, r.Resolution.Policy, r.Resolution.Scope, r.Resolution.ClaimStrength}, r.Resolution.Limitations...) {
		if !strings.Contains(human.String(), text) {
			t.Fatalf("doctor omitted %q", text)
		}
	}
	if strings.Contains(human.String(), "PATH resolves") || strings.Contains(human.String(), "PATH winner") {
		t.Fatalf("doctor overclaims: %s", &human)
	}
}
