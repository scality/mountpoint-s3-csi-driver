package e2e

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/onsi/ginkgo/v2"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/constants"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/customsuites"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/pkg/s3client"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	f "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/utils/ptr"
)

var (
	AccessKeyId     string
	SecretAccessKey string
	S3EndpointUrl   string
	Performance     bool
)

type s3Driver struct {
	client     *s3client.Client
	driverInfo framework.DriverInfo
}

type s3Volume struct {
	bucketName           string
	deleteBucket         s3client.DeleteBucketFunc
	authenticationSource string
	isDynamic            bool
}

var (
	_ framework.TestDriver                     = &s3Driver{}
	_ framework.PreprovisionedVolumeTestDriver = &s3Driver{}
	_ framework.PreprovisionedPVTestDriver     = &s3Driver{}
	_ framework.DynamicPVTestDriver            = &s3Driver{}
)

func initS3Driver() *s3Driver {
	return &s3Driver{
		client: s3client.New("", "", ""),
		driverInfo: framework.DriverInfo{
			Name:        constants.DriverName,
			MaxFileSize: framework.FileSizeLarge,
			SupportedFsType: sets.NewString(
				"", // Default fsType
			),
			Capabilities: map[framework.Capability]bool{
				framework.CapPersistence:        true,
				framework.CapMultiPODs:          true,  // Support multiple pods accessing same volume
				framework.CapRWX:                true,  // Support ReadWriteMany access mode
				framework.CapExec:               false, // S3 doesn't support chmod permissions needed for these tests
				framework.CapSnapshotDataSource: false, // S3 doesn't support volume snapshots
				framework.CapPVCDataSource:      false, // S3 doesn't support PVC cloning
			},
			SupportedMountOption: sets.NewString(
				"allow-other", // Standard S3 mount options that are tested
				"allow-root",
				"gid",
				"uid",
			),
			RequiredAccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteMany, // S3 naturally supports multi-node read-write access
				// ReadOnlyMany is ReadWriteMany with read-only mount option
			},
		},
	}
}

func (d *s3Driver) GetDriverInfo() *framework.DriverInfo {
	return &d.driverInfo
}

func (d *s3Driver) GetDynamicProvisionStorageClass(
	ctx context.Context,
	config *framework.PerTestConfig,
	fsType string,
) *storagev1.StorageClass {
	// Generate unique storage class name
	scName := fmt.Sprintf("s3-sc-%s", uuid.NewString()[:8])

	// Create provisioner secret for authentication
	provSecretName, err := customsuites.CreateProvisionerSecret(ctx, config.Framework)
	if err != nil {
		f.Failf("Failed to create provisioner secret: %v", err)
	}

	// Create node-publish secret for mounting authentication
	nodeSecretName, err := customsuites.CreateNodePublishSecret(ctx, config.Framework)
	if err != nil {
		f.Failf("Failed to create node-publish secret: %v", err)
	}

	// Determine volume binding mode based on test pattern
	// WaitForFirstConsumer is useful for testing node affinity and delayed binding scenarios
	volumeBindingMode := storagev1.VolumeBindingImmediate
	if config.Prefix == "s3-waitforfirstconsumer" {
		volumeBindingMode = storagev1.VolumeBindingWaitForFirstConsumer
	}

	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		Provisioner: d.driverInfo.Name, // constants.DriverName
		Parameters: map[string]string{
			"csi.storage.k8s.io/provisioner-secret-name":       provSecretName,
			"csi.storage.k8s.io/provisioner-secret-namespace":  config.Framework.Namespace.Name,
			"csi.storage.k8s.io/node-publish-secret-name":      nodeSecretName,
			"csi.storage.k8s.io/node-publish-secret-namespace": config.Framework.Namespace.Name,
		},
		VolumeBindingMode: &volumeBindingMode,
		ReclaimPolicy:     ptr.To(v1.PersistentVolumeReclaimDelete),
	}

	return storageClass
}

func (d *s3Driver) SkipUnsupportedTest(pattern framework.TestPattern) {
	if pattern.VolType != framework.PreprovisionedPV && pattern.VolType != framework.DynamicPV {
		e2eskipper.Skipf("Scality CSI driver for S3: unsupported volume type - %v", pattern.VolType)
	}

	// Skip tests that require ReadWriteOnce access mode
	// S3 is naturally multi-node storage, only supports ReadWriteMany
	// Use mount options for read-only behavior instead of ReadOnlyMany access mode
	currentTestName := ginkgo.CurrentSpecReport().FullText()
	
	if pattern.VolType == framework.DynamicPV {
		// The Kubernetes e2e test framework defaults to ReadWriteOnce for most tests
		// This is incompatible with our S3 driver design which only supports ReadWriteMany
		if strings.Contains(currentTestName, "should provision storage with mount options") {
			e2eskipper.Skipf("Scality CSI driver for S3: mount options test uses ReadWriteOnce by default, S3 only supports ReadWriteMany")
		}

		// Skip volume data source tests - S3 doesn't support volume cloning or populators
		if strings.Contains(currentTestName, "should provision storage with any volume data source") {
			e2eskipper.Skipf("Scality CSI driver for S3: volume data sources not supported - S3 doesn't support volume cloning or populators")
		}
	}
	
	// Skip tests that are incompatible with v2 pod mounter architecture
	// These tests were designed for v1 systemd-based mounting and don't work correctly with pod-based mounting
	if strings.Contains(currentTestName, "should work with read-only mount option") ||
		strings.Contains(currentTestName, "should enforce read-only flag when specified") {
		e2eskipper.Skipf("Scality CSI driver for S3: Read-only mount tests not compatible with v2 pod mounter architecture - read-only enforcement differs between systemd and pod mounter")
	}
	
	if strings.Contains(currentTestName, "should properly apply permissions with pod security context settings") {
		e2eskipper.Skipf("Scality CSI driver for S3: File permission tests not compatible with v2 pod mounter architecture - permission propagation differs in pod mounter")
	}
	
	if strings.Contains(currentTestName, "fails to mount with 'Access Denied Error: Failed to create mount process' error when using valid credentials without permissions") {
		e2eskipper.Skipf("Scality CSI driver for S3: Credential error test not compatible with v2 pod mounter architecture - error messages differ in pod mounter")
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
	switch volumeType {
	case framework.PreprovisionedPV:
		var bucketName string
		var deleteBucket s3client.DeleteBucketFunc
		type contextKey string
		const authenticationSourceKey contextKey = "authenticationSource"

		bucketName, deleteBucket = d.client.CreateBucket(ctx)
		val, _ := ctx.Value(authenticationSourceKey).(string)

		return &s3Volume{
			bucketName:           bucketName,
			deleteBucket:         deleteBucket,
			authenticationSource: val,
			isDynamic:            false,
		}
	case framework.DynamicPV:
		// For dynamic provisioning, no pre-created bucket needed
		// The CSI driver will create the bucket during CreateVolume RPC
		return &s3Volume{
			isDynamic: true,
		}
	default:
		f.Failf("Unsupported volType: %v", volumeType)
		return nil
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
	if v.isDynamic {
		// For dynamic volumes, the CSI driver handles deletion
		// No manual bucket cleanup needed
		return
	}

	// Existing code for static volumes
	if v.deleteBucket != nil {
		err := v.deleteBucket(ctx)
		f.ExpectNoError(err, "failed to delete S3 Bucket: %s", v.bucketName)
	}
}
