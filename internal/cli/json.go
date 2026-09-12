package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/resolution"
)

const jsonSchemaVersion = 1
const resolutionJSONSchemaVersion = 2

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
	SchemaVersion     int             `json:"schema_version"`
	Command           string          `json:"command"`
	ResolutionScope   string          `json:"resolution_scope"`
	Policy            string          `json:"policy"`
	ClaimStrength     string          `json:"claim_strength"`
	Shell             *jsonShell      `json:"shell,omitempty"`
	Selected          *jsonCandidate  `json:"selected"`
	Candidates        []jsonCandidate `json:"candidates"`
	SelectionReason   string          `json:"selection_reason,omitempty"`
	NoCandidateReason string          `json:"no_candidate_reason,omitempty"`
	Limitations       []string        `json:"limitations"`
}

type jsonPathEntry struct {
	Index          int    `json:"index"`
	Value          string `json:"value"`
	EffectiveValue string `json:"effective_value"`
	Directory      bool   `json:"directory"`
	Missing        bool   `json:"missing"`
	Empty          bool   `json:"empty"`
	DuplicateOf    int    `json:"duplicate_of,omitempty"`
	Error          string `json:"error,omitempty"`
}

type jsonPathSummary struct {
	Entries      int `json:"entries"`
	Missing      int `json:"missing"`
	Duplicate    int `json:"duplicate"`
	Empty        int `json:"empty"`
	NotDirectory int `json:"not_directory"`
	Errors       int `json:"errors"`
}

type jsonPathDocument struct {
	SchemaVersion int             `json:"schema_version"`
	Kind          string          `json:"kind"`
	Scope         string          `json:"scope"`
	Policy        string          `json:"policy"`
	RawValue      string          `json:"raw_value"`
	Entries       []jsonPathEntry `json:"entries"`
	Summary       jsonPathSummary `json:"summary"`
	Error         string          `json:"error,omitempty"`
}

type jsonPlatform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type jsonDoctorDiscovery struct {
	State           string   `json:"state"`
	PathSelected    string   `json:"path_selected,omitempty"`
	ResolutionScope string   `json:"resolution_scope"`
	Policy          string   `json:"policy"`
	ClaimStrength   string   `json:"claim_strength"`
	OtherCandidates []string `json:"other_candidates"`
}

type jsonDoctorDocument struct {
	SchemaVersion     int                 `json:"schema_version"`
	Kind              string              `json:"kind"`
	Status            string              `json:"status"`
	Version           string              `json:"version"`
	Platform          jsonPlatform        `json:"platform"`
	RunningExecutable string              `json:"running_executable"`
	CommandDiscovery  jsonDoctorDiscovery `json:"command_discovery"`
	Limitations       []string            `json:"limitations"`
}

func printCommandJSON(stdout, stderr io.Writer, report resolution.Report) int {
	doc := commandJSONDocument(report)
	if err := writeJSON(stdout, doc); err != nil {
		fmt.Fprintf(stderr, "whichwhy: write JSON: %v\n", err)
		return 2
	}
	return report.ExitCode()
}

func runPathJSON(stdout, stderr io.Writer, lookup envLookup, inspect pathInspector) int {
	value, ok := lookup("PATH")
	if !ok {
		doc := jsonPathDocument{
			SchemaVersion: jsonSchemaVersion,
			Kind:          "path",
			Scope:         "process-path",
			Policy:        resolution.ProcessPolicy,
			Entries:       []jsonPathEntry{},
			Error:         "PATH is not set",
		}
		if err := writeJSON(stdout, doc); err != nil {
			fmt.Fprintf(stderr, "whichwhy: write JSON: %v\n", err)
			return 2
		}
		return 1
	}

	doc := pathJSONDocument(inspect(value))
	if err := writeJSON(stdout, doc); err != nil {
		fmt.Fprintf(stderr, "whichwhy: write JSON: %v\n", err)
		return 2
	}
	return 0
}

func runDoctorJSON(stdout, stderr io.Writer, version string, executable executableLocator, resolve externalResolver) int {
	report, err := inspectDoctor(version, executable, resolve)
	if err != nil {
		fmt.Fprintf(stderr, "whichwhy: %v\n", err)
		return 2
	}

	doc := doctorJSONDocument(report)
	if err := writeJSON(stdout, doc); err != nil {
		fmt.Fprintf(stderr, "whichwhy: write JSON: %v\n", err)
		return 2
	}
	return report.Status
}

func commandJSONDocument(report resolution.Report) jsonDocument {
	candidates := make([]jsonCandidate, 0, len(report.Candidates))
	for _, candidate := range report.Candidates {
		candidates = append(candidates, candidateJSON(candidate))
	}
	doc := jsonDocument{
		SchemaVersion:     resolutionJSONSchemaVersion,
		Command:           report.Command,
		ResolutionScope:   report.Scope,
		Policy:            report.Policy,
		ClaimStrength:     report.ClaimStrength,
		Candidates:        candidates,
		SelectionReason:   report.SelectionReason,
		NoCandidateReason: report.NoCandidateReason,
		Limitations:       report.Limitations,
	}
	if report.Shell != nil {
		doc.Shell = &jsonShell{Name: report.Shell.Name, Version: report.Shell.Version, Edition: report.Shell.Edition}
	}
	if report.Selected != nil {
		selected := candidateJSON(*report.Selected)
		doc.Selected = &selected
	}
	return doc
}

func candidateJSON(c resolution.Candidate) jsonCandidate {
	return jsonCandidate{Kind: normalizePowerShellKind(c.Type), Name: c.Name, Path: c.Path, Source: c.Source, AliasTarget: c.AliasTarget, PathIndex: c.PathIndex}
}

func pathJSONDocument(report pathdiag.Report) jsonPathDocument {
	entries := make([]jsonPathEntry, 0, len(report.Entries))
	for _, entry := range report.Entries {
		entries = append(entries, jsonPathEntry{
			Index:          entry.Index,
			Value:          entry.Value,
			EffectiveValue: entry.EffectiveValue,
			Directory:      entry.Directory,
			Missing:        entry.Missing,
			Empty:          entry.Empty,
			DuplicateOf:    entry.DuplicateOf,
			Error:          entry.Error,
		})
	}

	return jsonPathDocument{
		SchemaVersion: jsonSchemaVersion,
		Kind:          "path",
		Scope:         "process-path",
		Policy:        resolution.ProcessPolicy,
		RawValue:      report.RawValue,
		Entries:       entries,
		Summary: jsonPathSummary{
			Entries:      len(report.Entries),
			Missing:      report.MissingCount,
			Duplicate:    report.DuplicateCount,
			Empty:        report.EmptyCount,
			NotDirectory: report.NotDirectoryCount,
			Errors:       report.ErrorCount,
		},
	}
}

func doctorJSONDocument(report doctorReport) jsonDoctorDocument {
	otherCandidates := make([]string, 0)
	pathSelected := ""
	if report.Resolution.Selected != nil {
		pathSelected = report.Resolution.Selected.Path
	}
	for _, candidate := range report.Resolution.Alternatives {
		otherCandidates = append(otherCandidates, candidate.Path)
	}

	status := "ok"
	if report.Status != 0 {
		status = "warning"
	}

	return jsonDoctorDocument{
		SchemaVersion: resolutionJSONSchemaVersion,
		Kind:          "doctor",
		Status:        status,
		Version:       report.Version,
		Platform: jsonPlatform{
			OS:   report.OS,
			Arch: report.Arch,
		},
		RunningExecutable: report.RunningExecutable,
		CommandDiscovery: jsonDoctorDiscovery{
			State:           string(report.Discovery),
			PathSelected:    pathSelected,
			ResolutionScope: report.Resolution.Scope,
			Policy:          report.Resolution.Policy,
			ClaimStrength:   report.Resolution.ClaimStrength,
			OtherCandidates: otherCandidates,
		},
		Limitations: report.Resolution.Limitations,
	}
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
