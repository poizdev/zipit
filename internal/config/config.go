// Package config owns Zipit's global user configuration persistence.
package config

import "fmt"

// CurrentSchema is the only configuration schema supported by this version.
const CurrentSchema = 1

// Config is Zipit's persisted global configuration.
type Config struct {
	Schema int `toml:"schema"`
	Ignore struct {
		Patterns []string `toml:"patterns"`
	} `toml:"ignore"`
}

// Empty returns a valid configuration with no exclusion patterns.
func Empty() Config {
	return Config{Schema: CurrentSchema}
}

func validate(value Config) error {
	if value.Schema != CurrentSchema {
		return fmt.Errorf("unsupported schema %d (expected %d)", value.Schema, CurrentSchema)
	}
	return nil
}
