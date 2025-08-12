package e2e

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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
			Name:        "s3.csi.scality.com",
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

	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		Provisioner: d.driverInfo.Name, // "s3.csi.scality.com"
		Parameters: map[string]string{
			"csi.storage.k8s.io/provisioner-secret-name":       provSecretName,
			"csi.storage.k8s.io/provisioner-secret-namespace":  config.Framework.Namespace.Name,
			"csi.storage.k8s.io/node-publish-secret-name":      nodeSecretName,
			"csi.storage.k8s.io/node-publish-secret-namespace": config.Framework.Namespace.Name,
		},
		ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
	}
}

func (d *s3Driver) SkipUnsupportedTest(pattern framework.TestPattern) {
	if pattern.VolType != framework.PreprovisionedPV && pattern.VolType != framework.DynamicPV {
		e2eskipper.Skipf("Scality S3 Driver: unsupported volume type - %v", pattern.VolType)
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
