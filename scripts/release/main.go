// Release-only tooling. Run from the repository root; uses only Go and Git.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// SemVer 2.0.0, with a mandatory v prefix. Numeric prerelease identifiers
// cannot have leading zeroes; build metadata may. No whitespace is accepted.
var tagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?(\+([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?$`)

type target struct{ os, arch string }

var targets = []target{{"windows", "amd64"}, {"linux", "amd64"}, {"darwin", "amd64"}, {"darwin", "arm64"}}

func validateTag(tag string) error {
	if !tagPattern.MatchString(tag) {
		return fmt.Errorf("invalid release tag %q: expected v-prefixed SemVer", tag)
	}
	version := strings.SplitN(tag, "+", 2)[0]
	if _, pre, ok := strings.Cut(version, "-"); ok {
		for _, id := range strings.Split(pre, ".") {
			if len(id) > 1 && id[0] == '0' && strings.Trim(id, "0123456789") == "" {
				return fmt.Errorf("numeric prerelease identifier has a leading zero: %q", id)
			}
		}
	}
	return nil
}

func (t target) binary() string {
	if t.os == "windows" {
		return "whichwhy.exe"
	}
	return "whichwhy"
}

func (t target) archive(tag string) string {
	ext := ".tar.gz"
	if t.os == "windows" {
		ext = ".zip"
	}
	return "whichwhy_" + strings.TrimPrefix(tag, "v") + "_" + t.os + "_" + t.arch + ext
}

func gitOutput(args ...string) (string, error) {
	b, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, b)
	}
	return strings.TrimSpace(string(b)), nil
}

func checkSource(tag, commit string, git func(...string) (string, error)) error {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit) {
		return fmt.Errorf("expected a full commit SHA")
	}
	for _, ref := range []string{"HEAD", "refs/tags/" + tag + "^{commit}"} {
		got, err := git("rev-parse", "--verify", ref)
		if err != nil {
			return err
		}
		if got != commit {
			return fmt.Errorf("%s is %s, expected %s", ref, got, commit)
		}
	}
	status, err := git("status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("release requires a clean checkout: %s", status)
	}
	return nil
}

type member struct {
	name string
	mode int64
	data []byte
}

func archiveBytes(t target, members []member) ([]byte, error) {
	var buf bytes.Buffer
	// Stable member order, modes and timestamps; no host paths or ownership.
	stamp := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	if t.os == "windows" {
		w := zip.NewWriter(&buf)
		for _, m := range members {
			h := &zip.FileHeader{Name: m.name, Method: zip.Deflate, Modified: stamp}
			h.SetMode(os.FileMode(m.mode))
			f, err := w.CreateHeader(h)
			if err != nil {
				return nil, err
			}
			if _, err := f.Write(m.data); err != nil {
				return nil, err
			}
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
	} else {
		gz := gzip.NewWriter(&buf)
		w := tar.NewWriter(gz)
		for _, m := range members {
			if err := w.WriteHeader(&tar.Header{Name: m.name, Mode: m.mode, Size: int64(len(m.data)), ModTime: stamp, Typeflag: tar.TypeReg}); err != nil {
				return nil, err
			}
			if _, err := w.Write(m.data); err != nil {
				return nil, err
			}
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		if err := gz.Close(); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func build(tag string, t target, dir string) error {
	known := false
	for _, supported := range targets {
		known = known || t == supported
	}
	if !known {
		return fmt.Errorf("unsupported release target %s/%s", t.os, t.arch)
	}
	stage, err := os.MkdirTemp("", "whichwhy-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	binary := filepath.Join(stage, t.binary())
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=true", "-ldflags=-X main.version="+tag, "-o", binary, "./cmd/whichwhy")
	// Override inherited cross-compilation/build flags. CI uses a fresh checkout
	// and setup-go with caching disabled; the package has no external dependencies.
	cmd.Env = append(os.Environ(), "GOOS="+t.os, "GOARCH="+t.arch, "CGO_ENABLED=0", "GOFLAGS=", "GOAMD64=v1", "GOARM64=v8.0", "GOWORK=off")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	if t.os == runtime.GOOS && t.arch == runtime.GOARCH {
		out, err := exec.Command(binary, "--version").CombinedOutput()
		if err != nil || string(out) != "whichwhy "+tag+"\n" {
			return fmt.Errorf("native version check: output %q, error %v", out, err)
		}
		fmt.Printf("Native --version verified: %s/%s %s\n", t.os, t.arch, tag)
	} else {
		fmt.Printf("Cross-compiled %s/%s: NOT runtime-tested on %s/%s\n", t.os, t.arch, runtime.GOOS, runtime.GOARCH)
	}
	var members []member
	for _, spec := range []struct {
		name, path string
		mode       int64
	}{{t.binary(), binary, 0755}, {"LICENSE", "LICENSE", 0644}, {"README.md", "docs/release-usage.md", 0644}} {
		data, err := os.ReadFile(spec.path)
		if err != nil {
			return err
		}
		if len(data) == 0 {
			return fmt.Errorf("empty archive member: %s", spec.path)
		}
		members = append(members, member{spec.name, spec.mode, data})
	}
	data, err := archiveBytes(t, members)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return writeNew(filepath.Join(dir, t.archive(tag)), data)
}

func checksumManifest(tag, dir string) ([]byte, error) {
	expected := make(map[string]bool)
	for _, t := range targets {
		expected[t.archive(tag)] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	if len(entries) != len(expected) {
		return nil, fmt.Errorf("expected exactly %d archives, found %d entries", len(expected), len(entries))
	}
	var names []string
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		if !expected[e.Name()] || !info.Mode().IsRegular() || info.Size() == 0 {
			return nil, fmt.Errorf("unexpected, empty or non-regular asset: %s", e.Name())
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		h := sha256.New()
		_, readErr := io.Copy(h, f)
		closeErr := f.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		fmt.Fprintf(&manifest, "%x  %s\n", h.Sum(nil), name)
	}
	return []byte(manifest.String()), nil
}

func run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: release validate TAG | source TAG COMMIT | build TAG OS ARCH DIR | checksums TAG DIR")
	}
	if err := validateTag(args[1]); err != nil {
		return err
	}
	switch {
	case args[0] == "validate" && len(args) == 2:
		return nil
	case args[0] == "source" && len(args) == 3:
		return checkSource(args[1], args[2], gitOutput)
	case args[0] == "build" && len(args) == 5:
		return build(args[1], target{args[2], args[3]}, args[4])
	case args[0] == "checksums" && len(args) == 3:
		data, err := checksumManifest(args[1], args[2])
		if err != nil {
			return err
		}
		return writeNew(filepath.Join(args[2], "SHA256SUMS.txt"), data)
	default:
		return fmt.Errorf("unknown release command or argument count")
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}
