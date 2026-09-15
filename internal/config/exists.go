package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Exists reports whether the config path exists without creating anything.
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("inspect config %s: %w", path, err)
}
