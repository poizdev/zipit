package cli_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/cli"
)

// TestArchiveSucceedsWithoutUpdateState proves that absent update state does
// not affect archive creation when no interactive notification is possible.
func TestArchiveSucceedsWithoutUpdateState(t *testing.T) {
	isolateGlobalConfig(t)

	parent := t.TempDir()
	source := filepath.Join(parent, "project")
	output := filepath.Join(parent, "project.zip")

	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	command.SetArgs([]string{source, "-o", output})
	err := command.Execute()
	if err != nil {
		t.Fatalf("Archive creation failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify archive was created and is valid.
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("Archive not readable: %v", err)
	}
	defer reader.Close()

	if len(reader.File) != 1 || reader.File[0].Name != "hello.txt" {
		names := make([]string, len(reader.File))
		for i, f := range reader.File {
			names[i] = f.Name
		}
		t.Fatalf("unexpected archive contents: %v", names)
	}

	// Verify summary was printed to stdout.
	if !strings.Contains(stdout.String(), "1 file") {
		t.Fatalf("stdout missing summary: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Done in") {
		t.Fatalf("stdout missing Done: %q", stdout.String())
	}
}

// TestDryRunSucceedsWithoutUpdateState proves dry-run is also unaffected.
func TestDryRunSucceedsWithoutUpdateState(t *testing.T) {
	isolateGlobalConfig(t)

	parent := t.TempDir()
	source := filepath.Join(parent, "project")
	output := filepath.Join(parent, "project.zip")

	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	command.SetArgs([]string{source, "-o", output, "--dry-run"})
	err := command.Execute()
	if err != nil {
		t.Fatalf("Dry-run failed: %v\nstderr: %s", err, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Would include 1 file") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	// No archive should be created.
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("dry-run created output")
	}
}
