//go:build !windows

package archive_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ziparchive "github.com/poizdev/zipit/internal/archive"
)

type mutatingMatcher struct {
	mutate func(path string, isDir bool)
}

func (m mutatingMatcher) Match(path string, isDir bool) bool {
	m.mutate(path, isDir)
	return false
}

func TestRegularFileReplacedBySymlinkFailsWithoutPublishing(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	target := filepath.Join(source, "selected.txt")
	mustMkdir(t, source)
	mustWriteFile(t, target, "inside")
	mustWriteFile(t, filepath.Join(parent, "outside.txt"), "outside secret")
	mustWriteFile(t, output, "old archive")

	matcher := mutatingMatcher{mutate: func(path string, isDir bool) {
		if path == "selected.txt" && !isDir {
			_ = os.Remove(target)
			_ = os.Symlink(filepath.Join(parent, "outside.txt"), target)
		}
	}}
	_, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output, Force: true, Matcher: matcher})
	if err == nil || !strings.Contains(err.Error(), target) {
		t.Fatalf("Run() error = %v, want changed file path", err)
	}
	assertOldOutputAndNoTemps(t, output, parent)
}

func TestRegularFileReplacedByFIFOFailsWithoutBlockingOrPublishing(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	target := filepath.Join(source, "selected.txt")
	mustMkdir(t, source)
	mustWriteFile(t, target, "inside")
	mustWriteFile(t, output, "old archive")

	matcher := mutatingMatcher{mutate: func(path string, isDir bool) {
		if path == "selected.txt" && !isDir {
			_ = os.Remove(target)
			_ = makeFIFO(target)
		}
	}}
	_, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output, Force: true, Matcher: matcher})
	if err == nil || !strings.Contains(err.Error(), target) {
		t.Fatalf("Run() error = %v, want changed file path", err)
	}
	assertOldOutputAndNoTemps(t, output, parent)
}

func TestDirectoryReplacedBySymlinkCannotEscapeTraversal(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	directory := filepath.Join(source, "changing")
	outside := filepath.Join(parent, "outside")
	mustMkdir(t, directory)
	mustMkdir(t, outside)
	mustWriteFile(t, filepath.Join(outside, "secret.txt"), "outside secret")
	mustWriteFile(t, output, "old archive")

	matcher := mutatingMatcher{mutate: func(path string, isDir bool) {
		if path == "changing" && isDir {
			_ = os.Remove(directory)
			_ = os.Symlink(outside, directory)
		}
	}}
	_, err := ziparchive.Run(context.Background(), ziparchive.Request{Source: source, Output: output, Force: true, Matcher: matcher})
	if err == nil || !strings.Contains(err.Error(), directory) {
		t.Fatalf("Run() error = %v, want changed directory path", err)
	}
	assertOldOutputAndNoTemps(t, output, parent)
}

func assertOldOutputAndNoTemps(t *testing.T, output, tempDirectory string) {
	t.Helper()
	got, err := os.ReadFile(output)
	if err != nil || string(got) != "old archive" {
		t.Fatalf("existing output = %q, %v", got, err)
	}
	assertNoArchiveTemps(t, tempDirectory)
}
