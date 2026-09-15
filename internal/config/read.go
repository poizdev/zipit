package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Load reads and validates a config file. A missing file returns Empty.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Empty(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var value Config
	if err := toml.Unmarshal(data, &value); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := validate(value); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	return value, nil
}
