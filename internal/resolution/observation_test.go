package resolution

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestObservationCompletenessAndPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name                string
		statuses            []resolver.ObservationStatus
		complete, selection string
		code                int
	}{
		{"empty", nil, InspectionComplete, SelectionNone, 1},
		{"ordinary-negatives", []resolver.ObservationStatus{resolver.ObservedNotFound, resolver.ObservedDirectory, resolver.ObservedNotDirectory, resolver.ObservedIneligible}, InspectionComplete, SelectionNone, 1},
		{"incomplete-miss", []resolver.ObservationStatus{resolver.ObservedNotFound, resolver.ObservedError}, InspectionIncomplete, SelectionNone, 1},
		{"before", []resolver.ObservationStatus{resolver.ObservedError, resolver.ObservedCandidate, resolver.ObservedCandidate}, InspectionIncomplete, SelectionUncertain, 0},
		{"after", []resolver.ObservationStatus{resolver.ObservedCandidate, resolver.ObservedError, resolver.ObservedCandidate}, InspectionIncomplete, SelectionDefinitive, 0},
		{"between-repeated-candidate-attempts", []resolver.ObservationStatus{resolver.ObservedCandidate, resolver.ObservedError, resolver.ObservedCandidate}, InspectionIncomplete, SelectionDefinitive, 0},
		{"complete-selection", []resolver.ObservationStatus{resolver.ObservedDirectory, resolver.ObservedCandidate, resolver.ObservedNotFound}, InspectionComplete, SelectionDefinitive, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := resolver.Result{Command: "probe"}
			for i, status := range tc.statuses {
				o := resolver.Observation{Attempt: i + 1, PathIndex: i + 1, Path: "probe", Name: "probe", Status: status}
				if status == resolver.ObservedError {
					o.Error = &resolver.ObservationError{Operation: "stat", Category: "permission-denied", Message: "controlled denial"}
				}
				e.Observations = append(e.Observations, o)
				if status == resolver.ObservedCandidate && (tc.name != "between-repeated-candidate-attempts" || len(e.Candidates) == 0) {
					e.Candidates = append(e.Candidates, resolver.Candidate{Path: "probe", DirectoryIndex: i})
				}
			}
			r := ProcessExternal(e)
			if r.Inspection.Completeness != tc.complete || r.SelectionStatus != tc.selection || r.ExitCode() != tc.code || r.ClaimStrength != "policy-only" || r.Policy != "process-path-order-v1" || r.Scope != "process-external" {
				t.Fatalf("%#v", r)
			}
			if len(e.Observations) > 0 && !reflect.DeepEqual(r.Inspection.Observations, e.Observations) {
				t.Fatal("lost ordered evidence")
			}
			if tc.code == 0 {
				if r.Selected.PathIndex != e.Candidates[0].DirectoryIndex+1 || len(r.Candidates) != len(e.Candidates) {
					t.Fatalf("selection/order changed: %#v", r)
				}
				if tc.selection == SelectionUncertain && (!strings.Contains(r.SelectionReason, "earlier unresolved") || strings.Contains(r.SelectionReason, "First eligible")) {
					t.Fatalf("overclaim: %s", r.SelectionReason)
				}
			} else if strings.Contains(r.NoCandidateReason, "incomplete") != (tc.complete == InspectionIncomplete) {
				t.Fatalf("miss hides completeness: %s", r.NoCandidateReason)
			}
		})
	}
}

func TestCandidateAbsolutePathFailureDoesNotInventPrecedenceFailure(t *testing.T) {
	r := ProcessExternal(resolver.Result{Command: "probe", Candidates: []resolver.Candidate{{Path: "probe"}}, Observations: []resolver.Observation{{Attempt: 1, PathIndex: 1, Path: "probe", Name: "probe", Status: resolver.ObservedCandidate, Error: &resolver.ObservationError{Operation: "absolute-path", Category: "other", Message: "cwd unavailable"}}}})
	if r.Inspection.Completeness != InspectionIncomplete || r.SelectionStatus != SelectionDefinitive || r.ExitCode() != 0 {
		t.Fatalf("%#v", r)
	}
}
