package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
)

// CredentialsConfig represents the structure of integration_config.json
type CredentialsConfig struct {
	Credentials struct {
		Account struct {
			Account1 struct {
				Username  string `json:"username"`
				AccessKey string `json:"accessKey"`
				SecretKey string `json:"secretKey"`
			} `json:"account1"`
		} `json:"account"`
	} `json:"credentials"`
}

// DetectClusterType automatically detects if kind or minikube is running
func DetectClusterType() string {
	// Try kind first
	if err := sh.Run("kind", "get", "clusters"); err == nil {
		fmt.Println("Detected kind cluster")
		return "kind"
	}

	// Check if minikube exists and has a running kubelet
	if output, err := sh.Output("minikube", "status"); err == nil || output != "" {
		// Even if status returns error, check if kubelet is running
		if strings.Contains(output, "kubelet: Running") {
			fmt.Println("Detected minikube cluster")
			return "minikube"
		}
	}

	panic("No kind or minikube cluster detected. Please start a cluster first.")
}

// GetNamespace returns the target namespace for installation
func GetNamespace() string {
	if ns := os.Getenv("CSI_NAMESPACE"); ns != "" {
		return ns
	}
	return "default"
}

// GetS3EndpointURL returns the S3 endpoint URL
func GetS3EndpointURL() string {
	if url := os.Getenv("S3_ENDPOINT_URL"); url != "" {
		return url
	}
	return "http://localhost:8000"
}

// GetContainerTag returns the container tag to use
func GetContainerTag() string {
	if tag := os.Getenv("CONTAINER_TAG"); tag != "" {
		return tag
	}
	return "local"
}

// GetContainerImage returns the full container image name
func GetContainerImage() string {
	image := "ghcr.io/scality/mountpoint-s3-csi-driver"
	if customImage := os.Getenv("CONTAINER_IMAGE"); customImage != "" {
		image = customImage
	}
	return fmt.Sprintf("%s:%s", image, GetContainerTag())
}

// LoadCredentialsFromFile loads credentials from integration_config.json
func LoadCredentialsFromFile() (*CredentialsConfig, error) {
	// Get the project root directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %v", err)
	}

	// Path to the integration config file
	configPath := filepath.Join(wd, "tests", "e2e", "integration_config.json")

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", configPath, err)
	}

	// Parse JSON
	var config CredentialsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return &config, nil
}

// Run executes a command and returns the output
func Run(cmd string, args ...string) error {
	return sh.Run(cmd, args...)
}

// RunV executes a command with verbose output
func RunV(cmd string, args ...string) error {
	return sh.RunV(cmd, args...)
}
