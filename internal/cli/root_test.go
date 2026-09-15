package cli_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/cli"
	"github.com/poizdev/zipit/internal/config"
)

func TestCommandDefaultsToCurrentDirectory(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "project")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(source); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %q", err, stderr.String())
	}

	output := filepath.Join(parent, "project.zip")
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("default archive %q was not created: %v", output, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1 file", "5 B input", "archive (project.zip)", "Done in"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want containing %q", stdout.String(), want)
		}
	}
}

func TestCommandDryRunComposesAllRuleSourcesWithoutWriting(t *testing.T) {
	configPath := isolateGlobalConfig(t)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("schema = 1\n[ignore]\npatterns = [\"node_modules/\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "workspace.zip")
	files := map[string]string{
		".zipitignore":              "recordings/\n",
		"src/main.go":               "12345",
		"node_modules/pkg/index.js": "global",
		"recordings/demo.bin":       "workspace",
		"debug.log":                 "cli",
	}
	for name, contents := range files {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout bytes.Buffer
	command := cli.NewRootCommand(&stdout, &bytes.Buffer{})
	command.SetArgs([]string{source, "-o", output, "-x", "*.log", "--dry-run"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Would include 2 files",
		"Skipped files: 1",
		"Skipped directories: 2",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout %q missing %q", stdout.String(), want)
		}
	}
	_, destination, found := strings.Cut(stdout.String(), "Would create:\n")
	if !found {
		t.Fatalf("stdout %q missing reported destination", stdout.String())
	}
	reported, _, found := strings.Cut(destination, "\n")
	if !found {
		t.Fatalf("stdout %q has unterminated reported destination", stdout.String())
	}
	if got, want := filepath.Base(reported), "workspace.zip"; got != want {
		t.Fatalf("reported destination basename = %q, want %q", got, want)
	}
	reportedParent, err := os.Stat(filepath.Dir(reported))
	if err != nil {
		t.Fatalf("stat reported destination parent: %v", err)
	}
	expectedParent, err := os.Stat(filepath.Dir(output))
	if err != nil {
		t.Fatalf("stat expected destination parent: %v", err)
	}
	if !os.SameFile(reportedParent, expectedParent) {
		t.Fatalf("reported destination parent %q and expected parent %q differ", filepath.Dir(reported), filepath.Dir(output))
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("dry-run output exists or stat failed: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(parent, ".zipit-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("dry-run temps = %v, %v", matches, err)
	}
}

func TestCommandDryRunAllowsExistingOutputAndReportsIt(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	command := cli.NewRootCommand(&stdout, &bytes.Buffer{})
	command.SetArgs([]string{source, "-o", output, "--dry-run", "--force"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Output already exists and would be replaced") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != "existing" {
		t.Fatalf("existing output = %q, %v", got, err)
	}
}

func TestCommandForceReplacesOutputAndPrintsMeasuredSummary(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	command := cli.NewRootCommand(&stdout, &bytes.Buffer{})
	command.SetArgs([]string{source, "-o", output, "--force"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1 file", "input", "archive", "Done in"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout %q missing %q", stdout.String(), want)
		}
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatalf("forced output is not a ZIP: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close forced output: %v", err)
	}
	if err := os.Remove(output); err != nil {
		t.Fatalf("remove forced output immediately after command: %v", err)
	}
}

func TestCommandReturnsCleanInterruptedError(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "archive.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	command := cli.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetContext(ctx)
	command.SetArgs([]string{source, "-o", output})
	err := command.Execute()
	if err == nil || err.Error() != "archive interrupted" {
		t.Fatalf("Execute() error = %v, want clean interrupted status", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("interrupted output exists or stat failed: %v", err)
	}
}

func TestCommandAcceptsCustomOutput(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "project")
	output := filepath.Join(parent, "custom.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	command.SetArgs([]string{source, "--output", output})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %q", err, stderr.String())
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("custom output was not created: %v", err)
	}
}

func TestCommandRejectsTooManyPaths(t *testing.T) {
	isolateGlobalConfig(t)
	command := cli.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"one", "two"})
	if err := command.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want argument validation error")
	}
}

func TestCommandHelpAndVersion(t *testing.T) {
	isolateGlobalConfig(t)
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "help", args: []string{"--help"}, want: "zipit [PATH]"},
		{name: "version", args: []string{"--version"}, want: "dev"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			command := cli.NewRootCommand(&stdout, &stderr)
			command.SetArgs(test.args)
			if err := command.Execute(); err != nil {
				t.Fatalf("Execute() error = %v, stderr = %q", err, stderr.String())
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("stdout = %q, want containing %q", stdout.String(), test.want)
			}
		})
	}
}

func TestCommandAcceptsRepeatableExclusions(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "project")
	output := filepath.Join(parent, "project.zip")
	for path, content := range map[string]string{
		"main.go":                   "package main",
		"debug.log":                 "debug",
		"node_modules/pkg/index.js": "generated",
	} {
		fullPath := filepath.Join(source, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	command.SetArgs([]string{source, "-o", output, "-x", "node_modules/", "--exclude", "*.log"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr = %q", err, stderr.String())
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != 1 || reader.File[0].Name != "main.go" {
		t.Fatalf("archive entries = %v, want only main.go", reader.File)
	}
}

func TestCommandRejectsInvalidExclusionBeforeCreatingOutput(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "project")
	output := filepath.Join(parent, "project.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}

	command := cli.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{source, "-o", output, "-x", "["})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid exclusion rule") {
		t.Fatalf("Execute() error = %v, want invalid-rule error", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("final output exists or stat failed: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(parent, ".zipit-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary outputs left behind: %v", matches)
	}
}

func TestCommandLoadsRootIgnoreFileAndComposesCLIExclusions(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "workspace.zip")
	files := map[string]string{
		".zipitignore":              "# workspace rules\n\nnode_modules/\nrecordings/\n*.log\n**/.cache/**\n",
		"src/main.go":               "package main",
		"node_modules/pkg/index.js": "generated",
		"recordings/demo.txt":       "recording",
		"logs/debug.log":            "debug",
		"nested/.cache/data.bin":    "cache",
		"videos/demo.mp4":           "video",
		"README.md":                 "readme",
		"nested/.zipitignore":       "*.secret\n",
		"nested/keep.secret":        "nested ignore must not load",
	}
	for path, content := range files {
		fullPath := filepath.Join(source, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	command := cli.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{source, "-o", output, "-x", "*.mp4"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got := make(map[string]bool)
	for _, entry := range reader.File {
		got[entry.Name] = true
	}
	for _, included := range []string{".zipitignore", "src/main.go", "README.md", "nested/.zipitignore", "nested/keep.secret"} {
		if !got[included] {
			t.Errorf("archive missing included entry %q; entries = %v", included, got)
		}
	}
	for _, excluded := range []string{"node_modules/", "recordings/", "logs/debug.log", "nested/.cache/", "videos/demo.mp4"} {
		if got[excluded] {
			t.Errorf("archive contains excluded entry %q", excluded)
		}
	}
}

func TestCommandInvalidIgnoreRuleLeavesNoOutput(t *testing.T) {
	isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "workspace.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".zipitignore"), []byte("# comment\n*.log\n[\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	command := cli.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{source, "-o", output})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), ".zipitignore:3") {
		t.Fatalf("Execute() error = %v, want .zipitignore line context", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("final output exists or stat failed: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(parent, ".zipit-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary outputs left behind: %v", matches)
	}
}

func TestCommandMissingGlobalConfigDoesNotCreateIt(t *testing.T) {
	configPath := isolateGlobalConfig(t)
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "workspace.zip")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}

	command := cli.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{source, "-o", output})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(configPath)); !os.IsNotExist(err) {
		t.Fatalf("archive read created global config directory or stat failed: %v", err)
	}
}

func TestCommandComposesGlobalWorkspaceAndCLIExclusions(t *testing.T) {
	configPath := isolateGlobalConfig(t)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("schema = 1\n[ignore]\npatterns = [\"node_modules/\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	source := filepath.Join(parent, "workspace")
	output := filepath.Join(parent, "workspace.zip")
	files := map[string]string{
		".zipitignore":              "recordings/\n",
		"node_modules/pkg/index.js": "generated",
		"recordings/demo.txt":       "recording",
		"videos/demo.mp4":           "video",
		"src/main.go":               "package main",
	}
	for name, contents := range files {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	command := cli.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{source, "-o", output, "-x", "*.mp4"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, entry := range reader.File {
		if strings.HasPrefix(entry.Name, "node_modules/") || strings.HasPrefix(entry.Name, "recordings/") || strings.HasSuffix(entry.Name, ".mp4") {
			t.Fatalf("archive contains excluded entry %q", entry.Name)
		}
	}
}

func TestCommandRejectsInvalidGlobalConfigBeforeArchiveCreation(t *testing.T) {
	for _, test := range []struct {
		name     string
		contents string
		want     string
	}{
		{name: "malformed TOML", contents: "schema = [", want: "parse"},
		{name: "unsupported schema", contents: "schema = 2\n", want: "unsupported schema 2"},
		{name: "invalid rule", contents: "schema = 1\n[ignore]\npatterns = [\"[\"]\n", want: "config.toml"},
	} {
		t.Run(test.name, func(t *testing.T) {
			configPath := isolateGlobalConfig(t)
			if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configPath, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			parent := t.TempDir()
			source := filepath.Join(parent, "workspace")
			output := filepath.Join(parent, "workspace.zip")
			if err := os.Mkdir(source, 0o755); err != nil {
				t.Fatal(err)
			}

			command := cli.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
			command.SetArgs([]string{source, "-o", output})
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), configPath) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Execute() error = %v, want config path and %q", err, test.want)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("final output exists or stat failed: %v", err)
			}
			matches, err := filepath.Glob(filepath.Join(parent, ".zipit-*.tmp"))
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 0 {
				t.Fatalf("temporary archives remain: %v", matches)
			}
		})
	}
}

func isolateGlobalConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", root)
	case "darwin":
		t.Setenv("HOME", root)
	default:
		t.Setenv("XDG_CONFIG_HOME", root)
	}
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	return path
}
