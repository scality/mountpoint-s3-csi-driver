// package e2e

// import (
// 	"context"

// 	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"

// 	v1 "k8s.io/api/core/v1"
// 	storagev1 "k8s.io/api/storage/v1"
// 	"k8s.io/apimachinery/pkg/util/sets"
// 	f "k8s.io/kubernetes/test/e2e/framework"
// 	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
// 	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
// )

// const (
// 	// DriverName is the name of the CSI driver
// 	DriverName = "s3.csi.aws.com"
// 	// DriverNamespace is the namespace where the CSI driver is installed
// 	DriverNamespace = "kube-system"
// 	// DefaultFSType is the default filesystem type to use
// 	DefaultFSType = "ext4"
// 	// DefaultMountOptions are the default mount options to use
// 	DefaultMountOptions = "allow_other,uid=1000,gid=1000"
// )

// // ScalityDriver implements the TestDriver interface for Scality S3 CSI driver
// type ScalityDriver struct {
// 	s3Client   *s3client.Client
// 	driverInfo storageframework.DriverInfo
// }

// var _ storageframework.TestDriver = &ScalityDriver{}
// var _ storageframework.PreprovisionedVolumeTestDriver = &ScalityDriver{}
// var _ storageframework.PreprovisionedPVTestDriver = &ScalityDriver{}

// // ScalityVolume implements the TestVolume interface for Scality S3 volumes
// type ScalityVolume struct {
// 	bucketName string
// 	s3Client   *s3client.Client
// }

// var _ storageframework.TestVolume = &ScalityVolume{}

// // InitScalityDriver initializes the Scality S3 driver with the given configuration
// func InitScalityDriver(config *s3client.Config) (*ScalityDriver, error) {
// 	// Create S3 client from the provided configuration
// 	client, err := s3client.NewClient(config)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &ScalityDriver{
// 		s3Client: client,
// 		driverInfo: storageframework.DriverInfo{
// 			Name:        "s3.csi.aws.com", // Using the same driver name as specified
// 			MaxFileSize: storageframework.FileSizeLarge,
// 			SupportedFsType: sets.NewString(
// 				"", // Default fsType
// 			),
// 			Capabilities: map[storageframework.Capability]bool{
// 				storageframework.CapPersistence: true,
// 			},
// 			RequiredAccessModes: []v1.PersistentVolumeAccessMode{
// 				v1.ReadWriteMany,
// 				v1.ReadOnlyMany,
// 			},
// 		},
// 	}, nil
// }

// // GetDriverInfo returns the driver information
// func (d *ScalityDriver) GetDriverInfo() *storageframework.DriverInfo {
// 	return &d.driverInfo
// }

// // SkipUnsupportedTest skips tests that are not supported by the Scality S3 driver
// func (d *ScalityDriver) SkipUnsupportedTest(pattern storageframework.TestPattern) {
// 	if pattern.VolType != storageframework.PreprovisionedPV {
// 		e2eskipper.Skipf("Scality S3 Driver only supports static provisioning -- skipping")
// 	}
// }

// // PrepareTest prepares the test environment for the Scality S3 driver
// func (d *ScalityDriver) PrepareTest(ctx context.Context, f *f.Framework) *storageframework.PerTestConfig {
// 	config := &storageframework.PerTestConfig{
// 		Driver:    d,
// 		Prefix:    "s3",
// 		Framework: f,
// 	}

// 	return config
// }

// // CreateVolume creates a new S3 bucket for testing
// func (d *ScalityDriver) CreateVolume(ctx context.Context, config *storageframework.PerTestConfig, volumeType storageframework.TestVolType) storageframework.TestVolume {
// 	if volumeType != storageframework.PreprovisionedPV {
// 		f.Failf("Unsupported volumeType: %v is specified", volumeType)
// 	}

// 	// Create a bucket with a unique name using the namespace as part of the name
// 	bucketName, err := d.s3Client.CreateBucket(ctx)
// 	if err != nil {
// 		f.Failf("Failed to create bucket: %v", err)
// 	}

// 	return &ScalityVolume{
// 		bucketName: bucketName,
// 		s3Client:   d.s3Client,
// 	}
// }

// // GetPersistentVolumeSource returns the PV source for the Scality S3 volume
// func (d *ScalityDriver) GetPersistentVolumeSource(readOnly bool, fsType string, testVolume storageframework.TestVolume) (*v1.PersistentVolumeSource, *v1.VolumeNodeAffinity) {
// 	scalityVolume, ok := testVolume.(*ScalityVolume)
// 	if !ok {
// 		f.Failf("Failed to cast test volume to Scality volume")
// 	}

// 	volumeAttributes := map[string]string{"bucketName": scalityVolume.bucketName}

// 	return &v1.PersistentVolumeSource{
// 		CSI: &v1.CSIPersistentVolumeSource{
// 			Driver:           d.driverInfo.Name,
// 			VolumeHandle:     scalityVolume.bucketName,
// 			VolumeAttributes: volumeAttributes,
// 		},
// 	}, nil
// }

// // GetDynamicProvisionStorageClass returns a storage class for dynamic provisioning
// func (s *ScalityDriver) GetDynamicProvisionStorageClass(
// 	ctx context.Context, config *storageframework.PerTestConfig, fsType string,
// ) *storagev1.StorageClass {
// 	parameters := map[string]string{
// 		"csi.storage.k8s.io/provisioner-secret-name":       "s3-secret",
// 		"csi.storage.k8s.io/provisioner-secret-namespace":  DriverNamespace,
// 		"csi.storage.k8s.io/node-stage-secret-name":        "s3-secret",
// 		"csi.storage.k8s.io/node-stage-secret-namespace":   DriverNamespace,
// 		"csi.storage.k8s.io/node-publish-secret-name":      "s3-secret",
// 		"csi.storage.k8s.io/node-publish-secret-namespace": DriverNamespace,
// 	}

// 	if fsType != "" {
// 		parameters["csi.storage.k8s.io/fstype"] = fsType
// 	}

// 	return storageframework.GetStorageClass(
// 		DriverName,
// 		parameters,
// 		nil,
// 		config.Framework.Namespace.Name,
// 	)
// }

// // DeleteVolume deletes the S3 bucket
// func (v *ScalityVolume) DeleteVolume(ctx context.Context) {
// 	if err := v.s3Client.DeleteBucket(ctx, v.bucketName); err != nil {
// 		f.Logf("Failed to delete S3 Bucket: %s, error: %v", v.bucketName, err)
// 	}
// }

package e2e

import (
	"context"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	f "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/storage/framework"
)

var (
	CommitId     string
	BucketRegion string // assumed to be the same as k8s cluster's region
	BucketPrefix string
)

type s3Driver struct {
	client     *s3client.Client
	driverInfo framework.DriverInfo
}

type s3Volume struct {
	bucketName           string
	deleteBucket         s3client.DeleteBucketFunc // TODO: Add this to s3client.go
	authenticationSource string
}

var _ framework.TestDriver = &s3Driver{}
var _ framework.PreprovisionedVolumeTestDriver = &s3Driver{}
var _ framework.PreprovisionedPVTestDriver = &s3Driver{}

func initS3Driver() *s3Driver {
	return &s3Driver{
		client: s3client.New(),
		driverInfo: framework.DriverInfo{
			Name:        "s3.csi.aws.com",
			MaxFileSize: framework.FileSizeLarge,
			SupportedFsType: sets.NewString(
				"", // Default fsType
			),
			Capabilities: map[framework.Capability]bool{
				framework.CapPersistence: true,
			},
			RequiredAccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteMany,
				v1.ReadOnlyMany,
			},
		},
	}
}

func (d *s3Driver) GetDriverInfo() *framework.DriverInfo {
	return &d.driverInfo
}

func (d *s3Driver) SkipUnsupportedTest(pattern framework.TestPattern) {
	if pattern.VolType != framework.PreprovisionedPV {
		e2eskipper.Skipf("Scality S3 Driver only supports static provisioning -- skipping")
	}
}

func (d *s3Driver) PrepareTest(ctx context.Context, f *f.Framework) *framework.PerTestConfig {
	config := &framework.PerTestConfig{
		Driver:    d,
		Prefix:    "s3",
		Framework: f,
	}

	return config
}

func (d *s3Driver) CreateVolume(ctx context.Context, config *framework.PerTestConfig, volumeType framework.TestVolType) framework.TestVolume {
	if volumeType != framework.PreprovisionedPV {
		f.Failf("Unsupported volType: %v is specified", volumeType)
	}

	var bucketName string
	var deleteBucket s3client.DeleteBucketFunc
	// if config.Prefix == custom_testsuites.S3ExpressTestIdentifier {
	// 	bucketName, deleteBucket = d.client.CreateDirectoryBucket(ctx)
	// } else {
	// 	bucketName, deleteBucket = d.client.CreateStandardBucket(ctx)
	// }
	type contextKey string
	const authenticationSourceKey contextKey = "authenticationSource"

	bucketName, deleteBucket = d.client.CreateStandardBucket(ctx)
	val, _ := ctx.Value(authenticationSourceKey).(string)

	return &s3Volume{
		bucketName:           bucketName,
		deleteBucket:         deleteBucket,
		authenticationSource: val, // TODO: Add this to credentials.go file
	}
}

func (d *s3Driver) GetPersistentVolumeSource(readOnly bool, fsType string, testVolume framework.TestVolume) (*v1.PersistentVolumeSource, *v1.VolumeNodeAffinity) {
	volume, _ := testVolume.(*s3Volume)

	volumeAttributes := map[string]string{"bucketName": volume.bucketName}
	if volume.authenticationSource != "" {
		f.Logf("Using authentication source %s for volume", volume.authenticationSource)
		volumeAttributes["authenticationSource"] = volume.authenticationSource
	}

	return &v1.PersistentVolumeSource{
		CSI: &v1.CSIPersistentVolumeSource{
			Driver:           d.driverInfo.Name,
			VolumeHandle:     volume.bucketName,
			VolumeAttributes: volumeAttributes,
		},
	}, nil
}


func (v *s3Volume) DeleteVolume(ctx context.Context) {
	err := v.deleteBucket(ctx)
	f.ExpectNoError(err, "Failed to delete S3 Bucket: %s", v.bucketName)
}

