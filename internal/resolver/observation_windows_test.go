package resolver

import (
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func TestWindowsObservationFailuresAndNameOrder(t *testing.T) {
	dir := t.TempDir()
	writeTestCommand(t, dir, "probe")
	info, err := os.Stat(filepath.Join(dir, "probe.EXE"))
	if err != nil {
		t.Fatal(err)
	}
	entries := processpath.Parse(`missing;"quoted;entry";last;last`).Entries
	var operands []string
	r, err := findWindowsCandidates("probe.v1", entries, ".EXE;.CMD;.cmd", func(path string) (os.FileInfo, error) {
		operands = append(operands, path)
		switch len(operands) {
		case 1:
			return nil, &os.PathError{Op: "stat", Path: path, Err: syscall.ERROR_ACCESS_DENIED}
		case 2:
			return info, nil
		case 3:
			return nil, &os.PathError{Op: "stat", Path: path, Err: syscall.Errno(53)} // ERROR_BAD_NETPATH
		default:
			return nil, os.ErrNotExist
		}
	})
	var want []string
	for _, e := range entries {
		for _, name := range []string{"probe.v1", "probe.v1.EXE", "probe.v1.CMD"} {
			want = append(want, filepath.Join(e.Value, name))
		}
	}
	if err != nil || !reflect.DeepEqual(operands, want) || len(r.Observations) != 12 || len(r.Candidates) != 1 {
		t.Fatalf("%#v calls=%q err=%v", r, operands, err)
	}
	for i, o := range r.Observations {
		if o.Attempt != i+1 || o.PathIndex != i/3+1 || o.Path != want[i] {
			t.Fatalf("%#v", o)
		}
	}
	if r.Observations[0].Error.Category != "permission-denied" || r.Observations[1].Status != ObservedCandidate || r.Observations[2].Status != ObservedError {
		t.Fatalf("%#v", r.Observations)
	}
}

func TestWindowsObservationErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		code   syscall.Errno
		status ObservationStatus
	}{
		{syscall.ERROR_FILE_NOT_FOUND, ObservedNotFound}, {syscall.ERROR_PATH_NOT_FOUND, ObservedNotFound},
		{syscall.ERROR_ACCESS_DENIED, ObservedError}, {syscall.Errno(53), ObservedError},
		{syscall.Errno(267), ObservedError}, // ERROR_DIRECTORY: invalid directory name, not proof of ENOTDIR
		{syscall.Errno(123), ObservedError}, // ERROR_INVALID_NAME
		{syscall.Errno(32), ObservedError},  // sharing violation
	} {
		s, e := classifyStatError(&os.PathError{Op: "stat", Path: "fixture", Err: tc.code})
		if s != tc.status || e.Message == "" {
			t.Fatalf("%d: %s %#v", tc.code, s, e)
		}
	}
}
