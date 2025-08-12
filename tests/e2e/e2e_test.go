package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/customsuites"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/pkg/s3client"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	f "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

func init() {
	testing.Init()
	f.RegisterClusterFlags(flag.CommandLine) // configures --kubeconfig flag
	f.RegisterCommonFlags(flag.CommandLine)  // configures --kubectl flag
	// Finalize and validate the test context after all flags are parsed.
	// This sets up global test configuration (e.g., kubeconfig, kubectl path, timeouts)
	// and ensures the E2E framework is ready to run tests.
	f.AfterReadingAllFlags(&f.TestContext)

	flag.StringVar(&AccessKeyId, "access-key-id", "", "S3 access key (or use ACCOUNT1_ACCESS_KEY env var)")
	flag.StringVar(&SecretAccessKey, "secret-access-key", "", "S3 secret access key (or use ACCOUNT1_SECRET_KEY env var)")
	flag.StringVar(&S3EndpointUrl, "s3-endpoint-url", "", "S3 endpoint URL, e.g. https://s3.example.com:8000")
	flag.BoolVar(&Performance, "performance", false, "run performance tests")
	flag.Parse()

	// Try to get configuration from environment variables if not provided via flags
	AccessKeyId = customsuites.GetEnv("ACCOUNT1_ACCESS_KEY", AccessKeyId)
	SecretAccessKey = customsuites.GetEnv("ACCOUNT1_SECRET_KEY", SecretAccessKey)
	S3EndpointUrl = customsuites.GetEnv("S3_ENDPOINT_URL", S3EndpointUrl)

	// Validate all required configuration after trying both flags and environment variables
	validateRequiredTestConfig("S3 endpoint URL", S3EndpointUrl, "--s3-endpoint-url", "S3_ENDPOINT_URL")
	validateRequiredTestConfig("S3 access key ID", AccessKeyId, "--access-key-id", "ACCOUNT1_ACCESS_KEY")
	validateRequiredTestConfig("S3 secret access key", SecretAccessKey, "--secret-access-key", "ACCOUNT1_SECRET_KEY")

	s3client.DefaultAccessKeyID = AccessKeyId
	s3client.DefaultSecretAccessKey = SecretAccessKey
	s3client.DefaultS3EndpointUrl = S3EndpointUrl
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Scality CSI Driver for S3 E2E Suite")
}

var CSITestSuites = []func() framework.TestSuite{
	// [sig-storage] CSI Volumes Test: Basic Data Persistence with Pre-provisioned PV.
	//
	// This test verifies that the S3 CSI driver supports storing and retrieving data
	// using a pre-provisioned PersistentVolume (PV). The backing S3 bucket is
	// created by the test driver's CreateVolume method.
	//
	// The test performs the following steps:
	// 1. Creates a Kubernetes namespace for test isolation.
	// 2. Creates an S3 bucket via the test driver.
	// 3. Sets up a pre-provisioned PV referencing this S3 bucket and a corresponding PVC.
	// 4. Launches a pod that mounts the PVC.
	// 5. Writes data to the S3 bucket through the mounted volume.
	// 6. Reads the data back and compares it to the expected content.
	// 7. Deletes all created resources (pod, PVC, PV, S3 bucket).
	//
	// This is part of the standard CSI driver compliance test suite and is
	// used to verify functional support for both static and dynamic provisioning with S3 storage.
	// Dynamic provisioning test: "[Testpattern: Dynamic PV (default fs)] volumes should store data"
	// Static provisioning test: "[Testpattern: Pre-provisioned PV (default fs)] volumes should store data"
	testsuites.InitVolumesTestSuite,

	// [sig-storage] CSI Volumes Test: Dynamic Provisioning
	//
	// This test verifies that the S3 CSI driver supports dynamic provisioning
	// where S3 buckets are created on-demand when PVCs are created.
	//
	// The test performs the following steps:
	// 1. Creates a Kubernetes namespace for test isolation.
	// 2. Creates a StorageClass for dynamic provisioning.
	// 3. Creates a PVC referencing the StorageClass.
	// 4. Verifies that a PV is dynamically created and bound.
	// 5. Launches a pod that mounts the PVC.
	// 6. Writes data to the S3 bucket through the mounted volume.
	// 7. Reads the data back and compares it to the expected content.
	// 8. Deletes all created resources (PVC deletion triggers PV and bucket cleanup).
	//
	// This validates the CSI driver's CreateVolume and DeleteVolume implementations.
	// This will be enabled in once we have proper authentication sources implemented
	// I am adding this so we do not forget it.
	// TODO(S3CSI-150): Re-enable this test once the "any volume data source" test is implemented
	// testsuites.InitProvisioningTestSuite,

	// Custom test suites specific to Scality CSI Driver for S3.
	customsuites.InitS3MountOptionsTestSuite,
	customsuites.InitS3MultiVolumeTestSuite,
	customsuites.InitS3CSICacheTestSuite,
	customsuites.InitS3FilePermissionsTestSuite,
	customsuites.InitS3DirectoryPermissionsTestSuite,
	customsuites.InitS3CredentialsTestSuite,
	customsuites.InitS3DynamicRbacTestSuite,
	customsuites.InitS3DynamicProvisioningAuthTestSuite,
}

// CSI test suite registration and execution.
// This registers the CSI driver with the Kubernetes E2E framework and defines which test suites to run.
// In performance mode, only performance tests are executed.
var _ = utils.SIGDescribe("CSI Volumes", func() {
	if Performance {
		CSITestSuites = []func() framework.TestSuite{customsuites.InitS3PerformanceTestSuite}
	}
	curDriver := initS3Driver()

	args := framework.GetDriverNameWithFeatureTags(curDriver)
	args = append(args, func() {
		framework.DefineTestSuites(curDriver, CSITestSuites)
	})
	f.Context(args...)
})

// validateRequiredTestConfig checks if a required test configuration value is set and exits with helpful error message if not
func validateRequiredTestConfig(description, value, flagName, envVarName string) {
	if value == "" {
		fmt.Printf("Error: %s is required for running tests but not provided via flags or environment variables.\n", description)
		fmt.Printf("To provide %s:\n", description)
		fmt.Printf("  - Use flag: %s\n", flagName)
		fmt.Printf("  - Or set environment variable: %s\n", envVarName)
		os.Exit(1)
	}
}
