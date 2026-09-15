// Package presets defines the static exclusion choices offered by config init.
package presets

// Preset is a named collection of explicit Zipit exclusion patterns.
type Preset struct {
	ID          string
	Name        string
	Description string
	Patterns    []string
	Default     bool
}

var catalog = []Preset{
	{ID: "git", Name: "Git metadata", Description: "Repository internals", Patterns: []string{".git/"}, Default: true},
	{ID: "node", Name: "Node dependencies", Patterns: []string{"node_modules/"}},
	{ID: "python", Name: "Python environments & cache", Patterns: []string{"__pycache__/", ".venv/", "venv/", "*.pyc"}},
	{ID: "rust", Name: "Rust build output", Patterns: []string{"target/"}},
	{ID: "os", Name: "OS metadata", Patterns: []string{".DS_Store", "Thumbs.db"}, Default: true},
	{ID: "logs", Name: "Log files", Patterns: []string{"*.log"}},
	{ID: "cache", Name: "Cache directories", Patterns: []string{".cache/"}},
	{ID: "build", Name: "Build outputs", Patterns: []string{"dist/", "build/"}},
	{ID: "editor", Name: "Editor metadata", Description: "May include intentionally shared settings", Patterns: []string{".idea/", ".vscode/"}},
}

// All returns the stable, ordered preset catalog.
func All() []Preset {
	result := make([]Preset, len(catalog))
	for index, preset := range catalog {
		result[index] = preset
		result[index].Patterns = append([]string(nil), preset.Patterns...)
	}
	return result
}
