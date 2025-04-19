package e2e

import (
	"flag"
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/config"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

var (
	// Kubectl path for executing kubectl commands
	KubectlPath string
	// Kubeconfig path for accessing the Kubernetes cluster
	Kubeconfig string
	// S3EndpointURL for the Scality S3 server
	S3EndpointURL string
	// AccessKeyID for authenticating with the S3 server
	AccessKeyID string
	// SecretAccessKey for authenticating with the S3 server
	SecretAccessKey string
	// BucketPrefix for creating unique bucket names
	BucketPrefix string
	// PerformanceTests flag to enable performance tests
	PerformanceTests bool
	// CleanupAfterTest flag to enable or disable cleanup after tests
	CleanupAfterTest bool
)

// Register the flags for the test execution
func init() {
	flag.StringVar(&KubectlPath, "kubectl-path", "", "The path to the kubectl binary")
	flag.StringVar(&Kubeconfig, "kubeconfig", "", "The path to the kubeconfig file")
	flag.StringVar(&S3EndpointURL, "s3-endpoint-url", "", "The endpoint URL for the Scality S3 server")
	flag.StringVar(&AccessKeyID, "access-key-id", "", "The access key ID for the S3 server")
	flag.StringVar(&SecretAccessKey, "secret-access-key", "", "The secret access key for the S3 server")
	flag.StringVar(&BucketPrefix, "bucket-prefix", "e2e-test", "Prefix for the S3 bucket names")
	flag.BoolVar(&PerformanceTests, "performance", false, "Enable performance tests")
	flag.BoolVar(&CleanupAfterTest, "cleanup", true, "Enable cleanup after tests")

	// Register kubernetes framework flags
	config.CopyFlags(config.Flags, flag.CommandLine)
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)
}

// TestE2E is the main entry point for the e2e tests
func TestE2E(t *testing.T) {
	// Parse command line flags
	if err := flag.Parse(); err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Register the fail handler to handle test failures
	gomega.RegisterFailHandler(ginkgo.Fail)

	// Set kubectl path in the framework config
	if KubectlPath != "" {
		if err := os.Setenv("TEST_KUBECTL", KubectlPath); err != nil {
			t.Fatalf("Failed to set TEST_KUBECTL environment variable: %v", err)
		}
	}

	// Required before running any tests
	if err := flag.Set("kubeconfig", Kubeconfig); err != nil {
		t.Fatalf("Error setting kubeconfig flag: %v", err)
	}

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
	if KubectlPath == "" {
		t.Fatalf("kubectl-path is required")
	}

	// Run the tests
	ginkgo.RunSpecs(t, "Scality S3 CSI Driver E2E Suite")
}

// ScalityTestSuites lists all test suites that will be run
var ScalityTestSuites = []ginkgo.NodeToBeRun{
	// Basic volume tests
	ginkgo.NodeToBeRun{
		Text: "Scality S3 CSI Driver",
		Nodes: []ginkgo.NodeToBeRun{
			{Text: "Basic Volume Tests"},
			{Text: "Static Provisioning"},
			{Text: "Dynamic Provisioning"},
		},
	},
	// Performance tests (if enabled)
	ginkgo.NodeToBeRun{
		Text: "Performance",
		Nodes: []ginkgo.NodeToBeRun{
			{Text: "Read/Write Performance"},
		},
	},
}

// StandardTestSuites lists the Kubernetes storage test suites to run
var StandardTestSuites = []func() storageframework.TestSuite{
	testsuites.InitVolumesTestSuite,
	testsuites.InitVolumeIOTestSuite,
	testsuites.InitVolumeModeTestSuite,
	testsuites.InitSubPathTestSuite,
	testsuites.InitProvisioningTestSuite,
	testsuites.InitMultiVolumeTestSuite,
}

// Define the context for running the test suites
var _ = ginkgo.Describe("Scality S3 CSI Driver", func() {
	// Initialize test driver
	driver, err := InitScalityDriver(&s3client.Config{
		EndpointURL:     S3EndpointURL,
		AccessKeyID:     AccessKeyID,
		SecretAccessKey: SecretAccessKey,
		BucketPrefix:    BucketPrefix,
	})

	ginkgo.BeforeEach(func() {
		if err != nil {
			framework.Failf("Failed to initialize test driver: %v", err)
		}
	})

	// Run standard test suites
	ginkgo.Context("Kubernetes Storage Suite", func() {
		storageframework.DefineTestSuites(driver, StandardTestSuites)
	})

	// Run performance tests if enabled
	if PerformanceTests {
		ginkgo.Context("Performance Tests", func() {
			ginkgo.It("should be able to read and write data efficiently", func() {
				// Performance tests will be implemented in a future phase
				framework.Logf("Performance tests are not yet implemented")
			})
		})
	}
})
