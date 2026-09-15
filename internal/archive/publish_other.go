//go:build !linux && !darwin && !windows

package archive

import "os"

func publishNoReplace(tempPath, finalPath string) error {
	// Keep unsupported targets safe: Link fails when finalPath exists. The
	// primary Linux, macOS, and Windows targets use native no-replace renames.
	if err := os.Link(tempPath, finalPath); err != nil {
		return err
	}
	if err := os.Remove(tempPath); err != nil {
		_ = os.Remove(finalPath)
		return err
	}
	return nil
}

func publishReplace(tempPath, finalPath string) error {
	return os.Rename(tempPath, finalPath)
}
