package cli

import (
	"fmt"
	"io"

	"github.com/poizdev/zipit/internal/buildinfo"
	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if short {
				return printShortVersion(cmd.OutOrStdout())
			}
			return printFullVersion(cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVar(&short, "short", false, "print only the version string")
	return cmd
}

func printShortVersion(w io.Writer) error {
	_, err := fmt.Fprintln(w, buildinfo.FullVersion())
	return err
}

func printFullVersion(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "Zipit %s\n", buildinfo.FullVersion()); err != nil {
		return err
	}
	if buildinfo.Commit != "none" {
		if _, err := fmt.Fprintf(w, "Commit: %s\n", buildinfo.Commit); err != nil {
			return err
		}
	}
	if buildinfo.BuildDate != "unknown" {
		if _, err := fmt.Fprintf(w, "Built: %s\n", buildinfo.BuildDate); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "Go: %s\n", buildinfo.GoVersion()); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "Platform: %s\n", buildinfo.Platform())
	return err
}
