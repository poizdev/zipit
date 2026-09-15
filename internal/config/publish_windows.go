//go:build windows

package config

import "golang.org/x/sys/windows"

func publishConfig(temporaryPath, path string) error {
	return moveConfig(temporaryPath, path, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func publishNewConfig(temporaryPath, path string) error {
	return moveConfig(temporaryPath, path, windows.MOVEFILE_WRITE_THROUGH)
}

func moveConfig(temporaryPath, path string, flags uint32) error {
	from, err := windows.UTF16PtrFromString(temporaryPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, flags)
}
