package resolver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/adhamcodes/whichwhy/internal/processpath"
)

func TestObservationStatClassification(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		status     ObservationStatus
		category   string
		incomplete bool
	}{
		{"missing", os.ErrNotExist, ObservedNotFound, "not-found", false},
		{"ENOENT", syscall.ENOENT, ObservedNotFound, "not-found", false},
		{"ENOTDIR", syscall.ENOTDIR, ObservedNotDirectory, "not-directory", false},
		{"EACCES", syscall.EACCES, ObservedError, "permission-denied", true},
		{"EPERM", syscall.EPERM, ObservedError, "permission-denied", true},
		{"unexpected", errors.New("controlled IO failure"), ObservedError, "other", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "ENOTDIR" && runtime.GOOS == "windows" {
				// Go aliases this to ERROR_PATH_NOT_FOUND on Windows.
				tc.status, tc.category = ObservedNotFound, "not-found"
			}
			failure := fmt.Errorf("wrapped: %w", &os.PathError{Op: "stat", Path: "entry/probe", Err: tc.err})
			r, err := collectCandidates([]processpath.Entry{{Index: 4, Value: "entry"}}, []string{"probe"},
				func(string) (os.FileInfo, error) { return nil, failure },
				func(os.FileInfo) (bool, error) { t.Fatal("eligibility on failed stat"); return false, nil }, filepath.Abs)
			if err != nil || len(r.Observations) != 1 || len(r.Candidates) != 0 {
				t.Fatalf("%#v %v", r, err)
			}
			o := r.Observations[0]
			if o.Attempt != 1 || o.PathIndex != 4 || o.Name != "probe" || o.Path != filepath.Join("entry", "probe") || o.Status != tc.status || o.Error.Category != tc.category || o.Error.Message != failure.Error() || o.Error.Operation != "stat" || o.Incomplete() != tc.incomplete {
				t.Fatalf("lost/classified incorrectly: %#v error=%#v", o, o.Error)
			}
		})
	}
}

func TestObservationRealFilesystemAndDuplicates(t *testing.T) {
	root := t.TempDir()
	name := "wwobservations"
	for _, d := range []string{"missing", "directory", "candidate"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "directory", testCommandFilename(name)), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestCommand(t, filepath.Join(root, "candidate"), name)
	entries := []string{filepath.Join(root, "missing"), filepath.Join(root, "directory"), filepath.Join(root, "candidate"), filepath.Join(root, "candidate")}
	r, err := resolveExternal(name, strings.Join(entries, string(os.PathListSeparator)), testPathExt())
	if err != nil || len(r.Observations) != 4 || len(r.Candidates) != 1 {
		t.Fatalf("%#v %v", r, err)
	}
	for i, status := range []ObservationStatus{ObservedNotFound, ObservedDirectory, ObservedCandidate, ObservedCandidate} {
		o := r.Observations[i]
		if o.Status != status || o.Attempt != i+1 || o.PathIndex != i+1 || o.Incomplete() || o.Path != filepath.Join(entries[i], testCommandFilename(name)) {
			t.Fatalf("attempt %d: %#v", i, o)
		}
	}
	if r.Candidates[0].DirectoryIndex != 2 {
		t.Fatalf("dedup lost first producer: %#v", r.Candidates)
	}
}

func TestObservationCollectorOrderAndFatalBoundary(t *testing.T) {
	dir := t.TempDir()
	writeTestCommand(t, dir, "probe")
	info, err := os.Stat(filepath.Join(dir, testCommandFilename("probe")))
	if err != nil {
		t.Fatal(err)
	}
	entries := []processpath.Entry{{Index: 2, Value: "first"}, {Index: 5, Value: "first"}}
	var calls []string
	stat := func(path string) (os.FileInfo, error) { calls = append(calls, path); return info, nil }
	r, err := collectCandidates(entries, []string{"a", "b"}, stat, func(os.FileInfo) (bool, error) { return true, nil }, filepath.Abs)
	want := []string{filepath.Join("first", "a"), filepath.Join("first", "b"), filepath.Join("first", "a"), filepath.Join("first", "b")}
	if err != nil || !reflect.DeepEqual(calls, want) || len(r.Candidates) != 4 || len(r.Observations) != 4 {
		t.Fatalf("%#v calls=%q err=%v", r, calls, err)
	}
	for i, o := range r.Observations {
		if o.Attempt != i+1 || o.PathIndex != entries[i/2].Index || o.Path != want[i] {
			t.Fatalf("order lost: %#v", o)
		}
	}
	failure := errors.New("ownership unavailable")
	r, err = collectCandidates(entries, []string{"a"}, stat, func(os.FileInfo) (bool, error) { return false, failure }, filepath.Abs)
	if !errors.Is(err, failure) || r.Candidates != nil || r.Observations != nil || !strings.Contains(err.Error(), fmt.Sprintf("%q", filepath.Join("first", "a"))) {
		t.Fatalf("fatal failure downgraded: %#v %v", r, err)
	}
	r, err = collectCandidates(entries[:1], []string{"a"}, stat, func(os.FileInfo) (bool, error) { return false, nil }, filepath.Abs)
	if err != nil || r.Observations[0].Status != ObservedIneligible || r.Observations[0].Incomplete() || r.Observations[0].Error != nil {
		t.Fatalf("mode rejection became stat error: %#v %v", r, err)
	}
	r, err = collectCandidates(entries[:1], []string{"a"}, stat, func(os.FileInfo) (bool, error) { return true, nil }, func(string) (string, error) { return "", failure })
	if err != nil || r.Candidates[0].Path != want[0] || !r.Observations[0].Incomplete() || r.Observations[0].Status != ObservedCandidate || r.Observations[0].Error.Operation != "absolute-path" {
		t.Fatalf("absolute failure lost: %#v %v", r, err)
	}
}
