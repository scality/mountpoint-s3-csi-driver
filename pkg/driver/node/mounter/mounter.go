//go:generate mockgen -source=mounter.go -destination=./mocks/mock_mount.go -package=mock_driver
package mounter

import (
	"context"
	"os"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/system"
)

type ServiceRunner interface {
	StartService(ctx context.Context, config *system.ExecConfig) (string, error)
	RunOneshot(ctx context.Context, config *system.ExecConfig) (string, error)
}

// Mounter is an interface for mount operations
type Mounter interface {
	Mount(ctx context.Context, bucketName string, target string, credentialCtx credentialprovider.ProvideContext, args mountpoint.Args) error
	Unmount(ctx context.Context, target string, credentialCtx credentialprovider.CleanupContext) error
	IsMountPoint(target string) (bool, error)
}

const (
	MountS3PathEnv     = "MOUNT_S3_PATH"
	defaultMountS3Path = "/usr/bin/mount-s3"
)

func MountS3Path() string {
	mountS3Path := os.Getenv(MountS3PathEnv)
	if mountS3Path == "" {
		mountS3Path = defaultMountS3Path
	}
	return mountS3Path
}
