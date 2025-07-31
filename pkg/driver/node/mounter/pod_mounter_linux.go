package mounter

import (
	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
	mpmounter "github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint/mounter"
)

// mountSyscallDefault creates a FUSE file descriptor and performs a `mount` syscall with given `target` and mount arguments.
// Deprecated: This function uses the new centralized mount operations from pkg/mountpoint/mounter.
func (pm *PodMounter) mountSyscallDefault(target string, args mountpoint.Args) (int, error) {
	fd, err := mpmounter.OpenFUSEDevice()
	if err != nil {
		return 0, err
	}

	// This will set false on a success condition and will stay true
	// in all error conditions to ensure we don't leave the file descriptor open in case we can't do
	// the mount operation.
	closeFd := true
	defer func() {
		if closeFd {
			mpmounter.CloseFUSEDevice(fd)
		}
	}()

	options, err := mpmounter.CreateMountOptions(fd, target, args)
	if err != nil {
		return 0, err
	}

	flags := mpmounter.CreateMountFlags(args)

	err = mpmounter.PerformMount(target, options, flags)
	if err != nil {
		return 0, err
	}

	// We successfully performed the mount operation, ensure to not close the FUSE file descriptor.
	closeFd = false
	return fd, nil
}
