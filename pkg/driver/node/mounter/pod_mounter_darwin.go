package mounter

import (
	"errors"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
)

func (pm *PodMounter) mountSyscallDefault(_ string, _ mountpoint.Args) (int, error) {
	return 0, errors.New("Only supported on Linux")
}
