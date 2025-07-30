package mounter

import (
	"errors"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
	mpmounter "github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint/mounter"
)

func (pm *PodMounter) mountSyscallDefault(_ string, _ mountpoint.Args) (int, error) {
	return 0, errors.New("Only supported on Linux")
}

// verifyMountPointStatx verifies that the given path is accessible on Darwin.
// Deprecated: Use mpmounter.VerifyMountPoint instead. This function is kept for backward compatibility.
func verifyMountPointStatx(path string) error {
	return mpmounter.VerifyMountPoint(path)
}
