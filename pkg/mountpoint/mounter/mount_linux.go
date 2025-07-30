//go:build linux

package mounter

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
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

// OpenFUSEDevice opens /dev/fuse and returns the file descriptor on Linux.
func OpenFUSEDevice() (int, error) {
	fd, err := syscall.Open("/dev/fuse", os.O_RDWR, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to open /dev/fuse: %w", err)
	}
	return fd, nil
}

// CloseFUSEDevice closes the given FUSE file descriptor on Linux.
func CloseFUSEDevice(fd int) {
	err := syscall.Close(fd)
	if err != nil {
		klog.V(4).Infof("Mount: failed to close /dev/fuse file descriptor %d: %v\n", fd, err)
	}
}

// CreateMountOptions creates FUSE mount options from the given file descriptor, target path, and mount arguments on Linux.
func CreateMountOptions(fd int, target string, args mountpoint.Args) ([]string, error) {
	var stat syscall.Stat_t
	err := syscall.Stat(target, &stat)
	if err != nil {
		return nil, fmt.Errorf("failed to stat mount point %s: %w", target, err)
	}

	options := []string{
		fmt.Sprintf("fd=%d", fd),
		fmt.Sprintf("rootmode=%o", stat.Mode&syscall.S_IFMT),
		fmt.Sprintf("user_id=%d", os.Geteuid()),
		fmt.Sprintf("group_id=%d", os.Getegid()),
		"default_permissions",
	}

	if args.Has(mountpoint.ArgAllowOther) || args.Has(mountpoint.ArgAllowRoot) {
		options = append(options, "allow_other")
	}

	return options, nil
}

// CreateMountFlags creates mount flags from the given mount arguments on Linux.
func CreateMountFlags(args mountpoint.Args) uintptr {
	flags := uintptr(syscall.MS_NODEV | syscall.MS_NOSUID | syscall.MS_NOATIME)

	if args.Has(mountpoint.ArgReadOnly) {
		flags |= syscall.MS_RDONLY
	}

	return flags
}

// PerformMount performs the actual mount syscall with the given parameters on Linux.
func PerformMount(target string, options []string, flags uintptr) error {
	optionsJoined := strings.Join(options, ",")
	klog.V(4).Infof("Mounting %s with options %s", target, optionsJoined)

	err := syscall.Mount(MountpointDeviceName, target, "fuse", flags, optionsJoined)
	if err != nil {
		return fmt.Errorf("failed to mount %s: %w", target, err)
	}

	return nil
}
