package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/resolution"
	"github.com/adhamcodes/whichwhy/internal/resolver"
)

func TestParseInvocation(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want invocation
	}{
		{nil, invocation{operation: showHelp}},
		{[]string{"git"}, invocation{command: "git"}},
		{[]string{"git", "--json"}, invocation{command: "git", json: true}},
		{[]string{"path"}, invocation{operation: pathOperation, command: "path"}},
		{[]string{"path", "--json"}, invocation{operation: pathOperation, command: "path", json: true}},
		{[]string{"doctor"}, invocation{operation: doctorOperation, command: "doctor"}},
		{[]string{"doctor", "--json"}, invocation{operation: doctorOperation, command: "doctor", json: true}},
		{[]string{"init", "PowerShell"}, invocation{operation: initPowerShell}},
		{[]string{"--help"}, invocation{operation: showHelp}},
		{[]string{"--version"}, invocation{operation: showVersion}},
	} {
		got, err := parseInvocation(tt.args)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("parse %q = %#v, %v; want %#v", tt.args, got, err, tt.want)
		}
	}
}

func TestLiteralInspectionRoutingAndPresentation(t *testing.T) {
	for _, name := range []string{
		"inspect", "path", "doctor", "init", "help", "version", "-h", "--help", "-v", "--version", "--json", "--",
		"__powershell", "__powershell-json", "--other", "name with space", " leading and trailing ",
		"ww*f9", "ww?f9", "ww[f9]", "ww]f9", "ww`f9", "ww`*?[]f9", "apostrophe'name", "unicode-λ-é-界",
	} {
		t.Run(name, func(t *testing.T) {
			for _, found := range []bool{true, false} {
				result := resolver.Result{Command: name}
				if found {
					result.Candidates = []resolver.Candidate{{Path: "/first/fixture", DirectoryIndex: 1}, {Path: "/second/fixture", DirectoryIndex: 3}}
				}
				for _, machine := range []bool{false, true} {
					args := []string{"inspect", name}
					if machine {
						args = append(args, "--json")
					}
					parsed, err := parseInvocation(args)
					if err != nil || parsed.operation != inspectCommand || !parsed.literal || parsed.command != name || parsed.json != machine {
						t.Fatalf("literal parse = %#v, %v", parsed, err)
					}
					var out, errors, expected bytes.Buffer
					called := 0
					code := run(args, &out, &errors, "test", func(command string) (resolver.Result, error) {
						called++
						if command != name {
							t.Fatalf("resolver command = %q, want %q", command, name)
						}
						return result, nil
					})
					report := resolution.ProcessExternal(result)
					if machine {
						printCommandJSON(&expected, &errors, report)
					} else {
						printCommandReport(&expected, report)
					}
					if code != report.ExitCode() || errors.Len() != 0 || called != 1 || out.String() != expected.String() {
						t.Fatalf("literal changed completed report: code=%d errors=%s resolver calls=%d output=%s", code, &errors, called, &out)
					}
					if machine {
						var doc jsonDocument
						if err := json.Unmarshal(out.Bytes(), &doc); err != nil || doc.Command != name || doc.Policy != "process-path-order-v1" || doc.ClaimStrength != "policy-only" || doc.ResolutionScope != "process-external" {
							t.Fatalf("literal JSON = %s; error=%v", &out, err)
						}
					} else if !strings.HasPrefix(out.String(), "WhichWhy — "+name+"\n\n") {
						t.Fatalf("literal heading = %s", &out)
					}
				}
			}
		})
	}
}

func TestMalformedLiteralInvocationCannotDispatchEvidence(t *testing.T) {
	for _, args := range [][]string{
		{"inspect"}, {"inspect", ""}, {"inspect", "", "--json"},
		{"inspect", "one", "two"}, {"inspect", "one", "two", "--json"}, {"inspect", "one", "--json", "extra"},
		{"inspect", "__powershell", "YQ==", "5.1", "Desktop"},
		{"inspect", "__powershell-json", "YQ==", "5.1", "Desktop"},
		{"__powershell"}, {"__powershell-json", "--json"}, {""},
		{"--", "path"}, {"--json", "--", "path"},
	} {
		var out, errors bytes.Buffer
		code := Run(args, &out, &errors, "test")
		if code != 2 || out.Len() != 0 || !strings.HasPrefix(errors.String(), "whichwhy: ") {
			t.Fatalf("%q: code=%d stdout=%s stderr=%s", args, code, &out, &errors)
		}
	}
}

func TestLiteralStandaloneReservedFixtures(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("PATH", dir)
	t.Setenv("PATHEXT", ".CMD")
	for _, name := range []string{"inspect", "path", "doctor", "init", "help", "version", "--help", "--version", "--json", "__powershell", "__powershell-json"} {
		filename, body := name, "#!/bin/sh\necho executed > inspected.marker\n"
		if runtime.GOOS == "windows" {
			filename += ".cmd"
			body = "@echo off\r\necho executed>inspected.marker\r\n"
		}
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"inspect", name}, {"inspect", name, "--json"}} {
			var out, errors bytes.Buffer
			if code := Run(args, &out, &errors, "test"); code != 0 || !strings.Contains(strings.ToLower(out.String()), strings.ToLower(filename)) {
				t.Fatalf("%q: code=%d stdout=%s stderr=%s", args, code, &out, &errors)
			}
			if _, err := os.Stat("inspected.marker"); !os.IsNotExist(err) {
				t.Fatalf("fixture executed or marker observation failed: %v", err)
			}
		}
	}
}
