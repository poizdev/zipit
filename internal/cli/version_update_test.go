package cli_test

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/cli"
)

func TestVersionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	command.SetArgs([]string{"version"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()

	// Must contain "Zipit dev" for dev builds.
	if !strings.Contains(output, "Zipit dev") {
		t.Fatalf("expected 'Zipit dev', got %q", output)
	}

	// Must contain Go version.
	if !strings.Contains(output, "Go: "+runtime.Version()) {
		t.Fatalf("expected Go version, got %q", output)
	}

	// Must contain platform.
	expectedPlatform := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(output, "Platform: "+expectedPlatform) {
		t.Fatalf("expected platform %q, got %q", expectedPlatform, output)
	}

	// Dev builds omit Commit and Built lines.
	if strings.Contains(output, "Commit:") {
		t.Fatalf("dev build should omit Commit, got %q", output)
	}
	if strings.Contains(output, "Built:") {
		t.Fatalf("dev build should omit Built, got %q", output)
	}
}

func TestVersionCommandShort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	command.SetArgs([]string{"version", "--short"})
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "dev" {
		t.Fatalf("--short output = %q, want %q", output, "dev")
	}
}

func TestUpdateCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	updateCommand, _, err := command.Find([]string{"update"})
	if err != nil {
		t.Fatal(err)
	}
	if updateCommand.Name() != "update" {
		t.Fatalf("command name = %q, want update", updateCommand.Name())
	}
	if updateCommand.Flags().Lookup("check") == nil {
		t.Fatal("update command is missing --check")
	}
}

func TestUpdateCommandBare(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := cli.NewRootCommand(&stdout, &stderr)
	updateCommand, _, err := command.Find([]string{"update"})
	if err != nil {
		t.Fatal(err)
	}
	if err := updateCommand.Args(updateCommand, nil); err != nil {
		t.Fatalf("bare update rejected: %v", err)
	}
}
