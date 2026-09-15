//go:build !windows

package archive_test

import "golang.org/x/sys/unix"

func makeFIFO(path string) error {
	return unix.Mkfifo(path, 0o600)
}
