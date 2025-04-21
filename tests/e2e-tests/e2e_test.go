// package e2e

// import (
// 	"context"
// 	"flag"
// 	"fmt"
// 	"os"
// 	"testing"

// 	ginkgo "github.com/onsi/ginkgo/v2"
// 	"github.com/onsi/gomega"
// 	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"
// 	f "k8s.io/kubernetes/test/e2e/framework"
// 	"k8s.io/kubernetes/test/e2e/framework/config" // Needed for flag registration
// 	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
// 	"k8s.io/kubernetes/test/e2e/storage/testsuites"
// 	"k8s.io/kubernetes/test/e2e/storage/utils"

// 	// Import for loading kubeconfig
// 	"k8s.io/client-go/tools/clientcmd"
// )

// var (
// 	// S3EndpointURL for the Scality S3 server
// 	S3EndpointURL string
// 	// AccessKeyID for authenticating with the S3 server
// 	AccessKeyID string
// 	// SecretAccessKey for authenticating with the S3 server
// 	SecretAccessKey string
// 	// BucketPrefix for creating unique bucket names
// 	BucketPrefix string
// 	// CleanupAfterTest flag to enable or disable cleanup after tests
// 	CleanupAfterTest bool
// )

// func init() {
// 	// Register framework flags first
// 	config.CopyFlags(config.Flags, flag.CommandLine)
// 	f.RegisterCommonFlags(flag.CommandLine)
// 	f.RegisterClusterFlags(flag.CommandLine)

// 	// Register our custom flags
// 	flag.StringVar(&S3EndpointURL, "s3-endpoint-url", "", "S3 endpoint URL")
// 	flag.StringVar(&AccessKeyID, "access-key-id", "", "S3 access key ID")
// 	flag.StringVar(&SecretAccessKey, "secret-access-key", "", "S3 secret access key")
// 	flag.StringVar(&BucketPrefix, "bucket-prefix", "e2e-test", "Prefix for S3 bucket names")
// 	flag.BoolVar(&CleanupAfterTest, "cleanup", true, "Enable cleanup after tests")
// }

// // Helper function to get server address from kubeconfig
// func getAPIServerHostFromKubeconfig(kubeconfigPath string) (string, error) {
// 	config, err := clientcmd.LoadFromFile(kubeconfigPath)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to load kubeconfig from %s: %w", kubeconfigPath, err)
// 	}

// 	// Get the current context
// 	currentContextName := config.CurrentContext
// 	currentContext := config.Contexts[currentContextName]
// 	if currentContext == nil {
// 		return "", fmt.Errorf("current context '%s' not found in kubeconfig %s", currentContextName, kubeconfigPath)
// 	}

// 	// Get the cluster associated with the current context
// 	clusterName := currentContext.Cluster
// 	cluster := config.Clusters[clusterName]
// 	if cluster == nil {
// 		return "", fmt.Errorf("cluster '%s' not found for context '%s' in kubeconfig %s", clusterName, currentContextName, kubeconfigPath)
// 	}

// 	if cluster.Server == "" {
// 		return "", fmt.Errorf("server address is empty for cluster '%s' in kubeconfig %s", clusterName, kubeconfigPath)
// 	}

// 	return cluster.Server, nil
// }

// // TestE2E runs the Scality S3 CSI driver E2E tests.
// func TestE2E(t *testing.T) {
// 	// Register Gomega's fail handler
// 	gomega.RegisterFailHandler(ginkgo.Fail)

// 	// Register the framework flags and handle context
// 	f.AfterReadingAllFlags(&f.TestContext)

// 	// Validate required flags *after* parsing
// 	if S3EndpointURL == "" {
// 		t.Fatalf("s3-endpoint-url is required")
// 	}
// 	if AccessKeyID == "" {
// 		t.Fatalf("access-key-id is required")
// 	}
// 	if SecretAccessKey == "" {
// 		t.Fatalf("secret-access-key is required")
// 	}

// 	// Set kubectl path in the environment if provided
// 	if f.TestContext.KubectlPath != "" {
// 		if err := os.Setenv("TEST_KUBECTL", f.TestContext.KubectlPath); err != nil {
// 			t.Fatalf("Failed to set TEST_KUBECTL environment variable: %v", err)
// 		}
// 	}

// 	// Run the Ginkgo specs
// 	ginkgo.RunSpecs(t, "Scality S3 CSI Driver E2E Suite")
// }

// // ScalityTestSuites lists the test suites to run.
// // For now, just the standard basic volume test.
// var ScalityTestSuites = []func() storageframework.TestSuite{
// 	testsuites.InitVolumesTestSuite,
// }

// // Add a simple direct test to verify S3 client works
// var _ = ginkgo.Describe("Scality S3 CSI Driver Basic", func() {
// 	// Simple test to verify S3 client
// 	ginkgo.It("should be able to create a bucket", func(ctx context.Context) {
// 		// Create S3 config from flags
// 		s3Config := &s3client.Config{
// 			EndpointURL:     S3EndpointURL,
// 			AccessKeyID:     AccessKeyID,
// 			SecretAccessKey: SecretAccessKey,
// 			BucketPrefix:    BucketPrefix,
// 		}

// 		// Create S3 client
// 		client, err := s3client.NewClient(s3Config)
// 		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create S3 client")
// 		gomega.Expect(client).NotTo(gomega.BeNil(), "S3 client must not be nil")

// 		// Create a test bucket
// 		bucketName, err := client.CreateBucket(ctx)
// 		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create bucket")
// 		gomega.Expect(bucketName).NotTo(gomega.BeEmpty(), "Bucket name must not be empty")

// 		// Clean up the bucket
// 		err = client.DeleteBucket(ctx, bucketName)
// 		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to delete bucket")
// 	})
// })

// // This executes testSuites for the Scality CSI driver.
// var _ = utils.SIGDescribe("Scality S3 CSI Driver", func() {
// 	// Create S3 config from flags *before* driver init
// 	s3Config := &s3client.Config{
// 		EndpointURL:     S3EndpointURL,
// 		AccessKeyID:     AccessKeyID,
// 		SecretAccessKey: SecretAccessKey,
// 		BucketPrefix:    BucketPrefix,
// 	}

// 	// Initialize the driver directly here, as framework needs it early
// 	driver, err := InitScalityDriver(s3Config)
// 	// Log potential initialization error. Proper checks will happen later.
// 	if err != nil {
// 		f.Logf("Initial driver initialization failed (will be checked before defining suites): %v", err)
// 	} else if driver == nil {
// 		f.Logf("Initial driver initialization returned nil driver (will be checked before defining suites)")
// 	}

// 	// Get framework arguments including driver name and feature tags
// 	args := storageframework.GetDriverNameWithFeatureTags(driver)

// 	// Append the function that defines which test suites to run
// 	args = append(args, func() {
// 		// Perform driver initialization checks HERE, inside the framework function
// 		defer ginkgo.GinkgoRecover() // Add recover here as well
// 		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to initialize S3 driver before defining suites")
// 		gomega.Expect(driver).NotTo(gomega.BeNil(), "Driver must not be nil when defining suites")

// 		// Define the test suites using the validated driver
// 		storageframework.DefineTestSuites(driver, ScalityTestSuites)
// 	})

// 	// Run the tests within the framework context
// 	f.Context(args...)

// 	// Cleanup S3 resources after each test if enabled
// 	if CleanupAfterTest {
// 		ginkgo.AfterEach(func(ctx context.Context) {
// 			// Check if driver and s3Client were successfully initialized before cleanup
// 			if driver != nil && driver.s3Client != nil {
// 				err := driver.s3Client.CleanupAllBuckets(ctx)
// 				if err != nil {
// 					f.Logf("Failed to clean up buckets: %v", err)
// 				}
// 			}
// 		})
// 	}
// })

package e2e

import (
	"flag"
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"
	f "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

func init() {
	testing.Init()

	f.RegisterClusterFlags(flag.CommandLine) // configures --kubeconfig flag // TODO: Check if this flag is correct in test.sh
	f.RegisterCommonFlags(flag.CommandLine)  // configures --kubectl flag // TODO: Check if this flag is correct in test.sh
	f.AfterReadingAllFlags(&f.TestContext)

	// TODO: Add these to testdriver.go
	flag.StringVar(&CommitId, "commit-id", "local", "commit id will be used to name buckets")
	flag.StringVar(&BucketRegion, "bucket-region", "us-east-1", "region where temporary buckets will be created")
	flag.StringVar(&BucketPrefix, "bucket-prefix", "local", "prefix for temporary buckets")
	flag.Parse()

	// TODO: Add this to s3client.go
	s3client.DefaultRegion = BucketRegion
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Scality S3 CSI Driver E2E Suite")
}

var CSITestSuites = []func() framework.TestSuite{
	testsuites.InitVolumesTestSuite,
}

// This executes testSuites for csi volumes.
var _ = utils.SIGDescribe("CSI Volumes", func() {
	curDriver := initS3Driver()

	args := framework.GetDriverNameWithFeatureTags(curDriver)
	args = append(args, func() {
		framework.DefineTestSuites(curDriver, CSITestSuites)
	})
	f.Context(args...)
})
