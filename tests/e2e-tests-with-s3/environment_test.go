package e2etests

import (
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Environment Setup", func() {
	It("should have all required environment variables", func() {
		By("Checking E2E_KUBECONFIG environment variable")
		kubeconfig := os.Getenv("E2E_KUBECONFIG")
		GinkgoWriter.Printf("E2E_KUBECONFIG is set to: %s\n", kubeconfig)

		// We don't fail if it's empty; it might be using default kubeconfig
		if kubeconfig == "" {
			GinkgoWriter.Println("Warning: E2E_KUBECONFIG is not set, tests will use default kubeconfig")
		}
	})

	It("should have CloudServer running and accessible", func() {
		By("Checking CloudServer endpoint")
		endpoint := "http://localhost:8000"
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		// Try a few times as CloudServer might need time to start up
		Eventually(func() bool {
			resp, err := client.Get(endpoint)
			if err != nil {
				GinkgoWriter.Printf("Error connecting to CloudServer: %v\n", err)
				return false
			}
			defer resp.Body.Close()

			GinkgoWriter.Printf("CloudServer responded with status: %s\n", resp.Status)
			return resp.StatusCode == http.StatusOK ||
				resp.StatusCode == http.StatusForbidden || // S3 might return 403 for unauthorized requests
				resp.StatusCode == http.StatusBadRequest // 400 is common for invalid S3 requests
		}, "10s", "1s").Should(BeTrue(), "CloudServer should be accessible")
	})
})
