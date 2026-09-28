package pathdiag

import (
	"fmt"
	"os"
	"reflect"
	"syscall"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func TestWindowsPATHObservationErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		missing bool
	}{
		{"file-not-found", syscall.Errno(2), true},
		{"path-not-found", syscall.Errno(3), true},
		{"enoent", syscall.ENOENT, true},
		{"not-exist-sentinel", os.ErrNotExist, true},
		{"bad-network-path", syscall.Errno(53), false},
		{"access-denied", syscall.ERROR_ACCESS_DENIED, false},
		{"invalid-directory", syscall.Errno(267), false},
	} {
		for _, wrapping := range []string{"bare", "path-error", "nested"} {
			t.Run(tc.name+"/"+wrapping, func(t *testing.T) {
				failure := tc.err
				if wrapping != "bare" {
					failure = &os.PathError{Op: "stat", Path: "fixture", Err: failure}
				}
				if wrapping == "nested" {
					failure = fmt.Errorf("observation: %w", failure)
				}
				r := inspectPath(processpath.Parse("fixture"), func(string) (os.FileInfo, error) { return nil, failure })
				if len(r.Entries) != 1 || r.Entries[0].Index != 1 || r.Entries[0].Value != "fixture" || r.Entries[0].Missing != tc.missing {
					t.Fatalf("wrong entry classification: %#v", r)
				}
				wantMissing, wantErrors, wantError := 0, 1, failure.Error()
				if tc.missing {
					wantMissing, wantErrors, wantError = 1, 0, ""
				}
				if r.MissingCount != wantMissing || r.ErrorCount != wantErrors || r.Entries[0].Error != wantError || r.NotDirectoryCount != 0 || r.EmptyCount != 0 || r.DuplicateCount != 0 {
					t.Fatalf("failure/counts lost: %#v", r)
				}
			})
		}
	}
}

func TestWindowsPATHMixedObservationCounts(t *testing.T) {
	dir := t.TempDir()
	directory, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(dir, "file")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	regular, err := os.Stat(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	value := "missing-file;missing-path;network;denied;;file;network"
	var operands []string
	r := inspectPath(processpath.Parse(value), func(path string) (os.FileInfo, error) {
		operands = append(operands, path)
		var failure error
		switch path {
		case "missing-file":
			failure = syscall.Errno(2)
		case "missing-path":
			failure = syscall.Errno(3)
		case "network":
			failure = syscall.Errno(53)
		case "denied":
			failure = syscall.ERROR_ACCESS_DENIED
		case ".":
			return directory, nil
		case "file":
			return regular, nil
		default:
			t.Fatalf("unexpected operand %q", path)
		}
		return nil, fmt.Errorf("observation: %w", &os.PathError{Op: "stat", Path: path, Err: failure})
	})
	want := []string{"missing-file", "missing-path", "network", "denied", ".", "file", "network"}
	if !reflect.DeepEqual(operands, want) || r.RawValue != value || len(r.Entries) != len(want) {
		t.Fatalf("order/operands changed: %v %#v", operands, r)
	}
	for i, entry := range r.Entries {
		if entry.Index != i+1 || entry.EffectiveValue != want[i] {
			t.Fatalf("entry identity/index changed: %#v", entry)
		}
	}
	if r.MissingCount != 2 || r.ErrorCount != 3 || r.EmptyCount != 1 || r.DuplicateCount != 1 || r.NotDirectoryCount != 1 {
		t.Fatalf("wrong overlapping counts: %#v", r)
	}
	if !r.Entries[4].Empty || !r.Entries[4].Directory || r.Entries[4].Value != "" || r.Entries[6].DuplicateOf != 3 || r.Entries[6].Missing || r.Entries[6].Error == "" {
		t.Fatalf("empty/duplicate evidence changed: %#v", r)
	}
}
