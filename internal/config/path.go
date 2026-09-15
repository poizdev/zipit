package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Path returns the platform-standard global Zipit configuration path.
func Path() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(directory, "zipit", "config.toml"), nil
}
