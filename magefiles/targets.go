package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/sh"
)

// LoadCredentials loads S3 credentials from integration_config.json and sets environment variables
func LoadCredentials() error {
	fmt.Println("Loading credentials from integration_config.json...")

	config, err := LoadCredentialsFromFile()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %v", err)
	}

	// Validate credentials exist
	if config.Credentials.Account.Account1.AccessKey == "" {
		return fmt.Errorf("account1.accessKey is empty in integration_config.json")
	}
	if config.Credentials.Account.Account1.SecretKey == "" {
		return fmt.Errorf("account1.secretKey is empty in integration_config.json")
	}

	// Set environment variables for later use
	_ = os.Setenv("ACCOUNT1_ACCESS_KEY", config.Credentials.Account.Account1.AccessKey)
	_ = os.Setenv("ACCOUNT1_SECRET_KEY", config.Credentials.Account.Account1.SecretKey)

	fmt.Printf("Credentials loaded for account: %s\n", config.Credentials.Account.Account1.Username)

	return nil
}

// BuildImage builds the CSI driver container image using make
func BuildImage() error {
	fmt.Printf("Building container image: %s\n", GetContainerImage())

	// Build with the correct repository name that matches the helm chart
	// Use quiet docker build for normal mode, verbose for VERBOSE mode
	if os.Getenv("VERBOSE") != "" {
		return sh.RunV("make", "container",
			fmt.Sprintf("CONTAINER_TAG=%s", GetContainerTag()),
			"CONTAINER_IMAGE=ghcr.io/scality/mountpoint-s3-csi-driver")
	} else {
		// Use docker build with --quiet flag for clean output
		image := fmt.Sprintf("ghcr.io/scality/mountpoint-s3-csi-driver:%s", GetContainerTag())
		return sh.RunV("docker", "build", "--quiet", "-t", image, ".")
	}
}

// LoadImageToCluster loads the built image to the detected cluster
func LoadImageToCluster() error {
	clusterType := DetectClusterType()
	image := GetContainerImage()

	fmt.Printf("Loading image %s to %s cluster...\n", image, clusterType)

	switch clusterType {
	case "kind":
		return sh.RunV("kind", "load", "docker-image", image)
	case "minikube":
		return sh.RunV("minikube", "image", "load", image)
	default:
		return fmt.Errorf("unsupported cluster type: %s", clusterType)
	}
}

// CreateSecret creates a Kubernetes secret with S3 credentials
func CreateSecret() error {
	namespace := GetNamespace()

	fmt.Printf("Creating S3 credentials secret in namespace: %s\n", namespace)

	// Load credentials automatically if not already loaded
	accessKey := os.Getenv("ACCOUNT1_ACCESS_KEY")
	secretKey := os.Getenv("ACCOUNT1_SECRET_KEY")

	if accessKey == "" || secretKey == "" {
		fmt.Println("Credentials not loaded, loading from config file...")
		if err := LoadCredentials(); err != nil {
			return fmt.Errorf("failed to load credentials: %v", err)
		}
		accessKey = os.Getenv("ACCOUNT1_ACCESS_KEY")
		secretKey = os.Getenv("ACCOUNT1_SECRET_KEY")
	}

	// Create namespace if it doesn't exist (ignore errors if it already exists)
	_ = sh.Run("kubectl", "create", "namespace", namespace)

	// Delete existing secret if it exists (ignore errors if it doesn't exist)
	_ = sh.Run("kubectl", "delete", "secret", "s3-secret", "-n", namespace)

	// Create the secret directly
	args := []string{
		"create", "secret", "generic", "s3-secret",
		fmt.Sprintf("--from-literal=access_key_id=%s", accessKey),
		fmt.Sprintf("--from-literal=secret_access_key=%s", secretKey),
		"-n", namespace,
	}

	if err := sh.RunV("kubectl", args...); err != nil {
		return fmt.Errorf("failed to create secret: %v", err)
	}

	fmt.Println("S3 credentials secret created/updated")
	return nil
}

// InstallCSI installs the CSI driver using Helm with the local chart
func InstallCSI() error {
	namespace := GetNamespace()
	s3EndpointURL := GetS3EndpointURL()
	imageTag := GetContainerTag()

	fmt.Printf("Installing CSI driver in namespace: %s\n", namespace)
	fmt.Printf("  Image tag: %s\n", imageTag)
	fmt.Printf("  S3 endpoint: %s\n", s3EndpointURL)

	// Helm upgrade --install command
	args := []string{
		"upgrade", "--install", "scality-s3-csi",
		"./charts/scality-mountpoint-s3-csi-driver",
		"--namespace", namespace,
		"--create-namespace",
		"--set", fmt.Sprintf("image.tag=%s", imageTag),
		"--set", "image.repository=ghcr.io/scality/mountpoint-s3-csi-driver",
		"--set", fmt.Sprintf("node.s3EndpointUrl=%s", s3EndpointURL),
		"--wait",
		"--timeout", "300s",
	}

	// Add debug flag if verbose mode is enabled
	if os.Getenv("VERBOSE") != "" {
		args = append(args, "--debug")
	}

	if err := sh.RunV("helm", args...); err != nil {
		return fmt.Errorf("helm install failed: %v", err)
	}

	fmt.Println("CSI driver installed successfully!")

	// Show the status of the installed resources
	fmt.Println("\nCSI Driver Status:")
	_ = sh.RunV("kubectl", "get", "pods", "-n", namespace, "-l", "app.kubernetes.io/name=scality-mountpoint-s3-csi-driver")

	return nil
}

// UninstallCSI removes the CSI driver using Helm
func UninstallCSI() error {
	namespace := GetNamespace()

	fmt.Printf("Uninstalling CSI driver from namespace: %s\n", namespace)

	// Check if the release exists first
	if err := sh.Run("helm", "status", "scality-s3-csi", "-n", namespace); err != nil {
		fmt.Println("CSI driver is not installed or already removed")
		return nil
	}

	// Uninstall the Helm release
	if err := sh.RunV("helm", "uninstall", "scality-s3-csi", "-n", namespace); err != nil {
		return fmt.Errorf("helm uninstall failed: %v", err)
	}

	fmt.Println("CSI driver uninstalled successfully")
	return nil
}

// RemoveSecret removes the S3 credentials secret
func RemoveSecret() error {
	namespace := GetNamespace()

	fmt.Printf("Removing S3 credentials secret from namespace: %s\n", namespace)

	// Check if the secret exists first
	if err := sh.Run("kubectl", "get", "secret", "s3-secret", "-n", namespace); err != nil {
		fmt.Println("S3 credentials secret doesn't exist or already removed")
		return nil
	}

	// Delete the secret
	if err := sh.Run("kubectl", "delete", "secret", "s3-secret", "-n", namespace); err != nil {
		return fmt.Errorf("failed to delete secret: %v", err)
	}

	fmt.Println("S3 credentials secret removed")
	return nil
}

// Status shows the current status of the CSI driver
func Status() error {
	namespace := GetNamespace()

	fmt.Printf("CSI Driver Status in namespace: %s\n", namespace)

	// Check Helm release status
	fmt.Println("\nHelm Release Status:")
	if err := sh.RunV("helm", "status", "scality-s3-csi", "-n", namespace); err != nil {
		fmt.Println("CSI driver is not installed")
	}

	// Check pods
	fmt.Println("\nPod Status:")
	_ = sh.RunV("kubectl", "get", "pods", "-n", namespace, "-l", "app.kubernetes.io/name=scality-mountpoint-s3-csi-driver")

	// Check secret
	fmt.Println("\nSecret Status:")
	if err := sh.RunV("kubectl", "get", "secret", "s3-secret", "-n", namespace); err != nil {
		fmt.Println("S3 credentials secret not found")
	}

	return nil
}
