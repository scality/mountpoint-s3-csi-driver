//go:build darwin

package mounter

import (
	"errors"
	"os"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
)

// VerifyMountPoint verifies that the given path is accessible on Darwin.
// Since statx is a Linux-specific syscall, this implementation uses the standard os.Stat function.
func VerifyMountPoint(path string) error {
	// statx is a Linux-specific syscall, use os.Stat on Darwin
	_, err := os.Stat(path)
	return err
}

// OpenFUSEDevice returns an error on Darwin as FUSE device operations are not supported.
func OpenFUSEDevice() (int, error) {
	return 0, errors.New("FUSE device operations only supported on Linux")
}

// CloseFUSEDevice is a no-op on Darwin as FUSE device operations are not supported.
func CloseFUSEDevice(fd int) {
	// No-op on Darwin
}

// CreateMountOptions returns an error on Darwin as FUSE mount options are Linux-specific.
func CreateMountOptions(fd int, target string, args mountpoint.Args) ([]string, error) {
	return nil, errors.New("FUSE mount options only supported on Linux")
}

// CreateMountFlags returns an error on Darwin as mount flags are Linux-specific.
func CreateMountFlags(args mountpoint.Args) uintptr {
	// Return 0 as mount flags are Linux-specific
	return 0
}

// PerformMount returns an error on Darwin as mount syscall is Linux-specific.
func PerformMount(target string, options []string, flags uintptr) error {
	return errors.New("mount syscall only supported on Linux")
}
