package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestObservationPresentationsAndDoctor(t *testing.T) {
	for _, tc := range []struct {
		name      string
		statuses  []resolver.ObservationStatus
		discovery doctorDiscovery
	}{
		{"before", []resolver.ObservationStatus{resolver.ObservedError, resolver.ObservedCandidate, resolver.ObservedCandidate}, doctorDiscoveryUncertain},
		{"after", []resolver.ObservationStatus{resolver.ObservedCandidate, resolver.ObservedError, resolver.ObservedCandidate}, doctorDiscoveryCurrent},
		{"missing", []resolver.ObservationStatus{resolver.ObservedError}, doctorDiscoveryUncertain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := resolver.Result{Command: "whichwhy"}
			for i, status := range tc.statuses {
				o := resolver.Observation{Attempt: i + 1, PathIndex: i + 1, Name: "whichwhy", Path: "fixture", Status: status}
				if status == resolver.ObservedError {
					o.Error = &resolver.ObservationError{Operation: "stat", Category: "permission-denied", Message: "controlled denial"}
				}
				e.Observations = append(e.Observations, o)
				if status == resolver.ObservedCandidate {
					e.Candidates = append(e.Candidates, resolver.Candidate{Path: "fixture", DirectoryIndex: i})
				}
			}
			r := resolution.ProcessExternal(e)
			assertReportPresentations(t, r)
			var machine, stderr bytes.Buffer
			printCommandJSON(&machine, &stderr, r)
			var doc jsonDocument
			if err := json.Unmarshal(machine.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(doc.Inspection, r.Inspection) || doc.SelectionStatus != r.SelectionStatus {
				t.Fatalf("JSON lost observations: %s", &machine)
			}
			d, err := inspectDoctor("test", func() (string, error) { return "fixture", nil }, func(string) (resolver.Result, error) { return e, nil })
			if err != nil || d.Status != 1 || d.Discovery != tc.discovery || !reflect.DeepEqual(d.Resolution.Inspection, r.Inspection) {
				t.Fatalf("doctor lost completeness/dedup evidence: %#v %v", d, err)
			}
			var human bytes.Buffer
			printDoctorResult(&human, d)
			j := doctorJSONDocument(d)
			if !reflect.DeepEqual(j.CommandDiscovery.Inspection, r.Inspection) || j.CommandDiscovery.SelectionStatus != r.SelectionStatus || !strings.Contains(human.String(), "controlled denial") {
				t.Fatalf("doctor presentation lost evidence: %#v %s", j, &human)
			}
			if tc.discovery == doctorDiscoveryUncertain && strings.Contains(human.String(), "OK       The process policy selects") {
				t.Fatalf("doctor overclaims: %s", &human)
			}
		})
	}
}

func TestHumanObservationOutputOmitsOrdinaryTrace(t *testing.T) {
	r := resolution.ProcessExternal(resolver.Result{Command: "probe", Observations: []resolver.Observation{
		{Attempt: 1, PathIndex: 1, Name: "probe", Path: "ordinary-missing/probe", Status: resolver.ObservedNotFound, Error: &resolver.ObservationError{Operation: "stat", Category: "not-found", Message: "ordinary negative diagnostic"}},
		{Attempt: 2, PathIndex: 2, Name: "probe", Path: "ordinary-directory/probe", Status: resolver.ObservedDirectory},
	}})
	var human bytes.Buffer
	printCommandReport(&human, r)
	if strings.Contains(human.String(), "ordinary-") || strings.Contains(human.String(), "ordinary negative diagnostic") {
		t.Fatalf("default trace too noisy: %s", &human)
	}
	if len(commandJSONDocument(r).Inspection.Observations) != 2 {
		t.Fatal("presentation simplicity erased evidence")
	}
}
