// Package mounter provides low-level mount operations for Mountpoint S3 CSI driver.
// This package centralizes mount, unmount, and mount point validation functionality
// that was previously scattered across different mounter implementations.
package mounter

import (
	"fmt"

	"k8s.io/mount-utils"
)

// Common constants for mount operations will be added as needed

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

// Additional utility functions will be added as needed in subsequent commits
