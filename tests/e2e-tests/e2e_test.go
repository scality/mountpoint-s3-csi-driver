package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"
	f "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config" // Needed for flag registration
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	// Import for loading kubeconfig
	"k8s.io/client-go/tools/clientcmd"
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
	// Register framework flags first
	config.CopyFlags(config.Flags, flag.CommandLine)
	f.RegisterCommonFlags(flag.CommandLine)
	f.RegisterClusterFlags(flag.CommandLine)
	f.AfterReadingAllFlags(&f.TestContext)

	// Register our custom flags
	flag.StringVar(&S3EndpointURL, "s3-endpoint-url", "", "S3 endpoint URL")
	flag.StringVar(&AccessKeyID, "access-key-id", "", "S3 access key ID")
	flag.StringVar(&SecretAccessKey, "secret-access-key", "", "S3 secret access key")
	flag.StringVar(&BucketPrefix, "bucket-prefix", "e2e-test", "Prefix for S3 bucket names")
	flag.BoolVar(&CleanupAfterTest, "cleanup", true, "Enable cleanup after tests")
}

// Helper function to get server address from kubeconfig
func getAPIServerHostFromKubeconfig(kubeconfigPath string) (string, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig from %s: %w", kubeconfigPath, err)
	}

	// Get the current context
	currentContextName := config.CurrentContext
	currentContext := config.Contexts[currentContextName]
	if currentContext == nil {
		return "", fmt.Errorf("current context '%s' not found in kubeconfig %s", currentContextName, kubeconfigPath)
	}

	// Get the cluster associated with the current context
	clusterName := currentContext.Cluster
	cluster := config.Clusters[clusterName]
	if cluster == nil {
		return "", fmt.Errorf("cluster '%s' not found for context '%s' in kubeconfig %s", clusterName, currentContextName, kubeconfigPath)
	}

	if cluster.Server == "" {
		return "", fmt.Errorf("server address is empty for cluster '%s' in kubeconfig %s", clusterName, kubeconfigPath)
	}

	return cluster.Server, nil
}

// TestE2E is the main entry point for the Ginkgo tests
func TestE2E(t *testing.T) {
	// Parse all flags
	flag.Parse()

	// Validate required framework flags are set *before* reading kubeconfig
	if f.TestContext.KubeConfig == "" {
		t.Fatalf("--kubeconfig is required")
	}
	if f.TestContext.KubectlPath == "" {
		t.Fatalf("--kubectl-path is required")
	}

	// Dynamically set the host from the provided kubeconfig
	apiServerHost, err := getAPIServerHostFromKubeconfig(f.TestContext.KubeConfig)
	if err != nil {
		t.Fatalf("Failed to get API server host from kubeconfig: %v", err)
	}
	f.TestContext.Host = apiServerHost
	f.Logf("Using API Server Host from kubeconfig: %s", f.TestContext.Host) // Log the host being used

	// Set up Gomega and Ginkgo fail handlers
	gomega.RegisterFailHandler(ginkgo.Fail)

	// Validate required S3 flags
	if S3EndpointURL == "" {
		t.Fatalf("s3-endpoint-url is required")
	}
	if AccessKeyID == "" {
		t.Fatalf("access-key-id is required")
	}
	if SecretAccessKey == "" {
		t.Fatalf("secret-access-key is required")
	}

	// Set kubectl path in the environment if provided
	if err := os.Setenv("TEST_KUBECTL", f.TestContext.KubectlPath); err != nil {
		t.Fatalf("Failed to set TEST_KUBECTL environment variable: %v", err)
	}

	// Run the Ginkgo specs
	ginkgo.RunSpecs(t, "Scality S3 CSI Driver E2E Suite")
}

// ScalityTestSuites lists the test suites to run.
// For now, just the standard basic volume test.
var ScalityTestSuites = []func() storageframework.TestSuite{
	testsuites.InitVolumesTestSuite,
}

// This executes testSuites for the Scality CSI driver.
var _ = utils.SIGDescribe("Scality S3 CSI Driver", func() {
	// Create S3 config from flags
	s3Config := &s3client.Config{
		EndpointURL:     S3EndpointURL,
		AccessKeyID:     AccessKeyID,
		SecretAccessKey: SecretAccessKey,
		BucketPrefix:    BucketPrefix,
	}

	// Initialize the driver directly here, before getting args
	driver, err := InitScalityDriver(s3Config)
	// Use GinkgoRecover to handle potential panics during setup
	defer ginkgo.GinkgoRecover()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to initialize S3 driver during setup")
	gomega.Expect(driver).NotTo(gomega.BeNil(), "Driver should not be nil after initialization")

	// Get framework arguments including driver name and feature tags
	args := storageframework.GetDriverNameWithFeatureTags(driver)
	// Append the function that defines which test suites to run
	args = append(args, func() {
		storageframework.DefineTestSuites(driver, ScalityTestSuites)
	})
	// Run the tests within the framework context
	f.Context(args...)

	// Cleanup S3 resources after each test if enabled
	if CleanupAfterTest {
		ginkgo.AfterEach(func(ctx context.Context) {
			// Re-initialize driver for cleanup in case setup failed partially?
			// Or rely on the driver instance from the outer scope if it was successful?
			// Let's assume the driver was successfully initialized if we reach here.
			if driver != nil && driver.s3Client != nil {
				err := driver.s3Client.CleanupAllBuckets(ctx)
				if err != nil {
					// Use framework logging
					f.Logf("Failed to clean up buckets: %v", err)
				}
			}
		})
	}
})
