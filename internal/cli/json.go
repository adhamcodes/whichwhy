package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/processpath"
	"github.com/adhamcodes/whichwhy/internal/resolution"
)

// Versions belong to document families, not the shared evidence model.
// See docs/json-contract.md before changing any public shape.
const (
	commandJSONSchemaVersion = 2
	pathJSONSchemaVersion    = 1
	doctorJSONSchemaVersion  = 2
)

// Presentation-owned evidence types keep internal model changes out of the API.
type jsonProcessPath struct {
	Raw     string                 `json:"raw_value"`
	Entries []jsonProcessPathEntry `json:"entries"`
}

type jsonProcessPathEntry struct {
	Index int    `json:"index"`
	Raw   string `json:"raw"`
	Value string `json:"value"`
}

type jsonInspection struct {
	Completeness string            `json:"completeness"`
	Observations []jsonObservation `json:"observations"`
}

type jsonObservation struct {
	Attempt   int                   `json:"attempt"`
	PathIndex int                   `json:"path_index"`
	Name      string                `json:"name"`
	Path      string                `json:"path"`
	Status    string                `json:"status"`
	Error     *jsonObservationError `json:"error,omitempty"`
}

type jsonObservationError struct {
	Operation string `json:"operation"`
	Category  string `json:"category"`
	Message   string `json:"message"`
}

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
	SchemaVersion     int              `json:"schema_version"`
	Command           string           `json:"command"`
	ResolutionScope   string           `json:"resolution_scope"`
	Policy            string           `json:"policy"`
	ClaimStrength     string           `json:"claim_strength"`
	Shell             *jsonShell       `json:"shell,omitempty"`
	Selected          *jsonCandidate   `json:"selected"`
	Candidates        []jsonCandidate  `json:"candidates"`
	SelectionReason   string           `json:"selection_reason,omitempty"`
	NoCandidateReason string           `json:"no_candidate_reason,omitempty"`
	Limitations       []string         `json:"limitations"`
	ProcessPath       *jsonProcessPath `json:"process_path,omitempty"`
	Inspection        *jsonInspection  `json:"inspection,omitempty"`
	SelectionStatus   string           `json:"selection_status,omitempty"`
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
	State             string           `json:"state"`
	PathSelected      string           `json:"path_selected,omitempty"`
	ResolutionScope   string           `json:"resolution_scope"`
	Policy            string           `json:"policy"`
	ClaimStrength     string           `json:"claim_strength"`
	OtherCandidates   []string         `json:"other_candidates"`
	ProcessPath       *jsonProcessPath `json:"process_path,omitempty"`
	Inspection        *jsonInspection  `json:"inspection,omitempty"`
	SelectionStatus   string           `json:"selection_status,omitempty"`
	SelectionReason   string           `json:"selection_reason,omitempty"`
	NoCandidateReason string           `json:"no_candidate_reason,omitempty"`
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
		printError(stderr, "write JSON: "+err.Error())
		return 2
	}
	return report.ExitCode()
}

func runPathJSON(stdout, stderr io.Writer, lookup envLookup, inspect pathInspector) int {
	value, ok := lookup("PATH")
	if !ok {
		doc := jsonPathDocument{
			SchemaVersion: pathJSONSchemaVersion,
			Kind:          "path",
			Scope:         "process-path",
			Policy:        resolution.ProcessPolicy,
			Entries:       []jsonPathEntry{},
			Error:         "PATH is not set",
		}
		if err := writeJSON(stdout, doc); err != nil {
			printError(stderr, "write JSON: "+err.Error())
			return 2
		}
		return 1
	}

	doc := pathJSONDocument(inspect(value))
	if err := writeJSON(stdout, doc); err != nil {
		printError(stderr, "write JSON: "+err.Error())
		return 2
	}
	return 0
}

func runDoctorJSON(stdout, stderr io.Writer, version string, executable executableLocator, resolve externalResolver) int {
	report, err := inspectDoctor(version, executable, resolve)
	if err != nil {
		printError(stderr, err.Error())
		return 2
	}

	doc := doctorJSONDocument(report)
	if err := writeJSON(stdout, doc); err != nil {
		printError(stderr, "write JSON: "+err.Error())
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
		SchemaVersion:     commandJSONSchemaVersion,
		Command:           report.Command,
		ResolutionScope:   report.Scope,
		Policy:            report.Policy,
		ClaimStrength:     report.ClaimStrength,
		Candidates:        candidates,
		SelectionReason:   report.SelectionReason,
		NoCandidateReason: report.NoCandidateReason,
		Limitations:       append([]string{}, report.Limitations...),
		ProcessPath:       processPathJSON(report.ProcessPath),
		Inspection:        inspectionJSON(report.Inspection),
		SelectionStatus:   report.SelectionStatus,
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
		SchemaVersion: pathJSONSchemaVersion,
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
		SchemaVersion: doctorJSONSchemaVersion,
		Kind:          "doctor",
		Status:        status,
		Version:       report.Version,
		Platform: jsonPlatform{
			OS:   report.OS,
			Arch: report.Arch,
		},
		RunningExecutable: report.RunningExecutable,
		CommandDiscovery: jsonDoctorDiscovery{
			State:             string(report.Discovery),
			PathSelected:      pathSelected,
			ResolutionScope:   report.Resolution.Scope,
			Policy:            report.Resolution.Policy,
			ClaimStrength:     report.Resolution.ClaimStrength,
			OtherCandidates:   otherCandidates,
			ProcessPath:       processPathJSON(report.Resolution.ProcessPath),
			Inspection:        inspectionJSON(report.Resolution.Inspection),
			SelectionStatus:   report.Resolution.SelectionStatus,
			SelectionReason:   report.Resolution.SelectionReason,
			NoCandidateReason: report.Resolution.NoCandidateReason,
		},
		Limitations: append([]string{}, report.Resolution.Limitations...),
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
	// Finish serialization before exposing bytes. A transport failure may still
	// accept a prefix; no writer API can retract bytes from a pipe or terminal.
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	n, err := stdout.Write(buffer.Bytes())
	if err == nil && n != buffer.Len() {
		return io.ErrShortWrite
	}
	return err
}

func processPathJSON(path *processpath.Path) *jsonProcessPath {
	if path == nil {
		return nil
	}
	doc := &jsonProcessPath{Raw: path.Raw, Entries: make([]jsonProcessPathEntry, 0, len(path.Entries))}
	for _, entry := range path.Entries {
		doc.Entries = append(doc.Entries, jsonProcessPathEntry{Index: entry.Index, Raw: entry.Raw, Value: entry.Value})
	}
	return doc
}

func inspectionJSON(inspection *resolution.ProcessInspection) *jsonInspection {
	if inspection == nil {
		return nil
	}
	doc := &jsonInspection{Completeness: inspection.Completeness, Observations: make([]jsonObservation, 0, len(inspection.Observations))}
	for _, observation := range inspection.Observations {
		o := jsonObservation{Attempt: observation.Attempt, PathIndex: observation.PathIndex, Name: observation.Name, Path: observation.Path, Status: string(observation.Status)}
		if observation.Error != nil {
			o.Error = &jsonObservationError{Operation: observation.Error.Operation, Category: observation.Error.Category, Message: observation.Error.Message}
		}
		doc.Observations = append(doc.Observations, o)
	}
	return doc
}
