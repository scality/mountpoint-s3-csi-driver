//go:build darwin

package mounter

import (
	"os"
)

// VerifyMountPoint verifies that the given path is accessible on Darwin.
// Since statx is a Linux-specific syscall, this implementation uses the standard os.Stat function.
func VerifyMountPoint(path string) error {
	// statx is a Linux-specific syscall, use os.Stat on Darwin
	_, err := os.Stat(path)
	return err
}
