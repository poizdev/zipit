package archive_test

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	ziparchive "github.com/poizdev/zipit/internal/archive"
)

func TestResolveRequestDefaultsOutputNextToSource(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "fixture")
	mustMkdir(t, source)

	req, err := ziparchive.ResolveRequest(source, "")
	if err != nil {
		t.Fatalf("ResolveRequest() error = %v", err)
	}

	wantSource, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	gotSource, err := os.Stat(req.Source)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotSource, wantSource) {
		t.Fatalf("Source = %q, want same filesystem directory as %q", req.Source, source)
	}
	if want := req.Source + ".zip"; req.Output != want {
		t.Fatalf("Output = %q, want %q", req.Output, want)
	}
}

func TestResolveRequestUsesCustomOutput(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "fixture")
	output := filepath.Join(parent, "custom.zip")
	mustMkdir(t, source)

	req, err := ziparchive.ResolveRequest(source, output)
	if err != nil {
		t.Fatalf("ResolveRequest() error = %v", err)
	}
	want, err := filepath.Abs(output)
	if err != nil {
		t.Fatal(err)
	}
	gotParent, err := os.Stat(filepath.Dir(req.Output))
	if err != nil {
		t.Fatal(err)
	}
	wantParent, err := os.Stat(filepath.Dir(want))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(req.Output) != filepath.Base(want) || !os.SameFile(gotParent, wantParent) {
		t.Fatalf("Output = %q, want basename and parent identity of %q", req.Output, want)
	}
}

func TestResolveRequestRejectsInvalidSources(t *testing.T) {
	parent := t.TempDir()
	file := filepath.Join(parent, "file.txt")
	mustWriteFile(t, file, "not a directory")

	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "missing", source: filepath.Join(parent, "missing"), want: "source"},
		{name: "regular file", source: file, want: "not a directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ziparchive.ResolveRequest(test.source, "")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ResolveRequest() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCreatePreservesTreeContentsAndSafeNames(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "fixture")
	mustMkdir(t, filepath.Join(source, "nested"))
	mustMkdir(t, filepath.Join(source, "empty"))
	mustWriteFile(t, filepath.Join(source, "a.txt"), "alpha")
	mustWriteFile(t, filepath.Join(source, "nested", "b.txt"), "bravo")
	output := filepath.Join(parent, "archive.zip")

	if err := ziparchive.Create(ziparchive.Request{Source: source, Output: output}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	want := map[string]string{
		"a.txt":        "alpha",
		"nested/":      "",
		"nested/b.txt": "bravo",
		"empty/":       "",
	}
	got := make(map[string]string)
	for _, entry := range reader.File {
		if filepath.IsAbs(entry.Name) || strings.Contains(entry.Name, `\`) {
			t.Errorf("unsafe archive entry name %q", entry.Name)
		}
		if strings.Contains(entry.Name, filepath.ToSlash(source)) {
			t.Errorf("archive entry leaks source path: %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			got[entry.Name] = ""
			continue
		}
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		if err != nil {
			r.Close()
			t.Fatal(err)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		got[entry.Name] = string(data)
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("entry %q content = %q, want %q", name, got[name], content)
		}
	}
	if len(got) != len(want) {
		t.Errorf("archive entries = %v, want exactly %v", got, want)
	}
}

func TestCreateDoesNotOverwriteExistingOutput(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "fixture")
	output := filepath.Join(parent, "archive.zip")
	mustMkdir(t, source)
	mustWriteFile(t, filepath.Join(source, "a.txt"), "alpha")
	mustWriteFile(t, output, "keep me")

	err := ziparchive.Create(ziparchive.Request{Source: source, Output: output})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Create() error = %v, want existing-output error", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep me" {
		t.Fatalf("existing output was changed to %q", data)
	}
}

func TestCreateFailureLeavesNoOutputOrTemporaryFile(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "missing")
	output := filepath.Join(parent, "archive.zip")

	if err := ziparchive.Create(ziparchive.Request{Source: source, Output: output}); err == nil {
		t.Fatal("Create() error = nil, want failure")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("final output exists or stat failed: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(parent, ".zipit-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}

func TestCreatePreservesSymlinkWithoutDereferencing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks commonly requires additional Windows privileges")
	}
	parent := t.TempDir()
	source := filepath.Join(parent, "fixture")
	mustMkdir(t, source)
	mustWriteFile(t, filepath.Join(source, "target.txt"), "target")
	if err := os.Symlink("target.txt", filepath.Join(source, "link.txt")); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(parent, "archive.zip")

	if err := ziparchive.Create(ziparchive.Request{Source: source, Output: output}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	found := false
	for _, entry := range reader.File {
		if entry.Name == "link.txt" {
			found = true
			if entry.Mode()&os.ModeSymlink == 0 {
				t.Fatal("symlink entry was stored as a regular file")
			}
		}
	}
	if !found {
		t.Fatal("symlink entry was not preserved")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
