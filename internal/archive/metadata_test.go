package archive_test

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	ziparchive "github.com/poizdev/zipit/internal/archive"
)

func TestPortableMetadataAndOddPaths(t *testing.T) {
	source := t.TempDir()
	deep := filepath.Join(source, "space dir", "深い", ".hidden")
	mustMkdir(t, filepath.Join(deep, "empty-dir"))
	mustWriteFile(t, filepath.Join(deep, "empty file.txt"), "")
	executable := filepath.Join(source, "run.sh")
	mustWriteFile(t, executable, "#!/bin/sh\n")
	modified := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(executable, modified, modified); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(executable, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(t.TempDir(), "archive.zip")
	if _, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output}); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	want := map[string]bool{
		"space dir/深い/.hidden/empty-dir/":     false,
		"space dir/深い/.hidden/empty file.txt": false,
		"run.sh": false,
	}
	for _, entry := range reader.File {
		if _, ok := want[entry.Name]; ok {
			want[entry.Name] = true
		}
		if entry.Name == "run.sh" {
			if runtime.GOOS != "windows" && entry.Mode().Perm()&0o111 == 0 {
				t.Fatal("executable mode was not preserved")
			}
			if entry.Modified.Unix() != modified.Unix() {
				t.Fatalf("mtime = %v, want %v", entry.Modified, modified)
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("archive missing %q", name)
		}
	}
}
