package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestRunWithoutArgumentsShowsHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(nil, &stdout, &stderr, "dev")

	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q, want usage text", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--version"}, &stdout, &stderr, "1.2.3")

	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0", code)
	}
	if got, want := stdout.String(), "whichwhy 1.2.3\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCommandShowsPolicySelectionAndAlternatives(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{
			Command: command,
			Candidates: []resolver.Candidate{
				{Path: `/first/python`},
				{Path: `/second/python`},
			},
		}, nil
	}

	code := run([]string{"python"}, &stdout, &stderr, "dev", resolve)

	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0", code)
	}
	output := stdout.String()
	for _, want := range []string{"PROCESS POLICY SELECTED CANDIDATE", `/first/python`, "OTHER CANDIDATES UNDER THIS POLICY", `/second/python`, "CURRENT LIMIT"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCommandReportsMissingExternalCandidate(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{Command: command}, nil
	}

	code := run([]string{"missing"}, &stdout, &stderr, "dev", resolve)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "No external command candidate") {
		t.Fatalf("stdout = %q, want missing-candidate message", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCommandReportsResolverError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	resolve := func(string) (resolver.Result, error) {
		return resolver.Result{}, errors.New("resolver failed")
	}

	code := run([]string{"python"}, &stdout, &stderr, "dev", resolve)

	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "resolver failed") {
		t.Fatalf("stderr = %q, want resolver error", stderr.String())
	}
}
