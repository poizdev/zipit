package presets_test

import (
	"slices"
	"testing"

	"github.com/poizdev/zipit/internal/presets"
	"github.com/poizdev/zipit/internal/rules"
)

func TestCatalogIsValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, preset := range presets.All() {
		if preset.ID == "" || seen[preset.ID] {
			t.Fatalf("invalid or duplicate preset ID %q", preset.ID)
		}
		seen[preset.ID] = true
		if _, err := rules.Compile(preset.Patterns); err != nil {
			t.Fatalf("preset %q has invalid patterns: %v", preset.ID, err)
		}
	}
	for _, id := range []string{"git", "node", "python", "rust", "os", "logs", "cache", "build", "editor"} {
		if !seen[id] {
			t.Errorf("missing preset %q", id)
		}
	}
}

func TestOnlyConservativePresetsDefaultSelected(t *testing.T) {
	defaults := map[string]bool{}
	for _, preset := range presets.All() {
		defaults[preset.ID] = preset.Default
	}
	if !defaults["git"] || !defaults["os"] {
		t.Fatal("Git and OS metadata must be selected by default")
	}
	for _, id := range []string{"node", "python", "rust", "logs", "cache", "build", "editor"} {
		if defaults[id] {
			t.Errorf("preset %q must remain opt-in", id)
		}
	}
}

func TestCatalogContainsDocumentedPatterns(t *testing.T) {
	want := map[string][]string{
		"git":    {".git/"},
		"node":   {"node_modules/"},
		"python": {"__pycache__/", ".venv/", "venv/", "*.pyc"},
		"rust":   {"target/"},
		"os":     {".DS_Store", "Thumbs.db"},
		"logs":   {"*.log"},
		"cache":  {".cache/"},
		"build":  {"dist/", "build/"},
		"editor": {".idea/", ".vscode/"},
	}
	for _, preset := range presets.All() {
		if !slices.Equal(preset.Patterns, want[preset.ID]) {
			t.Errorf("preset %q patterns = %v, want %v", preset.ID, preset.Patterns, want[preset.ID])
		}
	}
}
