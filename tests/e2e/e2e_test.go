package e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/customsuites"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/pkg/s3client"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/pkg/vault"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	f "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

var (
	// Vault configuration flags
	VaultEndpoint       string
	VaultAdminAccessKey string
	VaultAdminSecretKey string

	// Global vault client and default test account
	vaultClient        *vault.VaultTestClient
	defaultTestAccount *vault.TestAccount
)

func init() {
	testing.Init()
	f.RegisterClusterFlags(flag.CommandLine) // configures --kubeconfig flag
	f.RegisterCommonFlags(flag.CommandLine)  // configures --kubectl flag
	// Finalize and validate the test context after all flags are parsed.
	// This sets up global test configuration (e.g., kubeconfig, kubectl path, timeouts)
	// and ensures the E2E framework is ready to run tests.
	f.AfterReadingAllFlags(&f.TestContext)

	// Original S3 credential flags (for backward compatibility)
	flag.StringVar(&AccessKeyId, "access-key-id", "", "S3 access key, e.g. accessKey1")
	flag.StringVar(&SecretAccessKey, "secret-access-key", "", "S3 secret access key, e.g. verySecretKey1")
	flag.StringVar(&S3EndpointUrl, "s3-endpoint-url", "", "S3 endpoint URL, e.g. https://s3.example.com:8000")

	// New Vault configuration flags
	flag.StringVar(&VaultEndpoint, "vault-endpoint", "", "Vault endpoint URL for dynamic account creation, e.g. https://vault.example.com")
	flag.StringVar(&VaultAdminAccessKey, "vault-admin-access-key", "", "Vault admin access key for account management")
	flag.StringVar(&VaultAdminSecretKey, "vault-admin-secret-key", "", "Vault admin secret key for account management")

	flag.BoolVar(&Performance, "performance", false, "run performance tests")
	flag.Parse()

	// Initialize credentials based on available configuration
	if VaultEndpoint != "" && VaultAdminAccessKey != "" && VaultAdminSecretKey != "" {
		// Use Vault for dynamic credential generation
		f.Logf("Initializing Vault client for dynamic credential generation")
		f.Logf("Vault endpoint: %s", VaultEndpoint)

		var err error
		vaultClient, err = vault.NewVaultTestClient(VaultEndpoint, VaultAdminAccessKey, VaultAdminSecretKey)
		if err != nil {
			fmt.Printf("Error: Failed to initialize Vault client: %v\n", err)
			os.Exit(1)
		}

		// Create default test account for general use
		defaultTestAccount, err = vaultClient.CreateTestAccount("E2EDefaultAccount")
		if err != nil {
			fmt.Printf("Error: Failed to create default test account: %v\n", err)
			os.Exit(1)
		}

		f.Logf("Created default test account: %s", defaultTestAccount.Name)
		f.Logf("Using dynamic credentials from Vault")

		// Set the default S3 client credentials to use the dynamically generated ones
		s3client.DefaultAccessKeyID = defaultTestAccount.AccessKey
		s3client.DefaultSecretAccessKey = defaultTestAccount.SecretKey

		// For backward compatibility, also set the global variables
		AccessKeyId = defaultTestAccount.AccessKey
		SecretAccessKey = defaultTestAccount.SecretKey

		// S3 endpoint is still required
		if S3EndpointUrl == "" {
			fmt.Println("Error: --s3-endpoint-url is required even when using Vault")
			os.Exit(1)
		}
		s3client.DefaultS3EndpointUrl = S3EndpointUrl

		// Set the VaultClient for credentials tests
		customsuites.SetVaultClient(vaultClient)

	} else if AccessKeyId != "" && SecretAccessKey != "" && S3EndpointUrl != "" {
		// Use traditional static credentials
		f.Logf("Using static credentials (traditional mode)")
		s3client.DefaultAccessKeyID = AccessKeyId
		s3client.DefaultSecretAccessKey = SecretAccessKey
		s3client.DefaultS3EndpointUrl = S3EndpointUrl

	} else {
		// Neither configuration is complete
		fmt.Println("Error: Either provide static credentials (--access-key-id, --secret-access-key, --s3-endpoint-url) OR Vault configuration (--vault-endpoint, --vault-admin-access-key, --vault-admin-secret-key, --s3-endpoint-url)")
		os.Exit(1)
	}
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Scality S3 CSI Driver E2E Suite")
}

// Setup cleanup for Vault accounts using AfterSuite
var _ = ginkgo.AfterSuite(func() {
	if vaultClient != nil {
		f.Logf("Cleaning up Vault accounts")
		if err := vaultClient.CleanupAllAccounts(); err != nil {
			f.Logf("Warning: Failed to cleanup Vault accounts: %v", err)
		}
	}
})

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
	// used to verify functional support for static provisioning with S3 storage.
	testsuites.InitVolumesTestSuite,

	// Custom test suites specific to Scality S3 CSI driver.
	customsuites.InitS3MountOptionsTestSuite,
	customsuites.InitS3MultiVolumeTestSuite,
	customsuites.InitS3CSICacheTestSuite,
	customsuites.InitS3FilePermissionsTestSuite,
	customsuites.InitS3DirectoryPermissionsTestSuite,
	customsuites.InitS3CredentialsTestSuite,
}

// initS3Driver initializes and returns an S3 CSI driver implementation for E2E testing.
// This function creates a test driver that implements required Kubernetes framework interfaces
// (TestDriver, PreprovisionedVolumeTestDriver, PreprovisionedPVTestDriver). The framework
// orchestrates testing by calling the driver's methods to:
// - Create S3 buckets via CreateVolume
// - Configure the buckets as CSI persistent volumes via GetPersistentVolumeSource
// - Clean up by deleting buckets via DeleteVolume
// This implementation supports both ReadWriteMany and ReadOnlyMany access modes and only works
// with pre-provisioned persistent volumes.
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

// GetVaultClient returns the global vault client instance (may be nil if not using Vault)
func GetVaultClient() *vault.VaultTestClient {
	return vaultClient
}

// GetDefaultTestAccount returns the default test account (may be nil if not using Vault)
func GetDefaultTestAccount() *vault.TestAccount {
	return defaultTestAccount
}
