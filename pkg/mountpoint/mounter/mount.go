// Package mounter provides low-level mount operations for Mountpoint S3 CSI driver.
// This package centralizes mount, unmount, and mount point validation functionality
// that was previously scattered across different mounter implementations.
package mounter

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
)

// Common constants for mount operations
const (
	// MountpointDeviceName is the device name used by mountpoint-s3
	// https://github.com/awslabs/mountpoint-s3/blob/9ed8b6243f4511e2013b2f4303a9197c3ddd4071/mountpoint-s3/src/cli.rs#L421
	MountpointDeviceName = "mountpoint-s3"
)

// Common errors for mount operations
var (
	// ErrMissingTarget indicates that the mount target path is missing or empty
	ErrMissingTarget = fmt.Errorf("mount target is missing or empty")

	// ErrTargetNotDirectory indicates that the mount target is not a directory
	ErrTargetNotDirectory = fmt.Errorf("mount target is not a directory")
)

// Target represents a mount target path
type Target = string

// MountOptions represents standardized mount options for Mountpoint operations
type MountOptions struct {
	ReadOnly   bool
	AllowOther bool
}

// Mounter provides an interface for low-level mount operations.
// This interface abstracts platform-specific mount implementations.
type Mounter struct {
	mountutils mount.Interface
}

// NewMounter creates a new Mounter instance with the given mount interface.
func NewMounter(mountInterface mount.Interface) *Mounter {
	return &Mounter{
		mountutils: mountInterface,
	}
}

// NewDefaultMounter creates a new Mounter instance with the default mount interface.
func NewDefaultMounter() *Mounter {
	return NewMounter(mount.New(""))
}

// CheckMountpoint returns whether given `target` is a `mount-s3` mount.
// We implement additional check on top of `mounter.IsMountPoint` because we need
// to verify not only that the target is a mount point but also that it is specifically a mount-s3 mount point.
// This is achieved by calling the `mounter.List()` method to enumerate all mount points.
func CheckMountpoint(mounter mount.Interface, target string) (bool, error) {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return false, err
	}

	// Resolve symlinks to handle macOS /var -> /private/var and similar cases
	// This is needed for tests to pass on macOS where temp directories created in /var/folders
	// actually resolve to /private/var/folders. Without this, mount point  detection fails
	// because mount.List() returns resolved paths while we're checking with unresolved paths.
	// This doesn't affect Linux systems but ensures tests work correctly on developer machines.
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		klog.V(5).Infof("Failed to resolve symlinks for %s: %v, using original path", target, err)
		resolvedTarget = target
	}

	mountPoints, err := mounter.List()
	if err != nil {
		return false, fmt.Errorf("failed to list mounts: %w", err)
	}
	for _, mp := range mountPoints {
		// Check both original and resolved paths
		if mp.Path == target || mp.Path == resolvedTarget {
			if mp.Device != MountpointDeviceName {
				klog.V(4).Infof("CheckMountpoint: %s is not a `mount-s3` mount. Expected device type to be %s but got %s, skipping unmount", target, MountpointDeviceName, mp.Device)
				continue
			}

			return true, nil
		}
	}
	return false, nil
}

// CheckMountpointWithMounter is a convenience function that uses the Mounter's internal mount interface.
func (m *Mounter) CheckMountpoint(target string) (bool, error) {
	return CheckMountpoint(m.mountutils, target)
}

// UnmountTarget performs unmount operation on the given target path.
func UnmountTarget(mounter mount.Interface, target string) error {
	return mounter.Unmount(target)
}

// Unmount unmounts the given target path.
func (m *Mounter) Unmount(target string) error {
	return m.mountutils.Unmount(target)
}

// IsMountpointCorrupted checks if a mount point error indicates corruption.
// A mount point is considered corrupted when it's in an inconsistent state.
func (m *Mounter) IsMountpointCorrupted(err error) bool {
	// Check if the error indicates a corrupted mount
	// This typically happens when a mount point exists but is in an invalid state
	return mount.IsCorruptedMnt(err)
}

// FindReferencesToMountpoint finds all bind mount references to the given mount point.
func (m *Mounter) FindReferencesToMountpoint(source string) ([]string, error) {
	return m.mountutils.GetMountRefs(source)
}
