package e2e

import (
	"context"
	"fmt"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
)

const (
	// DriverName is the name of the CSI driver
	DriverName = "s3.csi.scality.com"
	// DriverNamespace is the namespace where the CSI driver is installed
	DriverNamespace = "kube-system"
	// DefaultFSType is the default filesystem type to use
	DefaultFSType = "ext4"
	// DefaultMountOptions are the default mount options to use
	DefaultMountOptions = "allow_other,uid=1000,gid=1000"
)

// ScalityDriver implements the TestDriver interface for the S3 CSI driver
type ScalityDriver struct {
	driverInfo *storageframework.DriverInfo
	s3Client   *s3client.Client
}

var (
	// Ensure ScalityDriver implements TestDriver and other required interfaces
	_ storageframework.TestDriver                 = &ScalityDriver{}
	_ storageframework.DynamicPVTestDriver        = &ScalityDriver{}
	_ storageframework.PreprovisionedPVTestDriver = &ScalityDriver{}
)

// InitScalityDriver returns a new S3 CSI driver test driver
func InitScalityDriver(s3Config *s3client.Config) (*ScalityDriver, error) {
	if s3Config == nil {
		return nil, fmt.Errorf("s3Config cannot be nil")
	}

	s3Client, err := s3client.NewClient(s3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %v", err)
	}

	return &ScalityDriver{
		driverInfo: &storageframework.DriverInfo{
			Name:        DriverName,
			MaxFileSize: storageframework.FileSizeMedium,
			SupportedFsType: sets.NewString(
				"", // Default fsType
				DefaultFSType,
			),
			SupportedMountOption: sets.NewString(
				"allow_other",
				"uid",
				"gid",
				"cache_ttl",
				"readahead",
			),
			Capabilities: map[storageframework.Capability]bool{
				storageframework.CapPersistence: true,
				storageframework.CapExec:        true,
				storageframework.CapRWX:         true,
				storageframework.CapMultiPODs:   true,
			},
		},
		s3Client: s3Client,
	}, nil
}

// GetDriverInfo returns driver information
func (s *ScalityDriver) GetDriverInfo() *storageframework.DriverInfo {
	return s.driverInfo
}

// SkipUnsupportedTest skips tests that are not supported by the S3 CSI driver
func (s *ScalityDriver) SkipUnsupportedTest(pattern storageframework.TestPattern) {
	if pattern.VolType == storageframework.InlineVolume {
		framework.Skipf("S3 CSI Driver does not support inline volumes")
	}
	if pattern.VolType == storageframework.PreprovisionedPV && pattern.FsType == "" {
		framework.Skipf("S3 CSI Driver requires an explicit fsType to be specified")
	}
}

// PrepareTest prepares test resources
func (s *ScalityDriver) PrepareTest(ctx context.Context) (*storageframework.PerTestConfig, func()) {
	return &storageframework.PerTestConfig{
		Driver:    s,
		Prefix:    "s3",
		Framework: nil,
	}, func() {}
}

// CreateVolume creates a test volume for testing
func (s *ScalityDriver) CreateVolume(ctx context.Context, config *storageframework.PerTestConfig, volType storageframework.TestVolType) storageframework.TestVolume {
	switch volType {
	case storageframework.PreprovisionedPV:
		return s.createPreProvisionedVolume(ctx, config)
	case storageframework.DynamicPV:
		return s.createDynamicVolume(ctx, config)
	default:
		framework.Failf("Unsupported volType: %v is specified", volType)
		return nil
	}
}

// createPreProvisionedVolume creates a pre-provisioned volume
func (s *ScalityDriver) createPreProvisionedVolume(ctx context.Context, config *storageframework.PerTestConfig) *s3Volume {
	// Create a bucket in S3
	bucketName, err := s.s3Client.CreateBucket(ctx)
	if err != nil {
		framework.Failf("Failed to create bucket: %v", err)
	}

	return &s3Volume{
		bucketName: bucketName,
		s3Client:   s.s3Client,
	}
}

// createDynamicVolume creates a dynamic volume
func (s *ScalityDriver) createDynamicVolume(ctx context.Context, config *storageframework.PerTestConfig) *s3Volume {
	// Create a bucket in S3
	bucketName, err := s.s3Client.CreateBucket(ctx)
	if err != nil {
		framework.Failf("Failed to create bucket: %v", err)
	}

	return &s3Volume{
		bucketName: bucketName,
		s3Client:   s.s3Client,
	}
}

// GetPersistentVolumeSource returns a PV source for a pre-provisioned volume
func (s *ScalityDriver) GetPersistentVolumeSource(readOnly bool, fsType string, volume storageframework.TestVolume) *v1.PersistentVolumeSource {
	s3Vol, ok := volume.(*s3Volume)
	if !ok {
		framework.Failf("Failed to cast test volume to s3Volume")
	}

	return &v1.PersistentVolumeSource{
		CSI: &v1.CSIPersistentVolumeSource{
			Driver:           DriverName,
			VolumeHandle:     s3Vol.bucketName,
			FSType:           fsType,
			VolumeAttributes: map[string]string{"bucket": s3Vol.bucketName},
			ReadOnly:         readOnly,
		},
	}
}

// GetDynamicProvisionStorageClass returns a storage class for dynamic provisioning
func (s *ScalityDriver) GetDynamicProvisionStorageClass(
	ctx context.Context, config *storageframework.PerTestConfig, fsType string,
) *storagev1.StorageClass {
	parameters := map[string]string{
		"csi.storage.k8s.io/provisioner-secret-name":       "s3-secret",
		"csi.storage.k8s.io/provisioner-secret-namespace":  DriverNamespace,
		"csi.storage.k8s.io/node-stage-secret-name":        "s3-secret",
		"csi.storage.k8s.io/node-stage-secret-namespace":   DriverNamespace,
		"csi.storage.k8s.io/node-publish-secret-name":      "s3-secret",
		"csi.storage.k8s.io/node-publish-secret-namespace": DriverNamespace,
	}

	if fsType != "" {
		parameters["csi.storage.k8s.io/fstype"] = fsType
	}

	return storageframework.GetStorageClass(
		DriverName,
		parameters,
		nil,
		config.Framework.Namespace.Name,
	)
}

// s3Volume implements the TestVolume interface for S3 volumes
type s3Volume struct {
	bucketName string
	s3Client   *s3client.Client
}

var _ storageframework.TestVolume = &s3Volume{}

// DeleteVolume deletes the S3 bucket
func (v *s3Volume) DeleteVolume(ctx context.Context) {
	if v.s3Client != nil && v.bucketName != "" {
		if err := v.s3Client.DeleteBucket(ctx, v.bucketName); err != nil {
			framework.Logf("Failed to delete bucket %s: %v", v.bucketName, err)
		}
	}
}
