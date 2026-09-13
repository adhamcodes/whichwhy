package powershell

import (
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeEvidencePreservesPowerShellOrder(t *testing.T) {
	records := []string{
		encodeTestRecord("Alias", "wwprobe", "", "", "Get-Date"),
		encodeTestRecord("Application", "wwprobe.cmd", `C:\first\wwprobe.cmd`, `C:\first\wwprobe.cmd`, ""),
		encodeTestRecord("Application", "wwprobe.cmd", `C:\second\wwprobe.cmd`, `C:\second\wwprobe.cmd`, ""),
	}

	evidence, err := DecodeEvidence(encodeTestCommand("wwprobe"), "5.1.26100.9444", "Desktop", records)
	if err != nil {
		t.Fatalf("DecodeEvidence() error = %v", err)
	}
	if len(evidence.Matches) != 3 {
		t.Fatalf("match count = %d, want 3", len(evidence.Matches))
	}

	winner := evidence.Matches[0]
	if winner.CommandType != "Alias" || winner.AliasTarget != "Get-Date" {
		t.Fatalf("winner = %#v, want wwprobe alias to Get-Date", winner)
	}
	if evidence.Matches[1].Path != `C:\first\wwprobe.cmd` {
		t.Fatalf("second match path = %q", evidence.Matches[1].Path)
	}
}

func TestDecodeEvidenceRejectsMalformedRecord(t *testing.T) {
	bad := base64.StdEncoding.EncodeToString([]byte("only-one-field"))
	_, err := DecodeEvidence(encodeTestCommand("wwprobe"), "5.1", "Desktop", []string{bad})
	if !errors.Is(err, ErrMalformedEvidenceRecord) {
		t.Fatalf("error = %v, want ErrMalformedEvidenceRecord", err)
	}
}

func TestDecodeEmptyEvidencePreservesNoMatches(t *testing.T) {
	evidence, err := DecodeEvidence(encodeTestCommand("missing"), "5.1", "Desktop", nil)
	if err != nil {
		t.Fatalf("DecodeEvidence() error = %v", err)
	}
	if len(evidence.Matches) != 0 {
		t.Fatal("DecodeEvidence fabricated a match for empty evidence")
	}
}

func encodeTestRecord(fields ...string) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(fields, recordSeparator)))
}

func encodeTestCommand(command string) string {
	return base64.StdEncoding.EncodeToString([]byte(command))
}

func TestDecodeEvidenceCommandTransport(t *testing.T) {
	for _, command := range []string{
		"normal-name", "name with space", "apostrophe'name", `name"with"quote`,
		`backslash\name`, `trailing-backslash\`, `multiple\\backslashes`,
		"unicode-λ-é-界", `punctuation;,@#$(){}!`, `mixed space "quote" \end\`,
		`mixed \"quote" space\`, " leading and trailing ", " \t\r\n ",
		"decomposed-e\u0301", "supplementary-🙂", "\x00\ufeff\ufffd",
		"bm9ybWFsLW5hbWU=", // Must not decode a second time.
		"*?[]", "help",     // Codec coverage only: F9 bridge routing stays unchanged.
	} {
		t.Run(command, func(t *testing.T) {
			evidence, err := DecodeEvidence(encodeTestCommand(command), "5.1", "Desktop", nil)
			if err != nil || evidence.Command != command {
				t.Fatalf("DecodeEvidence() command = %q, error = %v; want %q", evidence.Command, err, command)
			}
		})
	}
}

func TestDecodeEvidenceRejectsMalformedCommandTransport(t *testing.T) {
	for _, tt := range []struct {
		name, encoded, message string
	}{
		{"empty", "", "empty"},
		{"raw name", "normal-name", "base64"},
		{"alphabet", "!!!!", "base64"},
		{"missing padding", "YQ", "base64"},
		{"extra padding", "YQ===", "base64"},
		{"nonzero pad bits", "YR==", "base64"},
		{"partial decode", "YQ==!!!!", "base64"},
		{"newline", "YQ==\r\n", "base64"},
		{"space", " YQ==", "base64"},
		{"invalid UTF8", "/w==", "UTF-8"},
		{"overlong UTF8", "wK8=", "UTF-8"},
		{"surrogate", "7aCA", "UTF-8"},
		{"truncated UTF8", "4oI=", "UTF-8"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			evidence, err := DecodeEvidence(tt.encoded, "5.1", "Desktop", []string{encodeTestRecord("Alias", "probe", "", "", "target")})
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("error = %v, want %q", err, tt.message)
			}
			if !reflect.DeepEqual(evidence, Evidence{}) {
				t.Fatalf("malformed request returned partial evidence: %#v", evidence)
			}
		})
	}
}
