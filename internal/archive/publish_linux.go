//go:build linux

package archive

import "golang.org/x/sys/unix"

func publishNoReplace(tempPath, finalPath string) error {
	return unix.Renameat2(unix.AT_FDCWD, tempPath, unix.AT_FDCWD, finalPath, unix.RENAME_NOREPLACE)
}

func publishReplace(tempPath, finalPath string) error {
	return unix.Rename(tempPath, finalPath)
}
