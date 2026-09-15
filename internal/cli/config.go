package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/poizdev/zipit/internal/config"
	"github.com/poizdev/zipit/internal/rules"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newConfigCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Manage global exclusion rules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	command.AddCommand(
		newConfigInitCommand(),
		newConfigAddCommand(),
		newConfigRemoveCommand(),
		newConfigListCommand(),
		newConfigEditCommand(),
	)
	return command
}

func newConfigInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactively create the global config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			exists, err := config.Exists(path)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("global config already exists:\n%s\n\nUse `zipit config edit` or the add/remove commands to modify it", path)
			}
			if !isInteractive(cmd.InOrStdin()) || !isInteractive(cmd.OutOrStdout()) {
				return fmt.Errorf("config init requires an interactive terminal; use `zipit config add <pattern>` instead")
			}
			return runConfigInit(cmd.OutOrStdout(), path, newHuhExclusionInteraction(cmd.InOrStdin(), cmd.OutOrStdout()))
		},
	}
}

func newConfigAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <pattern>",
		Short: "Add one global exclusion rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			value, err := config.Load(path)
			if err != nil {
				return err
			}
			if err := validateConfigRules(path, value); err != nil {
				return err
			}
			pattern, err := rules.Canonical(args[0])
			if err != nil {
				return err
			}
			for _, existing := range value.Ignore.Patterns {
				canonical, err := rules.Canonical(existing)
				if err != nil {
					return fmt.Errorf("invalid rule in %s: %w", path, err)
				}
				if canonical == pattern {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Already configured: %s\n", pattern)
					return err
				}
			}
			value.Ignore.Patterns = append(value.Ignore.Patterns, pattern)
			if err := config.Write(path, value); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Added: %s\n", pattern)
			return err
		},
	}
}

func newConfigRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <pattern>",
		Short: "Remove one global exclusion rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			exists, err := config.Exists(path)
			if err != nil {
				return err
			}
			pattern, err := rules.Canonical(args[0])
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("rule not configured: %s (global config does not exist: %s)", pattern, path)
			}
			value, err := config.Load(path)
			if err != nil {
				return err
			}
			if err := validateConfigRules(path, value); err != nil {
				return err
			}
			index := -1
			for current, existing := range value.Ignore.Patterns {
				canonical, canonicalErr := rules.Canonical(existing)
				if canonicalErr != nil {
					return fmt.Errorf("invalid rule in %s: %w", path, canonicalErr)
				}
				if canonical == pattern {
					index = current
					break
				}
			}
			if index < 0 {
				return fmt.Errorf("rule not configured: %s", pattern)
			}
			value.Ignore.Patterns = append(value.Ignore.Patterns[:index], value.Ignore.Patterns[index+1:]...)
			if err := config.Write(path, value); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed: %s\n", pattern)
			return err
		},
	}
}

func newConfigListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List global exclusion rules",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			value, err := config.Load(path)
			if err != nil {
				return err
			}
			if len(value.Ignore.Patterns) == 0 {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "No global excludes configured.\n\nConfig:\n%s\n", path)
				return err
			}
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), "Global excludes"); err != nil {
				return err
			}
			for index, pattern := range value.Ignore.Patterns {
				if _, err = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", index+1, pattern); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "\nConfig:\n%s\n", path)
			return err
		},
	}
}

func newConfigEditCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the global config with $VISUAL or $EDITOR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			exists, err := config.Exists(path)
			if err != nil {
				return err
			}
			if !exists {
				if err := config.Write(path, config.Empty()); err != nil {
					return err
				}
			}
			name, arguments, err := resolveEditor()
			if err != nil {
				return fmt.Errorf("%w\n\nSet $VISUAL or $EDITOR, or edit:\n%s", err, path)
			}
			arguments = append(arguments, path)
			process := exec.Command(name, arguments...)
			process.Stdin = cmd.InOrStdin()
			process.Stdout = cmd.OutOrStdout()
			process.Stderr = cmd.ErrOrStderr()
			if err := process.Run(); err != nil {
				return fmt.Errorf("editor failed: %w", err)
			}
			value, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("edited config is invalid: %w", err)
			}
			if err := validateConfigRules(path, value); err != nil {
				return fmt.Errorf("edited config is invalid: %w", err)
			}
			return nil
		},
	}
}

func validateConfigRules(path string, value config.Config) error {
	raw := make([]rules.Rule, 0, len(value.Ignore.Patterns))
	for _, pattern := range value.Ignore.Patterns {
		raw = append(raw, rules.Rule{Pattern: pattern, Source: path})
	}
	_, err := rules.CompileRules(raw)
	return err
}

func runConfigInit(writer io.Writer, path string, interaction exclusionInteraction) error {
	if _, err := fmt.Fprintln(writer, "Create global Zipit configuration"); err != nil {
		return err
	}
	patterns, err := chooseExclusionRules(interaction)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "\nGlobal exclusions"); err != nil {
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
	answer, err := interaction.confirm("Create config?")
	if err != nil {
		return err
	}
	if !answer {
		_, err = fmt.Fprintln(writer, "Cancelled.")
		return err
	}
	value := config.Empty()
	value.Ignore.Patterns = patterns
	if err := config.Create(path, value); err != nil {
		exists, existsErr := config.Exists(path)
		if existsErr == nil && exists {
			return fmt.Errorf("global config already exists:\n%s\n\nUse `zipit config edit` or the add/remove commands to modify it", path)
		}
		return err
	}
	_, err = fmt.Fprintf(writer, "Created:\n%s\n", path)
	return err
}

func isInteractive(stream any) bool {
	file, ok := stream.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func resolveEditor() (string, []string, error) {
	value := strings.TrimSpace(os.Getenv("VISUAL"))
	if value == "" {
		value = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if value == "" {
		return "", nil, fmt.Errorf("No editor configured")
	}
	fields, err := splitEditorCommand(value)
	if err != nil {
		return "", nil, fmt.Errorf("invalid editor command: %w", err)
	}
	return fields[0], fields[1:], nil
}

// splitEditorCommand supports whitespace, single and double quotes, and
// backslash-escaped whitespace or quotes. It never expands variables or runs a
// shell, keeping editor configuration portable and safe.
func splitEditorCommand(value string) ([]string, error) {
	runes := []rune(value)
	fields := make([]string, 0)
	var current strings.Builder
	var quote rune
	started := false
	for index := 0; index < len(runes); index++ {
		character := runes[index]
		if quote == '\'' {
			if character == quote {
				quote = 0
			} else {
				current.WriteRune(character)
			}
			continue
		}
		if quote == '"' {
			if character == quote {
				quote = 0
				continue
			}
			if character == '\\' && index+1 < len(runes) && (runes[index+1] == '"' || runes[index+1] == '\\') {
				index++
				current.WriteRune(runes[index])
				continue
			}
			current.WriteRune(character)
			continue
		}
		switch {
		case character == '\'' || character == '"':
			quote = character
			started = true
		case character == '\\' && index+1 < len(runes) && (runes[index+1] == '\\' || runes[index+1] == '\'' || runes[index+1] == '"' || runes[index+1] == ' ' || runes[index+1] == '\t'):
			index++
			current.WriteRune(runes[index])
			started = true
		case character == ' ' || character == '\t' || character == '\n' || character == '\r':
			if started {
				fields = append(fields, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(character)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if started {
		fields = append(fields, current.String())
	}
	if len(fields) == 0 || fields[0] == "" {
		return nil, fmt.Errorf("missing executable")
	}
	return fields, nil
}
