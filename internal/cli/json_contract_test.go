package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adhamcodes/whichwhy/internal/pathdiag"
	"github.com/adhamcodes/whichwhy/internal/processpath"
	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
)

// These expectations deliberately use literal wire keys and values, never the
// production JSON structs, tags, version constants, or conversion functions.
// Compare parsed values: whitespace and object-key order are not the API.
const contractProcess = `{
  "schema_version":2,"command":"probe","resolution_scope":"process-external",
  "policy":"process-path-order-v1","claim_strength":"policy-only",
  "selected":{"kind":"external","path":"second/probe","path_index":2},
  "candidates":[{"kind":"external","path":"second/probe","path_index":2},{"kind":"external","path":"third/probe","path_index":3}],
  "selection_reason":"<prose>","limitations":["<prose>"],
  "process_path":{"raw_value":"first;second;third","entries":[
    {"index":1,"raw":"first","value":"first"},
    {"index":2,"raw":"second","value":"second"},
    {"index":3,"raw":"third","value":"third"}]},
  "inspection":{"completeness":"complete","observations":[
    {"attempt":1,"path_index":1,"name":"probe","path":"first/probe","status":"not-found","error":{"operation":"stat","category":"not-found","message":"<prose>"}},
    {"attempt":2,"path_index":2,"name":"probe","path":"second/probe","status":"candidate"},
    {"attempt":3,"path_index":3,"name":"probe","path":"third/probe","status":"candidate"}]},
  "selection_status":"definitive"
}`

func contractObject(t *testing.T, data string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(data), &doc); err != nil || doc == nil {
		t.Fatalf("invalid contract object: %v: %s", err, data)
	}
	return doc
}

func contractReport(t *testing.T, stdout, stderr *bytes.Buffer, code, wantCode int) map[string]any {
	t.Helper()
	if code != wantCode || stderr.Len() != 0 {
		t.Fatalf("exit=%d want=%d stderr=%q", code, wantCode, stderr.String())
	}
	data := stdout.Bytes()
	if !utf8.Valid(data) || !bytes.HasSuffix(data, []byte("\n")) || bytes.HasSuffix(data, []byte("\n\n")) || bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatalf("invalid UTF-8/newline/BOM framing: %q", data)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var doc map[string]any
	if err := decoder.Decode(&doc); err != nil || doc == nil {
		t.Fatalf("invalid report: %v: %s", err, data)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("stdout must contain exactly one JSON value: %v", err)
	}
	return doc
}

// Prose content/count/order is not stable, but its presence and types are.
// Only known prose keys are normalized, so renames cannot evade the comparison.
func contractProse(t *testing.T, value any) {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		for key, field := range v {
			switch key {
			case "selection_reason", "no_candidate_reason", "message":
				if s, ok := field.(string); !ok || s == "" {
					t.Fatalf("%s must be nonempty prose: %#v", key, field)
				}
				v[key] = "<prose>"
			case "error":
				if s, ok := field.(string); ok {
					if s == "" {
						t.Fatal("empty optional error must be omitted")
					}
					v[key] = "<prose>"
				} else {
					contractProse(t, field)
				}
			case "limitations":
				items, ok := field.([]any)
				if !ok {
					t.Fatalf("limitations must be an array: %#v", field)
				}
				for _, item := range items {
					if s, ok := item.(string); !ok || s == "" {
						t.Fatalf("invalid limitation: %#v", item)
					}
				}
				if len(items) > 0 {
					v[key] = []any{"<prose>"}
				}
			default:
				contractProse(t, field)
			}
		}
	case []any:
		for _, item := range v {
			contractProse(t, item)
		}
	}
}

func contractEqual(t *testing.T, got, want map[string]any) {
	t.Helper()
	contractProse(t, got)
	if !reflect.DeepEqual(got, want) {
		actual, _ := json.MarshalIndent(got, "", "  ")
		expected, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("wire contract mismatch\ngot:\n%s\nwant:\n%s", actual, expected)
	}
}

func contractEvidence() resolver.Result {
	return resolver.Result{
		Command: "probe",
		Path: processpath.Path{Raw: "first;second;third", Entries: []processpath.Entry{
			{Index: 1, Raw: "first", Value: "first"}, {Index: 2, Raw: "second", Value: "second"}, {Index: 3, Raw: "third", Value: "third"},
		}},
		Candidates: []resolver.Candidate{{Path: "second/probe", DirectoryIndex: 1}, {Path: "third/probe", DirectoryIndex: 2}},
		Observations: []resolver.Observation{
			{Attempt: 1, PathIndex: 1, Name: "probe", Path: "first/probe", Status: resolver.ObservedNotFound, Error: &resolver.ObservationError{Operation: "stat", Category: "not-found", Message: "controlled absence"}},
			{Attempt: 2, PathIndex: 2, Name: "probe", Path: "second/probe", Status: resolver.ObservedCandidate},
			{Attempt: 3, PathIndex: 3, Name: "probe", Path: "third/probe", Status: resolver.ObservedCandidate},
		},
	}
}

func TestJSONContractCommand(t *testing.T) {
	for _, state := range []string{"found", "empty", "miss", "incomplete-miss", "uncertain", "incomplete-after"} {
		for _, literal := range []bool{false, true} {
			t.Run(state+"/literal="+map[bool]string{false: "false", true: "true"}[literal], func(t *testing.T) {
				evidence := contractEvidence()
				want := contractObject(t, contractProcess)
				inspection := want["inspection"].(map[string]any)
				observations := inspection["observations"].([]any)
				code := 0
				if state == "empty" || state == "miss" || state == "incomplete-miss" {
					evidence.Candidates = nil
					evidence.Observations = evidence.Observations[:1]
					inspection["observations"] = observations[:1]
					want["selected"], want["candidates"] = nil, []any{}
					want["selection_status"], want["no_candidate_reason"] = "no-candidate", "<prose>"
					delete(want, "selection_reason")
					code = 1
				}
				if state == "empty" {
					evidence.Path = processpath.Path{}
					evidence.Observations = nil
					want["process_path"] = contractObject(t, `{"raw_value":"","entries":[]}`)
					inspection["observations"] = []any{}
				}
				if state == "uncertain" || state == "incomplete-miss" || state == "incomplete-after" {
					index := 0
					operation, category := "stat", "permission-denied"
					status := "error"
					if state == "incomplete-after" {
						index, operation, category, status = 2, "absolute-path", "other", "candidate"
					}
					evidence.Observations[index].Status = resolver.ObservationStatus(status)
					evidence.Observations[index].Error = &resolver.ObservationError{Operation: operation, Category: category, Message: "controlled failure"}
					observation := observations[index].(map[string]any)
					observation["status"] = status
					observation["error"] = map[string]any{"operation": operation, "category": category, "message": "<prose>"}
					inspection["completeness"] = "incomplete"
					if state == "uncertain" {
						want["selection_status"] = "precedence-uncertain"
					}
				}
				args := []string{"probe", "--json"}
				if literal {
					args = append([]string{"inspect"}, args...)
				}
				var stdout, stderr bytes.Buffer
				exit := run(args, &stdout, &stderr, "test", func(command string) (resolver.Result, error) {
					if command != "probe" {
						t.Fatalf("request identity changed: %q", command)
					}
					return evidence, nil
				})
				contractEqual(t, contractReport(t, &stdout, &stderr, exit, code), want)
			})
		}
	}
}

func TestJSONContractPowerShell(t *testing.T) {
	const expected = `{
	 "schema_version":2,"command":"Module\\PrObE[*]","resolution_scope":"powershell-loaded-session",
	 "policy":"powershell-loaded-session-order-v1","claim_strength":"shell-observed",
	 "shell":{"name":"PowerShell","version":"5.1","edition":"Desktop"},
	 "selected":{"kind":"alias","name":"probe[*]","alias_target":"Target"},
	 "candidates":[
	  {"kind":"alias","name":"probe[*]","alias_target":"Target"},
	  {"kind":"function","name":"probe[*]","source":"Module"},
	  {"kind":"cmdlet","name":"probe[*]","source":"Module"},
	  {"kind":"application","name":"probe[*].exe","path":"tools/probe[*].exe","source":"tools/probe[*].exe"},
	  {"kind":"external-script","name":"probe[*].ps1","path":"tools/probe[*].ps1","source":"tools/probe[*].ps1"},
	  {"kind":"filter","name":"probe[*]"},
	  {"kind":"configuration","name":"probe[*]"}],
	 "selection_reason":"<prose>","limitations":["<prose>"]
	}`
	for _, missing := range []bool{false, true} {
		for _, edition := range []string{"Desktop", "Core", ""} {
			t.Run(edition+map[bool]string{false: "/found", true: "/miss"}[missing], func(t *testing.T) {
				version := "5.1"
				if edition != "Desktop" {
					version = "7.6.5"
				}
				args := []string{"__powershell-json", base64.StdEncoding.EncodeToString([]byte(`Module\PrObE[*]`)), version, edition}
				for _, fields := range [][]string{
					{"Alias", "probe[*]", "", "", "Target"}, {"Function", "probe[*]", "Module", "", ""}, {"Cmdlet", "probe[*]", "Module", "", ""},
					{"Application", "probe[*].exe", "tools/probe[*].exe", "tools/probe[*].exe", ""}, {"ExternalScript", "probe[*].ps1", "tools/probe[*].ps1", "tools/probe[*].ps1", ""},
					{"Filter", "probe[*]", "", "", ""}, {"Configuration", "probe[*]", "", "", ""},
				} {
					if !missing {
						args = append(args, base64.StdEncoding.EncodeToString([]byte(strings.Join(fields, "\x1f"))))
					}
				}
				want := contractObject(t, expected)
				shell := want["shell"].(map[string]any)
				shell["version"], shell["edition"] = version, edition
				if edition == "" {
					delete(shell, "edition")
				}
				code := 0
				if missing {
					code = 1
					want["selected"], want["candidates"] = nil, []any{}
					delete(want, "selection_reason")
					want["no_candidate_reason"] = "<prose>"
				}
				var stdout, stderr bytes.Buffer
				exit := Run(args, &stdout, &stderr, "test")
				contractEqual(t, contractReport(t, &stdout, &stderr, exit, code), want)
			})
		}
	}
}

func TestJSONContractPath(t *testing.T) {
	for _, state := range []string{"diagnostics", "empty", "unset"} {
		t.Run(state, func(t *testing.T) {
			want := contractObject(t, `{"schema_version":1,"kind":"path","scope":"process-path","policy":"process-path-order-v1","raw_value":"","entries":[],"summary":{"entries":0,"missing":0,"duplicate":0,"empty":0,"not_directory":0,"errors":0}}`)
			report := pathdiag.Report{}
			if state == "diagnostics" {
				report = pathdiag.Report{RawValue: `first;missing;first;;file;denied`, Entries: []pathdiag.Entry{
					{Index: 1, Value: "first", EffectiveValue: "first", Directory: true}, {Index: 2, Value: "missing", EffectiveValue: "missing", Missing: true},
					{Index: 3, Value: "first", EffectiveValue: "first", Directory: true, DuplicateOf: 1}, {Index: 4, EffectiveValue: ".", Empty: true, Directory: true},
					{Index: 5, Value: "file", EffectiveValue: "file"}, {Index: 6, Value: "denied", EffectiveValue: "denied", Error: "controlled denial"},
				}, MissingCount: 1, DuplicateCount: 1, EmptyCount: 1, NotDirectoryCount: 1, ErrorCount: 1}
				want["raw_value"] = `first;missing;first;;file;denied`
				want["summary"] = contractObject(t, `{"entries":6,"missing":1,"duplicate":1,"empty":1,"not_directory":1,"errors":1}`)
				want["entries"] = contractObject(t, `{"entries":[
				 {"index":1,"value":"first","effective_value":"first","directory":true,"missing":false,"empty":false},
				 {"index":2,"value":"missing","effective_value":"missing","directory":false,"missing":true,"empty":false},
				 {"index":3,"value":"first","effective_value":"first","directory":true,"missing":false,"empty":false,"duplicate_of":1},
				 {"index":4,"value":"","effective_value":".","directory":true,"missing":false,"empty":true},
				 {"index":5,"value":"file","effective_value":"file","directory":false,"missing":false,"empty":false},
				 {"index":6,"value":"denied","effective_value":"denied","directory":false,"missing":false,"empty":false,"error":"<prose>"}]}`)["entries"]
			}
			code := 0
			if state == "unset" {
				code = 1
				want["error"] = "<prose>"
			}
			var stdout, stderr bytes.Buffer
			exit := runPathJSON(&stdout, &stderr, func(string) (string, bool) { return report.RawValue, state != "unset" }, func(string) pathdiag.Report {
				if state == "unset" {
					t.Fatal("unset PATH was inspected")
				}
				return report
			})
			contractEqual(t, contractReport(t, &stdout, &stderr, exit, code), want)
		})
	}
}

func TestJSONContractDoctor(t *testing.T) {
	for _, state := range []string{"healthy", "alternatives", "different", "missing", "uncertain", "incomplete-miss", "incomplete-after"} {
		t.Run(state, func(t *testing.T) {
			evidence := contractEvidence()
			// Use bare fixture paths so identity comparison is portable and never
			// depends on installed executables or filesystem identity.
			evidence.Candidates = []resolver.Candidate{{Path: "f10-current", DirectoryIndex: 1}}
			want := contractObject(t, `{"schema_version":2,"kind":"doctor","status":"ok","version":"1.2.3","platform":{"os":"","arch":""},"running_executable":"f10-current","command_discovery":{"state":"current","path_selected":"f10-current","resolution_scope":"process-external","policy":"process-path-order-v1","claim_strength":"policy-only","other_candidates":[],"selection_status":"definitive","selection_reason":"<prose>"},"limitations":["<prose>"]}`)
			want["platform"] = map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH}
			discovery := want["command_discovery"].(map[string]any)
			process := contractObject(t, contractProcess)
			discovery["process_path"], discovery["inspection"] = process["process_path"], process["inspection"]
			code := 0
			if state != "healthy" {
				code = 1
				want["status"] = "warning"
			}
			if state == "alternatives" {
				evidence.Candidates = append(evidence.Candidates, resolver.Candidate{Path: "f10-second"}, resolver.Candidate{Path: "f10-third"}, resolver.Candidate{Path: "f10-second"})
				discovery["other_candidates"] = []any{"f10-second", "f10-third"}
			}
			if state == "different" {
				evidence.Candidates[0].Path = "f10-other"
				discovery["state"], discovery["path_selected"] = "different", "f10-other"
			}
			if state == "missing" || state == "incomplete-miss" {
				evidence.Candidates = nil
				evidence.Observations = evidence.Observations[:1]
				discovery["state"], discovery["selection_status"], discovery["no_candidate_reason"] = "missing", "no-candidate", "<prose>"
				delete(discovery, "path_selected")
				delete(discovery, "selection_reason")
				inspection := discovery["inspection"].(map[string]any)
				inspection["observations"] = inspection["observations"].([]any)[:1]
			}
			if state == "uncertain" || state == "incomplete-miss" || state == "incomplete-after" {
				index := 0
				if state == "incomplete-after" {
					index = 2
				} else {
					discovery["state"] = "uncertain"
				}
				evidence.Observations[index].Status = resolver.ObservedError
				evidence.Observations[index].Error = &resolver.ObservationError{Operation: "stat", Category: "permission-denied", Message: "controlled denial"}
				inspection := discovery["inspection"].(map[string]any)
				inspection["completeness"] = "incomplete"
				o := inspection["observations"].([]any)[index].(map[string]any)
				o["status"], o["error"] = "error", map[string]any{"operation": "stat", "category": "permission-denied", "message": "<prose>"}
				if state == "uncertain" {
					discovery["selection_status"] = "precedence-uncertain"
				}
			}
			var stdout, stderr bytes.Buffer
			exit := runDoctorJSON(&stdout, &stderr, "1.2.3", func() (string, error) { return "f10-current", nil }, func(command string) (resolver.Result, error) {
				if command != "whichwhy" {
					t.Fatal(command)
				}
				return evidence, nil
			})
			contractEqual(t, contractReport(t, &stdout, &stderr, exit, code), want)
		})
	}
}

func TestJSONContractMachineErrors(t *testing.T) {
	for _, args := range [][]string{
		{"probe", "extra", "--json"}, {"inspect"}, {"inspect", "", "--json"}, {"inspect", "probe", "extra", "--json"}, {"path", "extra", "--json"}, {"doctor", "extra", "--json"},
		{"__powershell-json"}, {"__powershell-json", "***", "5.1", "Desktop"},
		{"__powershell-json", "cHJvYmU=", "5.1", "Desktop", "***"},
		{"__powershell-json", "cHJvYmU=", "5.1", "Desktop", "YmFk"},
		{"__powershell-json", "/w==", "5.1", "Desktop"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr, "test")
			contractFailure(t, &stdout, &stderr, code)
		})
	}
	for _, args := range [][]string{{"probe", "--json"}, {"inspect", "probe", "--json"}, {"doctor", "--json"}} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr, "test", func(string) (resolver.Result, error) {
			return contractEvidence(), errors.New("controlled operational failure")
		})
		contractFailure(t, &stdout, &stderr, code)
	}
	var stdout, stderr bytes.Buffer
	code := runDoctorJSON(&stdout, &stderr, "test", func() (string, error) { return "", errors.New("controlled executable failure") }, func(string) (resolver.Result, error) {
		t.Fatal("resolver called after executable failure")
		return resolver.Result{}, nil
	})
	contractFailure(t, &stdout, &stderr, code)
}

func contractFailure(t *testing.T, stdout, stderr *bytes.Buffer, code int) {
	t.Helper()
	if code != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "whichwhy: ") || !strings.HasSuffix(stderr.String(), "\n") {
		t.Fatalf("failure contract: exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestJSONContractEncodingAndEmptyArrays(t *testing.T) {
	identity := "e\u0301雪😀 <tag>&\"\\\n\u2028\u2029"
	r := resolution.Report{Command: identity, Limitations: nil, ProcessPath: &processpath.Path{}, Inspection: &resolution.ProcessInspection{Completeness: "complete"}}
	var stdout, stderr bytes.Buffer
	code := printCommandJSON(&stdout, &stderr, r)
	got := contractReport(t, &stdout, &stderr, code, 1)
	if got["command"] != identity || !bytes.Contains(stdout.Bytes(), []byte("雪😀 <tag>&")) || bytes.Contains(stdout.Bytes(), []byte(`\u003c`)) {
		t.Fatalf("Unicode identity/HTML escaping drift: %s", &stdout)
	}
	want := contractObject(t, `{"schema_version":2,"command":"","resolution_scope":"","policy":"","claim_strength":"","selected":null,"candidates":[],"limitations":[],"process_path":{"raw_value":"","entries":[]},"inspection":{"completeness":"complete","observations":[]}}`)
	want["command"] = identity
	contractEqual(t, got, want)
	// Doctor owns the same empty-array guarantee even for a zero-value model.
	stdout.Reset()
	if err := writeJSON(&stdout, doctorJSONDocument(doctorReport{})); err != nil {
		t.Fatal(err)
	}
	doc := contractReport(t, &stdout, &stderr, 0, 0)
	if !reflect.DeepEqual(doc["limitations"], []any{}) || !reflect.DeepEqual(doc["command_discovery"].(map[string]any)["other_candidates"], []any{}) {
		t.Fatalf("doctor empty arrays: %s", &stdout)
	}
}

type contractWriter struct {
	bytes.Buffer
	fail bool
}

func (w *contractWriter) Write(p []byte) (int, error) {
	if w.fail {
		return 0, errors.New("controlled write failure")
	}
	return w.Buffer.Write(p[:len(p)/2]) // Deliberately broken writer: short with nil error.
}

func TestJSONContractWriteFailures(t *testing.T) {
	var stdout bytes.Buffer
	if err := writeJSON(&stdout, make(chan int)); err == nil || stdout.Len() != 0 {
		t.Fatal("encoding failure exposed partial JSON")
	}
	for _, fail := range []bool{false, true} {
		for _, family := range []string{"command", "powershell", "path", "doctor"} {
			t.Run(family+map[bool]string{false: "/short", true: "/failed"}[fail], func(t *testing.T) {
				writer := &contractWriter{fail: fail}
				var stderr bytes.Buffer
				var code int
				switch family {
				case "command":
					code = run([]string{"probe", "--json"}, writer, &stderr, "test", func(string) (resolver.Result, error) { return contractEvidence(), nil })
				case "powershell":
					code = Run([]string{"__powershell-json", "cHJvYmU=", "5.1", "Desktop"}, writer, &stderr, "test")
				case "path":
					code = runPathJSON(writer, &stderr, func(string) (string, bool) { return "", false }, pathdiag.Inspect)
				case "doctor":
					code = runDoctorJSON(writer, &stderr, "test", func() (string, error) { return "current", nil }, func(string) (resolver.Result, error) { return contractEvidence(), nil })
				}
				if code != 2 || !strings.HasPrefix(stderr.String(), "whichwhy: write JSON: ") {
					t.Fatalf("write failure accepted: %d %s", code, &stderr)
				}
				if fail && writer.Len() != 0 {
					t.Fatal("failed write produced output")
				}
			})
		}
	}
}

func TestJSONContractObservationVocabulary(t *testing.T) {
	// Production identifiers are inputs; expected wire strings are independent.
	for _, tc := range []struct {
		status         resolver.ObservationStatus
		want, category string
	}{
		{resolver.ObservedCandidate, "candidate", ""},
		{resolver.ObservedNotFound, "not-found", "not-found"},
		{resolver.ObservedDirectory, "directory", ""},
		{resolver.ObservedNotDirectory, "not-directory", "not-directory"},
		{resolver.ObservedIneligible, "mode-ineligible", ""},
		{resolver.ObservedError, "error", "other"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			o := resolver.Observation{Attempt: 1, PathIndex: 2, Name: "probe", Path: "second/probe", Status: tc.status}
			want := contractObject(t, `{"attempt":1,"path_index":2,"name":"probe","path":"second/probe","status":""}`)
			want["status"] = tc.want
			if tc.category != "" {
				o.Error = &resolver.ObservationError{Operation: "stat", Category: tc.category, Message: "controlled diagnostic"}
				want["error"] = map[string]any{"operation": "stat", "category": tc.category, "message": "<prose>"}
			}
			r := resolution.ProcessExternal(resolver.Result{Command: "probe", Observations: []resolver.Observation{o}})
			var stdout, stderr bytes.Buffer
			code := printCommandJSON(&stdout, &stderr, r)
			doc := contractReport(t, &stdout, &stderr, code, 1)
			got := doc["inspection"].(map[string]any)["observations"].([]any)[0].(map[string]any)
			contractEqual(t, got, want)
		})
	}
}

func TestJSONContractDoctorRetainsLargeTrace(t *testing.T) {
	const attempts = 512
	evidence := resolver.Result{Command: "whichwhy"}
	want := make([]any, 0, attempts)
	for i := 1; i <= attempts; i++ {
		evidence.Observations = append(evidence.Observations, resolver.Observation{Attempt: i, PathIndex: i, Name: "whichwhy.CMD", Path: "repeated/whichwhy.CMD", Status: resolver.ObservedNotFound, Error: &resolver.ObservationError{Operation: "stat", Category: "not-found", Message: "controlled absence"}})
		// Every repeated negative attempt and its original indices survive.
		want = append(want, map[string]any{"attempt": float64(i), "path_index": float64(i), "name": "whichwhy.CMD", "path": "repeated/whichwhy.CMD", "status": "not-found", "error": map[string]any{"operation": "stat", "category": "not-found", "message": "<prose>"}})
	}
	var stdout, stderr bytes.Buffer
	code := runDoctorJSON(&stdout, &stderr, "test", func() (string, error) { return "f10-current", nil }, func(string) (resolver.Result, error) { return evidence, nil })
	doc := contractReport(t, &stdout, &stderr, code, 1)
	inspection := doc["command_discovery"].(map[string]any)["inspection"].(map[string]any)
	contractEqual(t, inspection, map[string]any{"completeness": "complete", "observations": want})
}

func TestJSONContractSelectedIsAuthoritative(t *testing.T) {
	r := resolution.ProcessExternal(contractEvidence())
	r.Selected = &r.Candidates[1]
	r.SelectionReason = "Controlled completed selection"
	var stdout, stderr bytes.Buffer
	code := printCommandJSON(&stdout, &stderr, r)
	want := contractObject(t, contractProcess)
	want["selected"] = want["candidates"].([]any)[1]
	contractEqual(t, contractReport(t, &stdout, &stderr, code, 0), want)
}
