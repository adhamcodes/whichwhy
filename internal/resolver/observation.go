package resolver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

type ObservationStatus string

const (
	ObservedCandidate    ObservationStatus = "candidate"
	ObservedNotFound     ObservationStatus = "not-found"
	ObservedDirectory    ObservationStatus = "directory"
	ObservedNotDirectory ObservationStatus = "not-directory"
	ObservedIneligible   ObservationStatus = "mode-ineligible"
	ObservedError        ObservationStatus = "error"
)

// Observation retains each Stat operand, including repeats, in PATH/name order.
// Path is the actual operand, not the absolute/deduplicated candidate spelling.
type Observation struct {
	Attempt   int               `json:"attempt"`
	PathIndex int               `json:"path_index"`
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Status    ObservationStatus `json:"status"`
	Error     *ObservationError `json:"error,omitempty"`
}

type ObservationError struct {
	Operation string `json:"operation"`
	Category  string `json:"category"`
	Message   string `json:"message"`
}

// Incomplete distinguishes a failed observation from an observed negative.
// A candidate can also carry a failed absolute-path conversion; it stays usable
// with the existing cleaned-path fallback, but collection was incomplete.
func (o Observation) Incomplete() bool {
	return o.Status == ObservedError || (o.Status == ObservedCandidate && o.Error != nil)
}

type candidateEvidence struct {
	Candidates   []Candidate
	Observations []Observation
}

type statFunc func(string) (os.FileInfo, error)
type eligibilityFunc func(os.FileInfo) (bool, error)

// collectCandidates shares retention, not platform eligibility or name rules.
// Dependencies are passed per call so failure tests do not mutate global state.
func collectCandidates(entries []processpath.Entry, names []string, stat statFunc, eligible eligibilityFunc, abs func(string) (string, error)) (candidateEvidence, error) {
	r := candidateEvidence{Candidates: []Candidate{}, Observations: []Observation{}}
	for _, entry := range entries {
		for _, name := range names {
			path := filepath.Join(entry.EffectiveValue(), name)
			o := Observation{Attempt: len(r.Observations) + 1, PathIndex: entry.Index, Name: name, Path: path}
			info, err := stat(path)
			switch {
			case err != nil:
				o.Status, o.Error = classifyStatError(err)
			case info.IsDir():
				o.Status = ObservedDirectory
			default:
				ok, err := eligible(info)
				if err != nil {
					return candidateEvidence{}, fmt.Errorf("candidate eligibility for %q: %w", path, err)
				}
				o.Status = ObservedIneligible
				if ok {
					o.Status = ObservedCandidate
					candidatePath, err := abs(path)
					if err != nil {
						candidatePath = filepath.Clean(path)
						o.Error = &ObservationError{Operation: "absolute-path", Category: "other", Message: err.Error()}
					}
					r.Candidates = append(r.Candidates, Candidate{Path: candidatePath, DirectoryIndex: entry.Index - 1})
				}
			}
			r.Observations = append(r.Observations, o)
		}
	}
	return r, nil
}

func classifyStatError(err error) (ObservationStatus, *ObservationError) {
	status, category := ObservedError, "other"
	switch {
	case notDirectoryError(err):
		status, category = ObservedNotDirectory, "not-directory"
	case errors.Is(err, os.ErrPermission):
		category = "permission-denied"
	case ordinaryNotFound(err):
		status, category = ObservedNotFound, "not-found"
	}
	return status, &ObservationError{Operation: "stat", Category: category, Message: err.Error()}
}
