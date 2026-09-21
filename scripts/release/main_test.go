package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseTags(t *testing.T) {
	for _, tag := range []string{"v1.0.0", "v1.0.0-rc.1", "v0.0.0", "v12.23.34-alpha-beta.0.a01", "v1.0.0+build.001", "v1.0.0-rc.1+build.7"} {
		if err := validateTag(tag); err != nil {
			t.Errorf("valid %q: %v", tag, err)
		}
	}
	for _, tag := range []string{"", "dev", "1.0.0", "v1", "v1.0", "v01.0.0", "v1.00.0", "v1.0.01", "v1.0.0-01", "v1.0.0-rc.01", "v1.0.0-", "v1.0.0+", "v1.0.0-rc..1", "v1.0.0+a..b", "v1.0.0+foo+bar", "v1.0.0_rc.1", "v1.0.0\n", " v1.0.0", "v1.0.0;echo bad", "v1.0.0/../bad", "v1.0.0-β"} {
		if validateTag(tag) == nil {
			t.Errorf("accepted malformed %q", tag)
		}
		if run([]string{"build", tag, "windows", "amd64", t.TempDir()}) == nil {
			t.Errorf("build accepted malformed %q", tag)
		}
	}
}

func readArchive(t *testing.T, platform target, data []byte) []member {
	t.Helper()
	var result []member
	if platform.os == "windows" {
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range z.File {
			r, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			b, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			result = append(result, member{f.Name, int64(f.Mode().Perm()), b})
		}
	} else {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer gz.Close()
		r := tar.NewReader(gz)
		for {
			h, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if h.Typeflag != tar.TypeReg {
				t.Fatalf("non-regular member: %+v", h)
			}
			b, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			result = append(result, member{h.Name, h.Mode, b})
		}
	}
	return result
}

func TestArchives(t *testing.T) {
	for _, platform := range targets {
		t.Run(platform.os+"_"+platform.arch, func(t *testing.T) {
			want := []member{{platform.binary(), 0755, []byte("binary")}, {"LICENSE", 0644, []byte("license")}, {"README.md", 0644, []byte("usage")}}
			data, err := archiveBytes(platform, want)
			if err != nil {
				t.Fatal(err)
			}
			again, err := archiveBytes(platform, want)
			if err != nil || !bytes.Equal(data, again) {
				t.Fatalf("archive not deterministic: %v", err)
			}
			got := readArchive(t, platform, data)
			if len(got) != len(want) {
				t.Fatalf("members: %v", got)
			}
			for i := range want {
				if got[i].name != want[i].name || got[i].mode != want[i].mode || !bytes.Equal(got[i].data, want[i].data) {
					t.Fatalf("member %d: %+v", i, got[i])
				}
			}
		})
	}
}

func TestCompleteChecksums(t *testing.T) {
	for _, tag := range []string{"v1.0.0", "v1.0.0-rc.1"} {
		t.Run(tag, func(t *testing.T) {
			dir := t.TempDir()
			version := strings.TrimPrefix(tag, "v")
			names := []string{"whichwhy_" + version + "_darwin_amd64.tar.gz", "whichwhy_" + version + "_darwin_arm64.tar.gz", "whichwhy_" + version + "_linux_amd64.tar.gz", "whichwhy_" + version + "_windows_amd64.zip"}
			var want strings.Builder
			for i, name := range names {
				data := []byte(fmt.Sprintf("archive fixture %d", i))
				if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
					t.Fatal(err)
				}
				fmt.Fprintf(&want, "%x  %s\n", sha256.Sum256(data), name)
			}
			manifest, err := checksumManifest(tag, dir)
			if err != nil || string(manifest) != want.String() {
				t.Fatalf("manifest %q, %v", manifest, err)
			}
			if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
				t.Fatal(err)
			}
			if _, err := checksumManifest(tag, dir); err == nil {
				t.Fatal("accepted missing target")
			}
			if err := os.WriteFile(filepath.Join(dir, "wrong-version.tar.gz"), []byte("x"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := checksumManifest(tag, dir); err == nil {
				t.Fatal("accepted wrong asset name")
			}
			if err := os.Remove(filepath.Join(dir, "wrong-version.tar.gz")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, names[0]), nil, 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := checksumManifest(tag, dir); err == nil {
				t.Fatal("accepted empty archive")
			}
			if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, names[0]), 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := checksumManifest(tag, dir); err == nil {
				t.Fatal("accepted directory as archive")
			}
		})
	}
}

func TestNativeReleasePackage(t *testing.T) {
	platform := target{runtime.GOOS, runtime.GOARCH}
	supported := false
	for _, target := range targets {
		supported = supported || target == platform
	}
	if !supported {
		t.Skip("host is not a release target")
	}
	t.Chdir(filepath.Join("..", ".."))
	for _, tag := range []string{"v1.0.0-rc.1", "v1.0.0"} {
		dir := t.TempDir()
		if err := build(tag, platform, dir); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, platform.archive(tag)))
		if err != nil {
			t.Fatal(err)
		}
		members := readArchive(t, platform, data)
		if len(members) != 3 {
			t.Fatalf("expected binary, LICENSE and usage, got %d", len(members))
		}
		license, err := os.ReadFile("LICENSE")
		if err != nil {
			t.Fatal(err)
		}
		if members[1].name != "LICENSE" || !bytes.Equal(members[1].data, license) {
			t.Fatal("missing or incorrect LICENSE")
		}
		if members[2].name != "README.md" || !bytes.Contains(members[2].data, []byte("does not install itself")) {
			t.Fatal("missing usage")
		}
		info, err := buildinfo.Read(bytes.NewReader(members[0].data))
		if err != nil {
			t.Fatal(err)
		}
		settings := make(map[string]string)
		for _, s := range info.Settings {
			settings[s.Key] = s.Value
		}
		if settings["GOOS"] != platform.os || settings["GOARCH"] != platform.arch || settings["-trimpath"] != "true" || settings["CGO_ENABLED"] != "0" {
			t.Fatalf("incorrect build settings: %+v", info.Settings)
		}
		binary := filepath.Join(dir, platform.binary())
		if err := os.WriteFile(binary, members[0].data, 0755); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(binary, "--version").CombinedOutput()
		if err != nil || string(out) != "whichwhy "+tag+"\n" {
			t.Fatalf("extracted version %q: %v", out, err)
		}
	}
}

func TestRefuseOverwriteAndUnknownTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset")
	if err := writeNew(path, []byte("original")); err != nil {
		t.Fatal(err)
	}
	if err := writeNew(path, []byte("replacement")); err == nil {
		t.Fatal("overwrote asset")
	}
	if err := run([]string{"build", "v1.0.0", "windows", "arm64", t.TempDir()}); err == nil {
		t.Fatal("accepted unsupported target")
	}
}

func TestSourceIdentityFailsClosed(t *testing.T) {
	const commit = "0123456789012345678901234567890123456789"
	for _, tc := range []struct {
		name, head, tag, status, fail string
		ok                            bool
	}{
		{name: "clean exact commit", head: commit, tag: commit, ok: true},
		{name: "wrong HEAD", head: strings.Repeat("a", 40), tag: commit},
		{name: "moved tag", head: commit, tag: strings.Repeat("a", 40)},
		{name: "dirty source", head: commit, tag: commit, status: " M README.md"},
		{name: "untracked source", head: commit, tag: commit, status: "?? injected.go"},
		{name: "missing tag", head: commit, fail: "refs/tags/v1.0.0-rc.1^{commit}"},
		{name: "status failure", head: commit, tag: commit, fail: "status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Fake Git observations, never create or move real tags in tests.
			git := func(args ...string) (string, error) {
				key := args[0]
				if key == "rev-parse" {
					key = args[2]
				}
				if key == tc.fail {
					return "", fmt.Errorf("Git observation failed")
				}
				switch key {
				case "HEAD":
					return tc.head, nil
				case "refs/tags/v1.0.0-rc.1^{commit}":
					return tc.tag, nil
				case "status":
					return tc.status, nil
				default:
					t.Fatalf("unexpected Git call: %v", args)
					return "", nil
				}
			}
			if err := checkSource("v1.0.0-rc.1", commit, git); (err == nil) != tc.ok {
				t.Fatalf("source check: %v", err)
			}
		})
	}
}
