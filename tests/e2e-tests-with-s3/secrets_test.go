package e2etests

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kubernetes Secret Management", func() {
	const (
		secretName      = "s3-test-secret"
		secretNamespace = "default"
	)

	// Helper function to delete the secret
	// Ignores errors if the secret doesn't exist
	deleteSecret := func() {
		cmd := exec.Command("kubectl", "delete", "secret", secretName,
			"--namespace", secretNamespace, "--ignore-not-found")
		_, _ = cmd.CombinedOutput()
	}

	BeforeEach(func() {
		// Ensure the secret doesn't exist before each test
		deleteSecret()
	})

	AfterEach(func() {
		// Clean up after test
		deleteSecret()
	})

	It("should create and delete a secret for S3 credentials", func() {
		By("Creating a secret with S3 credentials")
		cmd := exec.Command("kubectl", "create", "secret", "generic", secretName,
			"--namespace", secretNamespace,
			"--from-literal=accessKey=testAccessKey",
			"--from-literal=secretKey=testSecretKey")

		output, err := cmd.CombinedOutput()
		GinkgoWriter.Println(string(output))
		Expect(err).NotTo(HaveOccurred(), "Failed to create secret")

		By("Verifying the secret exists")
		cmd = exec.Command("kubectl", "get", "secret", secretName, "--namespace", secretNamespace)
		output, err = cmd.CombinedOutput()
		GinkgoWriter.Println(string(output))
		Expect(err).NotTo(HaveOccurred(), "Secret should exist")

		By("Deleting the secret")
		cmd = exec.Command("kubectl", "delete", "secret", secretName, "--namespace", secretNamespace)
		output, err = cmd.CombinedOutput()
		GinkgoWriter.Println(string(output))
		Expect(err).NotTo(HaveOccurred(), "Failed to delete secret")

		By("Verifying the secret is deleted")
		// Wait briefly for deletion to propagate
		time.Sleep(1 * time.Second)

		cmd = exec.Command("kubectl", "get", "secret", secretName, "--namespace", secretNamespace)
		output, err = cmd.CombinedOutput()
		GinkgoWriter.Println(string(output))
		Expect(err).To(HaveOccurred(), "Secret should be deleted")
	})
})
