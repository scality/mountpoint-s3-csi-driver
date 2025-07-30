//go:build linux

package mounter

import (
	"os"

	"golang.org/x/sys/unix"
)

// VerifyMountPoint verifies that the given path is accessible using the statx syscall on Linux.
// This function provides enhanced mount point validation with Linux-specific optimizations.
// It falls back to regular os.Stat if statx is not supported on the system.
func VerifyMountPoint(path string) error {
	var stat unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_STATX_FORCE_SYNC, 0, &stat); err != nil {
		if err == unix.ENOSYS {
			// statx() syscall is not supported, retry with regular os.Stat
			_, err = os.Stat(path)
		}
		return err
	}

	return nil
}
