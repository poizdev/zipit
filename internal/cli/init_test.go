package cli

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/presets"
	"github.com/poizdev/zipit/internal/rules"
)

func TestInitCommandIsRegisteredAndAcceptsNoArguments(t *testing.T) {
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	initCommand, _, err := command.Find([]string{"init"})
	if err != nil {
		t.Fatal(err)
	}
	if initCommand == command || initCommand.Name() != "init" {
		t.Fatalf("Find(init) = %q", initCommand.Name())
	}
	if err := initCommand.Args(initCommand, []string{"workspace"}); err == nil {
		t.Fatal("init accepted a path argument")
	}
}

func TestRunWorkspaceInitWritesExpandedCanonicalRules(t *testing.T) {
	configPath := isolateConfigPath(t)
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".zipitignore")
	interaction := &fakeExclusionInteraction{
		selected: []presets.Preset{presetByID(t, "node"), presetByID(t, "os")},
		confirms: []bool{true, true, true, false, true},
		inputs:   []string{`.\cache\`, "*.log", "node_modules/"},
	}
	var output bytes.Buffer
	if err := runWorkspaceInit(&output, path, interaction); err != nil {
		t.Fatal(err)
	}
	want := "node_modules/\n.DS_Store\nThumbs.db\ncache/\n*.log\n"
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf(".zipitignore = %q, want %q", got, want)
	}
	if strings.Contains(string(got), "node\n") || !strings.Contains(output.String(), "Workspace exclusions") {
		t.Fatalf("content = %q, output = %q", got, output.String())
	}
	loaded, err := rules.LoadIgnoreFile(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rules.CompileRules(loaded); err != nil {
		t.Fatalf("generated ignore file does not compile: %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("workspace init created or changed global config: %v", err)
	}
}

func TestRunWorkspaceInitCanCreateEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zipitignore")
	interaction := &fakeExclusionInteraction{selected: nil, confirms: []bool{false, true}}
	if err := runWorkspaceInit(&bytes.Buffer{}, path, interaction); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty .zipitignore = %q", got)
	}
}

func TestWriteWorkspaceIgnoreRejectsInvalidRulesWithoutArtifacts(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".zipitignore")
	if err := writeWorkspaceIgnore(path, []string{"["}); err == nil {
		t.Fatal("invalid rule was written")
	}
	assertNoWorkspaceIgnoreArtifacts(t, workspace)
}

func TestInitRefusesExistingFileByteForByte(t *testing.T) {
	workspace := chdirTempWorkspace(t)
	path := filepath.Join(workspace, ".zipitignore")
	want := []byte("# preserve exactly\r\nnode_modules/\r\n")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"init"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), ".zipitignore already exists") || !strings.Contains(err.Error(), path) {
		t.Fatalf("init error = %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("existing file changed: got %q, want %q", got, want)
	}
	assertNoWorkspaceTempFiles(t, workspace)
}

func TestInitRefusesNonTTYWithoutCreatingFile(t *testing.T) {
	workspace := chdirTempWorkspace(t)
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetIn(strings.NewReader(""))
	command.SetArgs([]string{"init"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("init error = %v", err)
	}
	assertNoWorkspaceIgnoreArtifacts(t, workspace)
}

func TestRunWorkspaceInitCancellationLeavesNoArtifacts(t *testing.T) {
	workspace := t.TempDir()
	interaction := &fakeExclusionInteraction{selected: defaultPresets(), confirms: []bool{false, false}}
	if err := runWorkspaceInit(&bytes.Buffer{}, filepath.Join(workspace, ".zipitignore"), interaction); err != nil {
		t.Fatal(err)
	}
	assertNoWorkspaceIgnoreArtifacts(t, workspace)
}

func TestRunWorkspaceInitChooserCancellationLeavesNoArtifacts(t *testing.T) {
	workspace := t.TempDir()
	cancelled := errors.New("interactive form cancelled")
	interaction := &fakeExclusionInteraction{selectErr: cancelled}
	err := runWorkspaceInit(&bytes.Buffer{}, filepath.Join(workspace, ".zipitignore"), interaction)
	if !errors.Is(err, cancelled) {
		t.Fatalf("runWorkspaceInit() error = %v, want cancellation", err)
	}
	assertNoWorkspaceIgnoreArtifacts(t, workspace)
}

func TestWorkspacePublisherDoesNotReplaceExistingTarget(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".zipitignore")
	want := []byte("existing/\n")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeWorkspaceIgnore(path, []string{"replacement/"}); err == nil {
		t.Fatal("writer replaced existing .zipitignore")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("existing file changed: got %q, want %q", got, want)
	}
	assertNoWorkspaceTempFiles(t, workspace)
}

func TestWorkspacePublicationRacePreservesCompetingFile(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, ".zipitignore")
	want := []byte("competing/\n")
	err := writeWorkspaceIgnoreWithPublisher(path, []string{"ours/"}, func(_, destination string) error {
		if writeErr := os.WriteFile(destination, want, 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
		return fs.ErrExist
	})
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("write error = %v, want exists", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("race winner changed: got %q, want %q", got, want)
	}
	assertNoWorkspaceTempFiles(t, workspace)
}

func TestGeneratedWorkspaceIgnoreIsHonoredByArchiveCommand(t *testing.T) {
	isolateConfigPath(t)
	workspace := t.TempDir()
	for name, content := range map[string]string{
		"src/main.go":               "package main",
		"node_modules/pkg/index.js": "generated",
	} {
		path := filepath.Join(workspace, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeWorkspaceIgnore(filepath.Join(workspace, ".zipitignore"), []string{"node_modules/"}); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "workspace.zip")
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{workspace, "-o", output})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	entries := make(map[string]bool)
	for _, entry := range reader.File {
		entries[entry.Name] = true
	}
	if !entries["src/main.go"] || entries["node_modules/"] || entries["node_modules/pkg/index.js"] {
		t.Fatalf("archive entries = %v", entries)
	}
}

func chdirTempWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return workspace
}

func assertNoWorkspaceIgnoreArtifacts(t *testing.T, workspace string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(workspace, ".zipitignore")); !os.IsNotExist(err) {
		t.Fatalf(".zipitignore exists or stat failed: %v", err)
	}
	assertNoWorkspaceTempFiles(t, workspace)
}

func assertNoWorkspaceTempFiles(t *testing.T, workspace string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(workspace, ".zipitignore-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}

func TestWorkspaceIgnorePermissionsAreShareable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), ".zipitignore")
	if err := writeWorkspaceIgnore(path, []string{"*.log"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("permissions = %o, want 644", info.Mode().Perm())
	}
}
