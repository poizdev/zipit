//go:build darwin

package archive

import "golang.org/x/sys/unix"

func publishNoReplace(tempPath, finalPath string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, tempPath, unix.AT_FDCWD, finalPath, unix.RENAME_EXCL)
}

func publishReplace(tempPath, finalPath string) error {
	return unix.Rename(tempPath, finalPath)
}
