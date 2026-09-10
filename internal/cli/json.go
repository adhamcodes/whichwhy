package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/resolver"
	ps "github.com/adhamcodes/whichwhy/internal/shell/powershell"
)

const jsonSchemaVersion = 1

type jsonShell struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Edition string `json:"edition,omitempty"`
}

type jsonCandidate struct {
	Kind        string `json:"kind"`
	Name        string `json:"name,omitempty"`
	Path        string `json:"path,omitempty"`
	Source      string `json:"source,omitempty"`
	AliasTarget string `json:"alias_target,omitempty"`
	PathIndex   int    `json:"path_index,omitempty"`
}

type jsonDocument struct {
	SchemaVersion   int             `json:"schema_version"`
	Command         string          `json:"command"`
	ResolutionScope string          `json:"resolution_scope"`
	Shell           *jsonShell      `json:"shell,omitempty"`
	Winner          *jsonCandidate  `json:"winner"`
	Candidates      []jsonCandidate `json:"candidates"`
	WinnerReason    string          `json:"winner_reason,omitempty"`
	Limitations     []string        `json:"limitations"`
}

func printExternalJSON(stdout, stderr io.Writer, result resolver.Result) int {
	doc := externalJSONDocument(result)
	if err := writeJSON(stdout, doc); err != nil {
		fmt.Fprintf(stderr, "whichwhy: write JSON: %v\n", err)
		return 2
	}
	if doc.Winner == nil {
		return 1
	}
	return 0
}

func printPowerShellJSON(stdout, stderr io.Writer, evidence ps.Evidence) int {
	doc := powerShellJSONDocument(evidence)
	if err := writeJSON(stdout, doc); err != nil {
		fmt.Fprintf(stderr, "whichwhy: write JSON: %v\n", err)
		return 2
	}
	if doc.Winner == nil {
		return 1
	}
	return 0
}

func externalJSONDocument(result resolver.Result) jsonDocument {
	candidates := make([]jsonCandidate, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		candidates = append(candidates, jsonCandidate{
			Kind:      "external",
			Path:      candidate.Path,
			PathIndex: candidate.DirectoryIndex + 1,
		})
	}

	doc := jsonDocument{
		SchemaVersion:   jsonSchemaVersion,
		Command:         result.Command,
		ResolutionScope: "process-external",
		Candidates:      candidates,
		Limitations: []string{
			"Shell-local aliases, functions, built-ins, cmdlets, and command caches are not inspected.",
		},
	}
	if len(candidates) > 0 {
		winner := candidates[0]
		doc.Winner = &winner
		doc.WinnerReason = "first candidate in process-visible external command search order"
	}
	return doc
}

func powerShellJSONDocument(evidence ps.Evidence) jsonDocument {
	candidates := make([]jsonCandidate, 0, len(evidence.Matches))
	for _, match := range evidence.Matches {
		candidates = append(candidates, jsonCandidate{
			Kind:        normalizePowerShellKind(match.CommandType),
			Name:        match.Name,
			Path:        match.Path,
			Source:      match.Source,
			AliasTarget: match.AliasTarget,
		})
	}

	doc := jsonDocument{
		SchemaVersion:   jsonSchemaVersion,
		Command:         evidence.Command,
		ResolutionScope: "powershell-loaded-session",
		Shell: &jsonShell{
			Name:    "PowerShell",
			Version: evidence.Version,
			Edition: evidence.Edition,
		},
		Candidates: candidates,
		Limitations: []string{
			"Unloaded module auto-loading is not modeled yet.",
		},
	}
	if len(candidates) > 0 {
		winner := candidates[0]
		doc.Winner = &winner
		doc.WinnerReason = "first match reported by PowerShell for the current loaded session"
	}
	return doc
}

func normalizePowerShellKind(commandType string) string {
	switch commandType {
	case "ExternalScript":
		return "external-script"
	default:
		return strings.ToLower(commandType)
	}
}

func writeJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
