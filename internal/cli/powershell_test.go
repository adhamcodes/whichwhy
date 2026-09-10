package cli

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestRunPowerShellEvidenceShowsAliasWinner(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	record := base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{
		"Alias", "wwprobe", "", "", "Get-Date",
	}, "\x1f")))

	code := Run([]string{"__powershell", "wwprobe", "5.1.26100.9444", "Desktop", record}, &stdout, &stderr, "dev")
	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0", code)
	}
	output := stdout.String()
	for _, want := range []string{
		"POWERSHELL WINNER",
		"Alias wwprobe -> Get-Date",
		"PowerShell 5.1.26100.9444 (Desktop)",
		"Unloaded module auto-loading is not modeled yet",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunPowerShellEvidenceReportsNoLoadedMatch(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"__powershell", "missing", "5.1", "Desktop"}, &stdout, &stderr, "dev")
	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "No command match was found") {
		t.Fatalf("stdout = %q, want missing match message", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunInitPowerShellPrintsBridge(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"init", "powershell"}, &stdout, &stderr, "dev")
	if code != 0 {
		t.Fatalf("Run() exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "function global:whichwhy") {
		t.Fatalf("stdout = %q, want PowerShell bridge", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
