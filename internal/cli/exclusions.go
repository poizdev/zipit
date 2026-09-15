package cli

import (
	"fmt"
	"io"

	"github.com/charmbracelet/huh"
	"github.com/poizdev/zipit/internal/presets"
	"github.com/poizdev/zipit/internal/rules"
)

type exclusionInteraction interface {
	selectPresets([]presets.Preset) ([]presets.Preset, error)
	confirm(string) (bool, error)
	input(string, func(string) error) (string, error)
}

type huhExclusionInteraction struct {
	inputReader  io.Reader
	outputWriter io.Writer
}

func newHuhExclusionInteraction(input io.Reader, output io.Writer) exclusionInteraction {
	return &huhExclusionInteraction{inputReader: input, outputWriter: output}
}

func (h *huhExclusionInteraction) selectPresets(catalog []presets.Preset) ([]presets.Preset, error) {
	options := make([]huh.Option[string], 0, len(catalog))
	for _, preset := range catalog {
		label := preset.Name
		if preset.Description != "" {
			label += " — " + preset.Description
		}
		options = append(options, huh.NewOption(label, preset.ID).Selected(preset.Default))
	}
	selectedIDs := make([]string, 0)
	err := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Select common exclusions").
			Options(options...).
			Value(&selectedIDs),
	)).WithInput(h.inputReader).WithOutput(h.outputWriter).Run()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]presets.Preset, len(catalog))
	for _, preset := range catalog {
		byID[preset.ID] = preset
	}
	selected := make([]presets.Preset, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		preset, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown preset %q", id)
		}
		selected = append(selected, preset)
	}
	return selected, nil
}

func (h *huhExclusionInteraction) confirm(title string) (bool, error) {
	answer := false
	err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title(title).Affirmative("Yes").Negative("No").Value(&answer),
	)).WithInput(h.inputReader).WithOutput(h.outputWriter).Run()
	return answer, err
}

func (h *huhExclusionInteraction) input(title string, validate func(string) error) (string, error) {
	value := ""
	err := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title(title).Prompt("> ").Validate(validate).Value(&value),
	)).WithInput(h.inputReader).WithOutput(h.outputWriter).Run()
	return value, err
}

func defaultPresets() []presets.Preset {
	defaults := make([]presets.Preset, 0)
	for _, preset := range presets.All() {
		if preset.Default {
			defaults = append(defaults, preset)
		}
	}
	return defaults
}

func chooseExclusionRules(interaction exclusionInteraction) ([]string, error) {
	selected, err := interaction.selectPresets(presets.All())
	if err != nil {
		return nil, err
	}
	custom := make([]string, 0)
	addCustom, err := interaction.confirm("Add custom exclusions?")
	if err != nil {
		return nil, err
	}
	for addCustom {
		pattern, inputErr := interaction.input("Exclusion pattern", func(value string) error {
			_, validationErr := rules.Canonical(value)
			return validationErr
		})
		if inputErr != nil {
			return nil, inputErr
		}
		custom = append(custom, pattern)
		addCustom, err = interaction.confirm("Add another?")
		if err != nil {
			return nil, err
		}
	}
	return buildInitialPatterns(selected, custom)
}

func buildInitialPatterns(selected []presets.Preset, custom []string) ([]string, error) {
	patterns := make([]string, 0)
	for _, preset := range selected {
		patterns = append(patterns, preset.Patterns...)
	}
	patterns = append(patterns, custom...)

	result := make([]string, 0, len(patterns))
	seen := make(map[string]bool, len(patterns))
	for _, pattern := range patterns {
		canonical, err := rules.Canonical(pattern)
		if err != nil {
			return nil, err
		}
		if !seen[canonical] {
			seen[canonical] = true
			result = append(result, canonical)
		}
	}
	return result, nil
}
