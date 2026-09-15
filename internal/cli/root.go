// Package cli defines Zipit's command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/poizdev/zipit/internal/archive"
	"github.com/poizdev/zipit/internal/buildinfo"
	"github.com/poizdev/zipit/internal/config"
	"github.com/poizdev/zipit/internal/rules"
	"github.com/spf13/cobra"
)

// ErrInterrupted identifies an archive operation stopped by an interrupt.
var ErrInterrupted = errors.New("archive interrupted")

// NewRootCommand constructs the Zipit command.
func NewRootCommand(stdout, stderr io.Writer) *cobra.Command {
	var output string
	var excludes []string
	var dryRun bool
	var force bool

	command := &cobra.Command{
		Use:     "zipit [PATH]",
		Short:   "Create a ZIP archive from a directory",
		Version: buildinfo.Version,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			started := time.Now()
			source := "."
			if len(args) == 1 {
				source = args[0]
			}
			request, err := archive.ResolveRequest(source, output)
			if err != nil {
				return err
			}
			configPath, err := config.Path()
			if err != nil {
				return err
			}
			globalConfig, err := config.Load(configPath)
			if err != nil {
				return err
			}
			effectiveRules := make([]rules.Rule, 0, len(globalConfig.Ignore.Patterns)+len(excludes))
			for _, pattern := range globalConfig.Ignore.Patterns {
				effectiveRules = append(effectiveRules, rules.Rule{Pattern: pattern, Source: configPath})
			}
			workspaceRules, err := rules.LoadIgnoreFile(request.Source)
			if err != nil {
				return err
			}
			effectiveRules = append(effectiveRules, workspaceRules...)
			for _, pattern := range excludes {
				effectiveRules = append(effectiveRules, rules.Rule{Pattern: pattern})
			}
			matcher, err := rules.CompileRules(effectiveRules)
			if err != nil {
				return err
			}
			request.Matcher = matcher
			request.DryRun = dryRun
			request.Force = force
			ctx, stop := signal.NotifyContext(cmd.Context(), interruptSignals()...)
			defer stop()
			result, err := archive.Run(ctx, request)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return ErrInterrupted
				}
				return err
			}
			if dryRun {
				return printDryRunSummary(cmd.OutOrStdout(), request.Output, request.Force, result, time.Since(started))
			}
			if err := printArchiveSummary(cmd.OutOrStdout(), request.Output, result, time.Since(started)); err != nil {
				return err
			}
			maybeNotifyUpdate(cmd.Context(), cmd.ErrOrStderr())
			return nil
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.Flags().StringVarP(&output, "output", "o", "", "output ZIP path")
	command.Flags().StringArrayVarP(&excludes, "exclude", "x", nil, "exclude pattern (repeatable)")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "preview archive selection without writing a ZIP")
	command.Flags().BoolVarP(&force, "force", "f", false, "safely replace an existing output archive")
	command.AddCommand(newInitCommand(), newConfigCommand(), newVersionCommand(), newUpdateCommand())
	return command
}

func printDryRunSummary(writer io.Writer, output string, force bool, result archive.Result, elapsed time.Duration) error {
	if _, err := fmt.Fprintf(writer, "Would include %s\nInput size: %s\n", fileCount(result.Stats.FilesIncluded), formatBytes(result.Stats.BytesIncluded)); err != nil {
		return err
	}
	if result.Stats.FilesExcluded > 0 {
		if _, err := fmt.Fprintf(writer, "Skipped files: %d\n", result.Stats.FilesExcluded); err != nil {
			return err
		}
	}
	if result.Stats.DirectoriesExcluded > 0 {
		if _, err := fmt.Fprintf(writer, "Skipped directories: %d\n", result.Stats.DirectoriesExcluded); err != nil {
			return err
		}
	}
	if err := printFilesystemWarning(writer, result.Stats); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "\nWould create:\n%s\n", output); err != nil {
		return err
	}
	if _, err := os.Lstat(output); err == nil {
		message := "Output already exists and would require --force."
		if force {
			message = "Output already exists and would be replaced."
		}
		if _, err := fmt.Fprintln(writer, message); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect output %q: %w", output, err)
	}
	_, err := fmt.Fprintf(writer, "\nDone in %s\n", formatDuration(elapsed))
	return err
}

func printArchiveSummary(writer io.Writer, output string, result archive.Result, elapsed time.Duration) error {
	if _, err := fmt.Fprintf(
		writer,
		"%s • %s input → %s archive (%s)\n",
		fileCount(result.Stats.FilesIncluded),
		formatBytes(result.Stats.BytesIncluded),
		formatBytes(result.ArchiveSize),
		filepath.Base(output),
	); err != nil {
		return err
	}
	if err := printFilesystemWarning(writer, result.Stats); err != nil {
		return err
	}
	_, err := fmt.Fprintf(writer, "Done in %s\n", formatDuration(elapsed))
	return err
}

func printFilesystemWarning(writer io.Writer, stats archive.Stats) error {
	if stats.SymlinksIncluded > 0 {
		label := "symlinks"
		if stats.SymlinksIncluded == 1 {
			label = "symlink"
		}
		if _, err := fmt.Fprintf(writer, "Preserved %d %s\n", stats.SymlinksIncluded, label); err != nil {
			return err
		}
	}
	if stats.SpecialFilesSkipped == 0 {
		return nil
	}
	_, err := fmt.Fprintf(writer, "Skipped %d unsupported filesystem entries\n", stats.SpecialFilesSkipped)
	return err
}

func fileCount(count int64) string {
	if count == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", count)
}

func formatBytes(size int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func formatDuration(duration time.Duration) string {
	if duration < time.Millisecond {
		return "<1ms"
	}
	if duration < time.Second {
		return duration.Round(time.Millisecond).String()
	}
	return fmt.Sprintf("%.1fs", duration.Seconds())
}
