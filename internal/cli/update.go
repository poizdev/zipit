package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/poizdev/zipit/internal/buildinfo"
	"github.com/poizdev/zipit/internal/update"
	"github.com/spf13/cobra"
)

func newUpdateCommand() *cobra.Command {
	var check bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for updates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Keep the bare command and explicit check form equivalent: update
			// discovery never replaces the installed binary.
			return runUpdateCheck(cmd)
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "check for available updates")
	return cmd
}

func runUpdateCheck(cmd *cobra.Command) error {
	cacheDir, _ := update.DefaultCacheDir()

	client := update.NewClient(
		update.WithCacheDir(cacheDir),
	)

	result, err := client.Check(cmd.Context(), buildinfo.FullVersion())
	if err != nil {
		return fmt.Errorf("unable to check for updates: %w", err)
	}

	return printUpdateResult(cmd.OutOrStdout(), result)
}

func printUpdateResult(w io.Writer, result *update.CheckResult) error {
	if result.IsDevBuild {
		if _, err := fmt.Fprintf(w, "Current build: %s\n", result.Current); err != nil {
			return err
		}
		if result.Latest != nil {
			_, err := fmt.Fprintf(w, "Latest release: %s\n", result.Latest.Version)
			return err
		}
		_, err := fmt.Fprintln(w, "No releases found.")
		return err
	}

	current := ensureVPrefix(result.Current)

	if result.Latest == nil {
		_, err := fmt.Fprintf(w, "Zipit %s (no releases found)\n", current)
		return err
	}

	if result.NewerAvailable {
		if _, err := fmt.Fprintf(w, "Zipit %s is available.\n", result.Latest.Version); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, "Current: %s\n", current)
		return err
	}

	_, err := fmt.Fprintf(w, "Zipit %s is up to date.\n", current)
	return err
}

func ensureVPrefix(v string) string {
	if len(v) > 0 && v[0] != 'v' {
		return "v" + v
	}
	return v
}

// maybeNotifyUpdate reads cached update discovery state after an archive and
// prints a concise notice to stderr if it already records a newer version. It
// never performs network discovery; explicit update commands own refreshing
// the cache.
func maybeNotifyUpdate(ctx context.Context, stderr io.Writer) {
	// Only notify on interactive terminals.
	file, ok := stderr.(*os.File)
	if !ok || !isInteractive(file) {
		return
	}

	currentVersion := buildinfo.FullVersion()
	if update.IsDevVersion(currentVersion) {
		return
	}

	cacheDir, err := update.DefaultCacheDir()
	if err != nil {
		return
	}

	client := update.NewClient(
		update.WithCacheDir(cacheDir),
	)

	result, err := client.CheckImplicit(ctx, currentVersion)
	if err != nil || result == nil {
		return
	}

	if result.NewerAvailable && result.Latest != nil {
		fmt.Fprintf(stderr, "\nUpdate available: %s (current %s)\n",
			result.Latest.Version, ensureVPrefix(currentVersion))
	}
}
