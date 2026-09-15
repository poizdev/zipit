//go:build !windows

package cli

import "os"

func publishNewWorkspaceIgnore(temporaryPath, path string) error {
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}
	return os.Remove(temporaryPath)
}
