package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolution"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

func TestRunPowerShellEvidenceShowsAliasWinner(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	record := base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{
		"Alias", "wwprobe", "", "", "Get-Date",
	}, "\x1f")))

	code := Run([]string{"__powershell", encodeTestCommand("wwprobe"), "5.1.26100.9444", "Desktop", record}, &stdout, &stderr, "dev")
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

	code := Run([]string{"__powershell", encodeTestCommand("missing"), "5.1", "Desktop"}, &stdout, &stderr, "dev")
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

func encodeTestCommand(command string) string {
	return base64.StdEncoding.EncodeToString([]byte(command))
}

func TestPowerShellTransportHumanJSONIdentity(t *testing.T) {
	for _, command := range []string{
		"normal-name", "name with space", "apostrophe'name", `name"with"quote`,
		`backslash\name`, `trailing-backslash\`, `multiple\\backslashes`,
		"unicode-λ-é-界", `punctuation;,@#$(){}!`, `mixed space "quote" \end\`,
		" leading and trailing ", " \t\r\n ", "decomposed-e\u0301", "supplementary-🙂",
		"\x00\ufeff\ufffd", "bm9ybWFsLW5hbWU=",
	} {
		t.Run(command, func(t *testing.T) {
			encoded := encodeTestCommand(command)
			evidence, err := ps.DecodeEvidence(encoded, "5.1", "Desktop", nil)
			if err != nil {
				t.Fatal(err)
			}
			if report := resolution.PowerShell(evidence); report.Command != command {
				t.Fatalf("report command = %q, want %q", report.Command, command)
			}
			for _, entry := range []string{"__powershell", "__powershell-json"} {
				var stdout, stderr bytes.Buffer
				if code := Run([]string{entry, encoded, "5.1", "Desktop"}, &stdout, &stderr, "dev"); code != 1 || stderr.Len() != 0 {
					t.Fatalf("%s: code = %d, stderr = %s", entry, code, stderr.String())
				}
				if entry == "__powershell-json" {
					var doc jsonDocument
					if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil || doc.Command != command {
						t.Fatalf("JSON command = %q, error = %v; want %q", doc.Command, err, command)
					}
				} else if !strings.HasPrefix(stdout.String(), "WhichWhy — "+command+"\n\n") {
					t.Fatalf("human heading did not retain exact command: %q", stdout.String())
				}
			}
		})
	}
}

func TestPowerShellTransportMalformedEntrypoints(t *testing.T) {
	for _, entry := range []string{"__powershell", "__powershell-json"} {
		for _, args := range [][]string{
			nil, {"YQ=="}, {"YQ==", "5.1"},
			{"", "5.1", "Desktop"}, {"normal-name", "5.1", "Desktop"},
			{"!!!!", "5.1", "Desktop"}, {"YQ", "5.1", "Desktop"},
			{"YR==", "5.1", "Desktop"}, {"YQ==\r\n", "5.1", "Desktop"},
			{"YQ==!!!!", "5.1", "Desktop"}, {"/w==", "5.1", "Desktop"},
			{"YQ==", "5.1", "Desktop", "!!!!"},
		} {
			var stdout, stderr bytes.Buffer
			code := Run(append([]string{entry}, args...), &stdout, &stderr, "dev")
			if code != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "whichwhy: ") {
				t.Fatalf("%s %q: code = %d, stdout = %q, stderr = %q", entry, args, code, stdout.String(), stderr.String())
			}
		}
	}
}
