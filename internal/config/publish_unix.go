//go:build !windows

package config

import "os"

func publishConfig(temporaryPath, path string) error {
	return os.Rename(temporaryPath, path)
}

func publishNewConfig(temporaryPath, path string) error {
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}
	return os.Remove(temporaryPath)
}
