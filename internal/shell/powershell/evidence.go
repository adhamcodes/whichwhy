package powershell

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const recordSeparator = "\x1f"

var ErrMalformedEvidenceRecord = errors.New("malformed PowerShell evidence record")

// Match is one command candidate reported by the active PowerShell session.
// Matches are kept in the order PowerShell reported them.
type Match struct {
	CommandType string
	Name        string
	Source      string
	Path        string
	AliasTarget string
}

// Evidence is the minimal command-resolution information collected from an
// active PowerShell session.
type Evidence struct {
	Command string
	Version string
	Edition string
	Matches []Match
}

// DecodeEvidence converts the hidden entrypoint arguments into typed evidence.
// encodedCommand is canonical padded Base64 of nonempty UTF-8, never a raw name.
// Human and JSON callers decode here once, before resolution or presentation.
func DecodeEvidence(encodedCommand, version, edition string, encodedRecords []string) (Evidence, error) {
	command, err := base64.StdEncoding.Strict().DecodeString(encodedCommand)
	// Go's Base64 decoder ignores CR/LF even in Strict mode; the protocol does not.
	if err != nil || strings.ContainsAny(encodedCommand, "\r\n") {
		return Evidence{}, errors.New("malformed PowerShell command: expected canonical padded base64")
	}
	if !utf8.Valid(command) {
		return Evidence{}, errors.New("malformed PowerShell command: invalid UTF-8")
	}
	if len(command) == 0 {
		return Evidence{}, errors.New("PowerShell command name is empty")
	}

	matches := make([]Match, 0, len(encodedRecords))
	for index, encoded := range encodedRecords {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return Evidence{}, fmt.Errorf("%w at index %d: invalid base64: %v", ErrMalformedEvidenceRecord, index, err)
		}

		fields := strings.Split(string(decoded), recordSeparator)
		if len(fields) != 5 {
			return Evidence{}, fmt.Errorf("%w at index %d: got %d fields, want 5", ErrMalformedEvidenceRecord, index, len(fields))
		}

		matches = append(matches, Match{
			CommandType: fields[0],
			Name:        fields[1],
			Source:      fields[2],
			Path:        fields[3],
			AliasTarget: fields[4],
		})
	}

	return Evidence{
		Command: string(command),
		Version: version,
		Edition: edition,
		Matches: matches,
	}, nil
}
