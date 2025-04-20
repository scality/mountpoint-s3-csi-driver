package e2e

import (
	"context"
	"flag"
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
)

var (
	// S3EndpointURL for the Scality S3 server
	S3EndpointURL string
	// AccessKeyID for authenticating with the S3 server
	AccessKeyID string
	// SecretAccessKey for authenticating with the S3 server
	SecretAccessKey string
	// BucketPrefix for creating unique bucket names
	BucketPrefix string
	// CleanupAfterTest flag to enable or disable cleanup after tests
	CleanupAfterTest bool
)

func init() {
	flag.StringVar(&S3EndpointURL, "s3-endpoint-url", "", "S3 endpoint URL")
	flag.StringVar(&AccessKeyID, "access-key-id", "", "S3 access key ID")
	flag.StringVar(&SecretAccessKey, "secret-access-key", "", "S3 secret access key")
	flag.StringVar(&BucketPrefix, "bucket-prefix", "e2e-test", "Prefix for S3 bucket names")
	flag.BoolVar(&CleanupAfterTest, "cleanup", true, "Enable cleanup after tests")
}

// TestBasicVolume implements a basic volume test that verifies the driver
// and bucket operations work correctly
func TestBasicVolume(t *testing.T) {
	flag.Parse()

	// Validate required flags
	if S3EndpointURL == "" {
		t.Fatalf("s3-endpoint-url is required")
	}
	if AccessKeyID == "" {
		t.Fatalf("access-key-id is required")
	}
	if SecretAccessKey == "" {
		t.Fatalf("secret-access-key is required")
	}

	// Create a new S3 driver
	s3Config := &s3client.Config{
		EndpointURL:     S3EndpointURL,
		AccessKeyID:     AccessKeyID,
		SecretAccessKey: SecretAccessKey,
		BucketPrefix:    BucketPrefix,
	}

	driver, err := InitScalityDriver(s3Config)
	if err != nil {
		t.Fatalf("Failed to initialize S3 driver: %v", err)
	}

	// Verify driver info
	driverInfo := driver.GetDriverInfo()
	if driverInfo == nil {
		t.Fatalf("Driver info should not be nil")
	}
	if driverInfo.Name != DriverName {
		t.Fatalf("Driver name should be %s, got %s", DriverName, driverInfo.Name)
	}

	// Verify capabilities
	if !driverInfo.Capabilities[storageframework.CapPersistence] {
		t.Fatalf("Driver should support persistence")
	}

	// Create a bucket using the driver's s3Client
	ctx := context.Background()
	bucketName, err := driver.s3Client.CreateBucket(ctx)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	t.Logf("Successfully created bucket: %s", bucketName)

	// Clean up the bucket if needed
	if CleanupAfterTest {
		defer func() {
			if err := driver.s3Client.DeleteBucket(ctx, bucketName); err != nil {
				t.Logf("Failed to delete bucket %s: %v", bucketName, err)
			} else {
				t.Logf("Successfully deleted bucket: %s", bucketName)
			}
		}()
	}

	t.Logf("Basic volume test passed! The Scality S3 CSI driver is working correctly")
}
