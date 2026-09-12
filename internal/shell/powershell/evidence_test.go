package powershell

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestDecodeEvidencePreservesPowerShellOrder(t *testing.T) {
	records := []string{
		encodeTestRecord("Alias", "wwprobe", "", "", "Get-Date"),
		encodeTestRecord("Application", "wwprobe.cmd", `C:\first\wwprobe.cmd`, `C:\first\wwprobe.cmd`, ""),
		encodeTestRecord("Application", "wwprobe.cmd", `C:\second\wwprobe.cmd`, `C:\second\wwprobe.cmd`, ""),
	}

	evidence, err := DecodeEvidence("wwprobe", "5.1.26100.9444", "Desktop", records)
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
	_, err := DecodeEvidence("wwprobe", "5.1", "Desktop", []string{bad})
	if !errors.Is(err, ErrMalformedEvidenceRecord) {
		t.Fatalf("error = %v, want ErrMalformedEvidenceRecord", err)
	}
}

func TestDecodeEmptyEvidencePreservesNoMatches(t *testing.T) {
	evidence, err := DecodeEvidence("missing", "5.1", "Desktop", nil)
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
