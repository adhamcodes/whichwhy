package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestRunDoctorReportsConsistentCommandDiscovery(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	current := filepath.Join(t.TempDir(), "whichwhy")

	resolve := func(command string) (resolver.Result, error) {
		if command != "whichwhy" {
			t.Fatalf("resolve command = %q, want whichwhy", command)
		}
		return resolver.Result{
			Command: command,
			Candidates: []resolver.Candidate{
				{Path: current},
			},
		}, nil
	}

	code := runDoctor(&stdout, &stderr, "1.2.3", func() (string, error) { return current, nil }, resolve)
	if code != 0 {
		t.Fatalf("runDoctor() exit code = %d, want 0", code)
	}
	output := stdout.String()
	for _, want := range []string{"WhichWhy — doctor", "1.2.3", "The process policy selects this running executable for 'whichwhy'", "OK — command discovery is consistent", "changed nothing"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDoctorIgnoresRepeatedDiscoveryOfSameExecutable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	current := filepath.Join(t.TempDir(), "whichwhy")

	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{
			Command: command,
			Candidates: []resolver.Candidate{
				{Path: current, DirectoryIndex: 2},
				{Path: current, DirectoryIndex: 5},
			},
		}, nil
	}

	code := runDoctor(&stdout, &stderr, "dev", func() (string, error) { return current, nil }, resolve)
	if code != 0 {
		t.Fatalf("runDoctor() exit code = %d, want 0", code)
	}
	output := stdout.String()
	if strings.Contains(output, "OTHER WHICHWHY CANDIDATES") || !strings.Contains(output, "OK — command discovery is consistent") {
		t.Fatalf("stdout = %q, want repeated references to same executable treated as one candidate", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDoctorWarnsWhenCommandIsNotOnPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	current := filepath.Join(t.TempDir(), "whichwhy")

	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{Command: command}, nil
	}

	code := runDoctor(&stdout, &stderr, "dev", func() (string, error) { return current, nil }, resolve)
	if code != 1 {
		t.Fatalf("runDoctor() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "No 'whichwhy' candidate was observed under the process policy") {
		t.Fatalf("stdout = %q, want not-discoverable warning", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDoctorWarnsWhenDifferentExecutableWins(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	current := filepath.Join(t.TempDir(), "running", "whichwhy")
	winner := filepath.Join(t.TempDir(), "path", "whichwhy")

	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{
			Command: command,
			Candidates: []resolver.Candidate{
				{Path: winner},
			},
		}, nil
	}

	code := runDoctor(&stdout, &stderr, "dev", func() (string, error) { return current, nil }, resolve)
	if code != 1 {
		t.Fatalf("runDoctor() exit code = %d, want 1", code)
	}
	output := stdout.String()
	for _, want := range []string{"different executable", winner, current} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunDoctorWarnsAboutAdditionalCandidates(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	current := filepath.Join(t.TempDir(), "whichwhy")
	other := filepath.Join(t.TempDir(), "whichwhy")

	resolve := func(command string) (resolver.Result, error) {
		return resolver.Result{
			Command: command,
			Candidates: []resolver.Candidate{
				{Path: current},
				{Path: other},
			},
		}, nil
	}

	code := runDoctor(&stdout, &stderr, "dev", func() (string, error) { return current, nil }, resolve)
	if code != 1 {
		t.Fatalf("runDoctor() exit code = %d, want 1", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "OTHER WHICHWHY CANDIDATES") || !strings.Contains(output, other) {
		t.Fatalf("stdout = %q, want additional candidate", output)
	}
}

func TestRunDoctorReportsOperationalErrors(t *testing.T) {
	t.Run("executable", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		code := runDoctor(&stdout, &stderr, "dev", func() (string, error) { return "", errors.New("boom") }, func(string) (resolver.Result, error) {
			t.Fatal("resolver should not run when executable lookup fails")
			return resolver.Result{}, nil
		})
		if code != 2 || !strings.Contains(stderr.String(), "locate running executable") {
			t.Fatalf("code = %d stderr = %q", code, stderr.String())
		}
	})

	t.Run("resolver", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		current := filepath.Join(t.TempDir(), "whichwhy")

		code := runDoctor(&stdout, &stderr, "dev", func() (string, error) { return current, nil }, func(string) (resolver.Result, error) {
			return resolver.Result{}, errors.New("boom")
		})
		if code != 2 || !strings.Contains(stderr.String(), "inspect command discovery") {
			t.Fatalf("code = %d stderr = %q", code, stderr.String())
		}
	})
}
