package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Write atomically persists a complete configuration, creating its parent
// directory only when explicitly called.
func Write(path string, value Config) (err error) {
	return write(path, value, publishConfig)
}

// Create atomically persists a new configuration and fails if path already
// exists. It is used by interactive initialization so a concurrent creator is
// never overwritten.
func Create(path string, value Config) error {
	return write(path, value, publishNewConfig)
}

func write(path string, value Config, publish func(string, string) error) (err error) {
	if err := validate(value); err != nil {
		return fmt.Errorf("validate config %s: %w", path, err)
	}
	data, err := toml.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, ".zipit-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config in %s: %w", directory, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err = temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set temporary config permissions: %w", err)
	}
	if _, err = temporary.Write(data); err != nil {
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err = publish(temporaryPath, path); err != nil {
		return fmt.Errorf("publish config %s: %w", path, err)
	}
	return nil
}
