package powershell

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
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

// Winner returns the first match PowerShell reported.
func (e Evidence) Winner() (Match, bool) {
	if len(e.Matches) == 0 {
		return Match{}, false
	}
	return e.Matches[0], true
}

// DecodeEvidence converts the bridge arguments into typed PowerShell evidence.
func DecodeEvidence(command, version, edition string, encodedRecords []string) (Evidence, error) {
	if command == "" {
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
		Command: command,
		Version: version,
		Edition: edition,
		Matches: matches,
	}, nil
}
