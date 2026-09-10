package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestRunDoctorJSONReportsConsistentDiscovery(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	current := filepath.Join(t.TempDir(), "whichwhy")

	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{
			Command: command,
			Candidates: []resolver.Candidate{
				{Path: current, DirectoryIndex: 2},
			},
		}, nil
	}

	code := runDoctorJSON(&stdout, &stderr, "1.2.3", func() (string, error) { return current, nil }, resolve)
	if code != 0 {
		t.Fatalf("runDoctorJSON() exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var doc jsonDoctorDocument
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if doc.SchemaVersion != 1 || doc.Kind != "doctor" || doc.Status != "ok" || doc.Version != "1.2.3" {
		t.Fatalf("unexpected doctor JSON metadata: %#v", doc)
	}
	if doc.RunningExecutable != current || doc.CommandDiscovery.State != "current" || doc.CommandDiscovery.PathWinner != current {
		t.Fatalf("unexpected doctor discovery: %#v", doc)
	}
	if doc.CommandDiscovery.OtherCandidates == nil || len(doc.CommandDiscovery.OtherCandidates) != 0 {
		t.Fatalf("other candidates = %#v, want non-nil empty array", doc.CommandDiscovery.OtherCandidates)
	}
}

func TestRunDoctorJSONReportsDistinctConflict(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	current := filepath.Join(t.TempDir(), "running", "whichwhy")
	winner := filepath.Join(t.TempDir(), "winner", "whichwhy")
	other := filepath.Join(t.TempDir(), "other", "whichwhy")

	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{
			Command: command,
			Candidates: []resolver.Candidate{
				{Path: winner, DirectoryIndex: 0},
				{Path: other, DirectoryIndex: 1},
			},
		}, nil
	}

	code := runDoctorJSON(&stdout, &stderr, "dev", func() (string, error) { return current, nil }, resolve)
	if code != 1 {
		t.Fatalf("runDoctorJSON() exit code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var doc jsonDoctorDocument
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if doc.Status != "warning" || doc.CommandDiscovery.State != "different" || doc.CommandDiscovery.PathWinner != winner {
		t.Fatalf("unexpected conflict JSON: %#v", doc)
	}
	if got, want := len(doc.CommandDiscovery.OtherCandidates), 1; got != want || doc.CommandDiscovery.OtherCandidates[0] != other {
		t.Fatalf("other candidates = %#v, want [%q]", doc.CommandDiscovery.OtherCandidates, other)
	}
}
