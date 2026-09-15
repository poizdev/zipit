package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const workspaceIgnoreName = ".zipitignore"

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactively create workspace exclusions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workingDirectory, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve current workspace: %w", err)
			}
			path := filepath.Join(workingDirectory, workspaceIgnoreName)
			if err := refuseExistingWorkspaceIgnore(path); err != nil {
				return err
			}
			if !isInteractive(cmd.InOrStdin()) || !isInteractive(cmd.OutOrStdout()) {
				return fmt.Errorf("init requires an interactive terminal; create %s manually for non-interactive use", path)
			}
			return runWorkspaceInit(
				cmd.OutOrStdout(),
				path,
				newHuhExclusionInteraction(cmd.InOrStdin(), cmd.OutOrStdout()),
			)
		},
	}
}

func runWorkspaceInit(writer io.Writer, path string, interaction exclusionInteraction) error {
	if _, err := fmt.Fprintln(writer, "Initialize Zipit for this workspace"); err != nil {
		return err
	}
	patterns, err := chooseExclusionRules(interaction)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "\nWorkspace exclusions"); err != nil {
		return err
	}
	if len(patterns) == 0 {
		if _, err := fmt.Fprintln(writer, "  (none)"); err != nil {
			return err
		}
	} else {
		for _, pattern := range patterns {
			if _, err := fmt.Fprintf(writer, "  %s\n", pattern); err != nil {
				return err
			}
		}
	}
	confirmed, err := interaction.confirm("Create .zipitignore?")
	if err != nil {
		return err
	}
	if !confirmed {
		_, err = fmt.Fprintln(writer, "Cancelled.")
		return err
	}
	if err := writeWorkspaceIgnore(path, patterns); err != nil {
		if existingErr := refuseExistingWorkspaceIgnore(path); existingErr != nil {
			return existingErr
		}
		return err
	}
	_, err = fmt.Fprintf(writer, "Created:\n%s\n", path)
	return err
}

func refuseExistingWorkspaceIgnore(path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return fmt.Errorf(".zipitignore already exists:\n%s\n\nEdit it directly to change workspace exclusions", path)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	return nil
}

func writeWorkspaceIgnore(path string, patterns []string) error {
	return writeWorkspaceIgnoreWithPublisher(path, patterns, publishNewWorkspaceIgnore)
}

func writeWorkspaceIgnoreWithPublisher(path string, patterns []string, publish func(string, string) error) (err error) {
	canonical, err := buildInitialPatterns(nil, patterns)
	if err != nil {
		return err
	}
	contents := strings.Join(canonical, "\n")
	if len(canonical) > 0 {
		contents += "\n"
	}

	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".zipitignore-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary .zipitignore in %s: %w", directory, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err = temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("set temporary .zipitignore permissions: %w", err)
	}
	if _, err = io.WriteString(temporary, contents); err != nil {
		return fmt.Errorf("write temporary .zipitignore: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close temporary .zipitignore: %w", err)
	}
	if err = publish(temporaryPath, path); err != nil {
		return fmt.Errorf("publish %s: %w", path, err)
	}
	return nil
}
