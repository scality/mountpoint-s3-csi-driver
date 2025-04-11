package e2etests

import (
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E with CloudServer Suite")
}

var _ = BeforeSuite(func() {
	// This will run once before all tests
	GinkgoWriter.Println("Setting up E2E test environment with CloudServer")

	// Run the setup script
	cmd := exec.Command("./scripts/setup_cloudserver.sh")
	output, err := cmd.CombinedOutput()
	GinkgoWriter.Println(string(output))

	// If there's an error, we'll still continue but log it
	if err != nil {
		GinkgoWriter.Printf("Warning: CloudServer setup script returned error: %v\n", err)
		GinkgoWriter.Println("Tests will continue but may fail if CloudServer is not available")
	} else {
		GinkgoWriter.Println("CloudServer setup completed successfully")
	}

	// Wait a bit for CloudServer to stabilize
	time.Sleep(2 * time.Second)
})

var _ = AfterSuite(func() {
	// This will run once after all tests
	GinkgoWriter.Println("Tearing down E2E test environment")

	// Cleanup CloudServer (optional - you might want to keep it running for debugging)
	cmd := exec.Command("docker", "rm", "-f", "s3-cloudserver-test")
	output, err := cmd.CombinedOutput()

	if err != nil {
		GinkgoWriter.Printf("Warning: Failed to clean up CloudServer container: %v\n", err)
		GinkgoWriter.Println(string(output))
	} else {
		GinkgoWriter.Println("CloudServer container removed successfully")
	}
})
