package main

import (
	"fmt"
	"os"
	"strings"

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
	image := GetContainerImage()
	fmt.Printf("Building container image: %s\n", image)

	// Check if image already exists
	if err := sh.Run("docker", "image", "inspect", image); err == nil {
		fmt.Printf("Image %s already exists - overwriting\n", image)
	}

	// Build with the correct repository name that matches the helm chart
	var err error
	if IsVerbose() {
		err = sh.RunV("make", "container",
			fmt.Sprintf("CONTAINER_TAG=%s", GetContainerTag()),
			"CONTAINER_IMAGE=ghcr.io/scality/mountpoint-s3-csi-driver")
	} else {
		// Use docker build with --quiet flag for clean output
		err = sh.RunV("docker", "build", "--quiet", "-t", image, ".")
	}

	if err != nil {
		return fmt.Errorf("failed to build container image %s: %v", image, err)
	}

	fmt.Printf("Container image %s built successfully\n", image)
	return nil
}

// LoadImageToCluster loads the built image to the detected cluster
func LoadImageToCluster() error {
	clusterType := DetectClusterType()
	image := GetContainerImage()

	// Check if image already exists in cluster
	switch clusterType {
	case "minikube":
		if output, err := sh.Output("minikube", "image", "ls", "--format=table"); err == nil {
			if strings.Contains(output, image) {
				fmt.Printf("Image %s already exists in minikube - overwriting\n", image)
			}
		}
	case "kind":
		// Kind doesn't have an easy way to list images, so we'll just proceed
	}

	fmt.Printf("Loading image %s to %s cluster...\n", image, clusterType)

	var err error
	switch clusterType {
	case "kind":
		err = sh.RunV("kind", "load", "docker-image", image)
	case "minikube":
		err = sh.RunV("minikube", "image", "load", image)
	default:
		return fmt.Errorf("unsupported cluster type: %s", clusterType)
	}

	if err != nil {
		return fmt.Errorf("failed to load image to %s cluster: %v", clusterType, err)
	}

	// Verify the image is actually available in the cluster
	fmt.Printf("Verifying image is available in cluster...\n")

	// For kind/minikube, we can check if the image exists
	switch clusterType {
	case "kind":
		if err := sh.Run("kind", "get", "nodes"); err == nil {
			// Try to verify image exists - this is best effort
			fmt.Printf("Image loaded to kind cluster successfully\n")
		}
	case "minikube":
		// Verify with minikube image ls - use plain format for reliable parsing
		if output, err := sh.Output("minikube", "image", "ls"); err == nil {
			// Check if our image repository and tag are both present
			imageRepo := strings.Split(image, ":")[0]
			imageTag := strings.Split(image, ":")[1]

			if strings.Contains(output, imageRepo) && strings.Contains(output, imageTag) {
				fmt.Printf("Image verified in minikube cluster\n")
			} else {
				// More detailed error message
				return fmt.Errorf("image %s not found in minikube after loading (repo: %s, tag: %s)", image, imageRepo, imageTag)
			}
		}
	}

	return nil
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
	if namespace != "default" {
		_ = RunCommand("kubectl", "create", "namespace", namespace)
	}

	// Delete existing secret if it exists (ignore errors if it doesn't exist)
	_ = sh.Run("kubectl", "delete", "secret", "s3-secret", "-n", namespace, "--ignore-not-found=true")

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

	// Verify the secret was actually created and contains the expected keys
	fmt.Printf("Verifying secret was created successfully...\n")
	if err := sh.Run("kubectl", "get", "secret", "s3-secret", "-n", namespace); err != nil {
		return fmt.Errorf("secret verification failed - secret not found: %v", err)
	}

	// Check that the secret has the expected keys
	keys, err := sh.Output("kubectl", "get", "secret", "s3-secret", "-n", namespace, "-o", "jsonpath={.data}")
	if err != nil {
		return fmt.Errorf("failed to get secret data: %v", err)
	}

	if !strings.Contains(keys, "access_key_id") || !strings.Contains(keys, "secret_access_key") {
		return fmt.Errorf("secret missing required keys (access_key_id, secret_access_key)")
	}

	fmt.Println("S3 credentials secret created/updated and verified")
	return nil
}

// InstallCSI installs the CSI driver using Helm with the local chart
func InstallCSI() error {
	namespace := GetNamespace()
	imageTag := GetContainerTag()

	// Configure DNS mapping and use s3.example.com as endpoint
	if err := ConfigureDNS(); err != nil {
		return fmt.Errorf("failed to configure DNS: %v", err)
	}

	s3EndpointURL := GetS3EndpointURL()

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

	fmt.Println("Helm installation completed. Verifying CSI driver deployment...")

	// Wait for pods to be ready
	fmt.Println("Waiting for CSI driver pods to become ready...")

	// Wait for controller pods
	if err := sh.RunV("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app=s3-csi-controller", "-n", namespace, "--timeout=120s"); err != nil {
		fmt.Printf("Warning: Controller pods not ready: %v\n", err)
	}

	// Wait for node pods
	if err := sh.RunV("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app=s3-csi-node", "-n", namespace, "--timeout=120s"); err != nil {
		fmt.Printf("Warning: Node pods not ready: %v\n", err)
	}

	// Verify pods are actually running
	fmt.Println("\nVerifying CSI driver pods are running:")
	if err := sh.RunV("kubectl", "get", "pods", "-n", namespace, "-l", "app.kubernetes.io/name=scality-mountpoint-s3-csi-driver"); err != nil {
		return fmt.Errorf("failed to get CSI driver pods: %v", err)
	}

	// Check if any pods are in error state
	if output, err := sh.Output("kubectl", "get", "pods", "-n", namespace,
		"-l", "app.kubernetes.io/name=scality-mountpoint-s3-csi-driver",
		"--field-selector=status.phase!=Running", "-o", "name"); err == nil && strings.TrimSpace(output) != "" {
		fmt.Printf("Warning: Some CSI driver pods are not running:\n")
		_ = sh.RunV("kubectl", "get", "pods", "-n", namespace, "-l", "app.kubernetes.io/name=scality-mountpoint-s3-csi-driver")
	}

	fmt.Println("CSI driver deployment verification completed!")
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

// ConfigureDNS configures Kubernetes DNS to map s3.example.com to the S3 endpoint
func ConfigureDNS() error {
	s3Host := GetS3Host()
	_ = GetS3MappingTarget() // Get the full URL for potential future use

	fmt.Printf("Configuring DNS: s3.example.com -> %s\n", s3Host)

	// Backup original CoreDNS config if verbose
	if IsVerbose() {
		fmt.Println("Backing up original CoreDNS configuration...")
		if err := sh.RunV("kubectl", "get", "configmap", "coredns", "-n", "kube-system", "-o", "yaml"); err != nil {
			return fmt.Errorf("failed to backup CoreDNS config: %v", err)
		}
		fmt.Println("Configuring s3.example.com in CoreDNS hosts block...")
	}

	// Update CoreDNS configuration
	if err := UpdateCoreDNSHosts(s3Host, false); err != nil {
		return fmt.Errorf("failed to update CoreDNS config: %v", err)
	}

	// Restart CoreDNS to pick up changes
	if err := RestartCoreDNS(); err != nil {
		return err
	}

	fmt.Printf("DNS configured successfully! s3.example.com now points to %s\n", s3Host)
	return nil
}

// RemoveDNS removes the s3.example.com DNS mapping from CoreDNS
func RemoveDNS() error {
	fmt.Println("Removing s3.example.com DNS mapping...")

	// Remove s3.example.com entries from CoreDNS configuration
	if err := UpdateCoreDNSHosts("", true); err != nil {
		return fmt.Errorf("failed to clean CoreDNS config: %v", err)
	}

	// Restart CoreDNS to pick up changes
	if err := RestartCoreDNS(); err != nil {
		return err
	}

	fmt.Println("DNS mapping removed successfully")
	return nil
}

// InstallCSIWithVersion installs CSI from OCI registry (for mage install command)
func InstallCSIWithVersion() error {
	chartVersion := GetCSIChartVersion()

	// Version should already be checked in Install(), but double-check here
	if chartVersion == "" {
		return fmt.Errorf("SCALITY_CSI_VERSION environment variable is required")
	}

	// Verify chart version exists
	fmt.Printf("Verifying chart version %s exists in registry...\n", chartVersion)
	chartPath := "oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver"
	if err := sh.Run("helm", "show", "chart", chartPath, "--version", chartVersion); err != nil {
		return fmt.Errorf("chart version %s not found in registry\n\n"+
			"Available versions can be checked with:\n"+
			"  helm search repo %s --versions\n\n"+
			"Error: %v", chartVersion, chartPath, err)
	}

	namespace := GetNamespace()

	// Configure DNS mapping
	if err := ConfigureDNS(); err != nil {
		return fmt.Errorf("failed to configure DNS: %v", err)
	}

	s3EndpointURL := GetS3EndpointURL()

	fmt.Printf("Installing CSI driver in namespace: %s\n", namespace)
	fmt.Printf("  Chart version: %s (from OCI registry)\n", chartVersion)
	fmt.Printf("  Using published images from version %s\n", chartVersion)
	fmt.Printf("  S3 endpoint: %s\n", s3EndpointURL)

	// Build helm args - NOTE: Same release name "scality-s3-csi" as InstallCSI
	args := []string{
		"upgrade", "--install", "scality-s3-csi",
		chartPath,
		"--version", chartVersion,
		"--namespace", namespace,
		"--create-namespace",
		"--set", fmt.Sprintf("node.s3EndpointUrl=%s", s3EndpointURL),
		"--wait",
		"--timeout", "300s",
	}

	if IsVerbose() {
		args = append(args, "--debug")
	}

	if err := sh.RunV("helm", args...); err != nil {
		return fmt.Errorf("helm install failed: %v", err)
	}

	// Verification code (same as InstallCSI)
	fmt.Println("Helm installation completed. Verifying CSI driver deployment...")

	// Wait for pods to be ready
	fmt.Println("Waiting for CSI driver pods to become ready...")

	// Wait for controller pods
	if err := sh.RunV("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app=s3-csi-controller", "-n", namespace, "--timeout=120s"); err != nil {
		fmt.Printf("Warning: Controller pods not ready: %v\n", err)
	}

	// Wait for node pods
	if err := sh.RunV("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app=s3-csi-node", "-n", namespace, "--timeout=120s"); err != nil {
		fmt.Printf("Warning: Node pods not ready: %v\n", err)
	}

	// Verify pods are running
	fmt.Println("\nVerifying CSI driver pods are running:")
	if err := sh.RunV("kubectl", "get", "pods", "-n", namespace,
		"-l", "app.kubernetes.io/name=scality-mountpoint-s3-csi-driver"); err != nil {
		return fmt.Errorf("failed to get CSI driver pods: %v", err)
	}

	fmt.Printf("CSI driver version %s installed successfully!\n", chartVersion)
	return nil
}

// ShowS3DNS shows the current S3 DNS configuration
func ShowS3DNS() error {
	fmt.Println("Current S3 DNS Configuration:")
	fmt.Println("=============================")

	// Get the current CoreDNS configuration
	output, err := sh.Output("kubectl", "get", "configmap", "coredns", "-n", "kube-system", "-o", "jsonpath={.data.Corefile}")
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS config: %v", err)
	}

	// Check if s3.example.com is configured
	if strings.Contains(output, "s3.example.com") {
		// Extract the IP address for s3.example.com
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "s3.example.com") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					ip := parts[0]
					fmt.Printf("s3.example.com is mapped to: %s\n", ip)
				}
				break
			}
		}

		// Test current DNS resolution
		fmt.Println("\nTesting current DNS resolution...")
		if err := sh.RunV("kubectl", "run", "dns-show-test", "--image=busybox:1.36", "--rm", "-i", "--restart=Never",
			"--", "nslookup", "s3.example.com"); err != nil {
			fmt.Println("S3 service DNS resolution failed - check if the target IP is reachable or S3 service is running")
		} else {
			fmt.Println("DNS resolution is working")
		}

		// Test S3 endpoint connectivity
		fmt.Println("\nTesting S3 endpoint connectivity...")
		s3EndpointURL := GetS3EndpointURL()
		fmt.Printf("Testing endpoint: %s\n", s3EndpointURL)

		// Use curl to test the S3 endpoint - we expect either success or AccessDenied
		// Include -w to get HTTP status code and use -o to separate response body
		testOutput, testErr := sh.Output("kubectl", "run", "s3-test", "--image=curlimages/curl:latest", "--rm", "-i", "--restart=Never",
			"--", "sh", "-c", fmt.Sprintf("curl -s -S -X GET '%s' -w '\\nHTTP_CODE:%%{http_code}\\n'", s3EndpointURL))

		// Combine output and error for parsing (kubectl may put output in error for non-zero exit codes)
		fullOutput := testOutput
		if testErr != nil && strings.Contains(testErr.Error(), "HTTP_CODE:") {
			fullOutput = testErr.Error()
		}

		// Extract HTTP status code
		httpCode := ""
		if idx := strings.Index(fullOutput, "HTTP_CODE:"); idx != -1 {
			codeStr := fullOutput[idx+10:]
			if endIdx := strings.Index(codeStr, "\n"); endIdx != -1 {
				httpCode = codeStr[:endIdx]
			} else {
				httpCode = codeStr
			}
		}

		// Check for S3 XML error response
		if strings.Contains(fullOutput, "<?xml") && strings.Contains(fullOutput, "<Error>") {
			// Parse the XML error code
			if strings.Contains(fullOutput, "<Code>AccessDenied</Code>") {
				fmt.Println("✓ S3 endpoint is reachable (returned AccessDenied - expected without credentials)")
			} else if strings.Contains(fullOutput, "<Code>InvalidAccessKeyId</Code>") {
				fmt.Println("✓ S3 endpoint is reachable (returned InvalidAccessKeyId - expected without credentials)")
			} else if strings.Contains(fullOutput, "<Code>SignatureDoesNotMatch</Code>") {
				fmt.Println("✓ S3 endpoint is reachable (returned SignatureDoesNotMatch - expected without credentials)")
			} else if codeIdx := strings.Index(fullOutput, "<Code>"); codeIdx != -1 {
				// Extract any other error code
				codeStart := codeIdx + 6
				if codeEnd := strings.Index(fullOutput[codeStart:], "</Code>"); codeEnd != -1 {
					errorCode := fullOutput[codeStart : codeStart+codeEnd]
					fmt.Printf("✓ S3 endpoint is reachable (returned %s error)\n", errorCode)
				} else {
					fmt.Println("✓ S3 endpoint is reachable (returned S3 error response)")
				}
			}
		} else if httpCode == "403" || httpCode == "401" {
			fmt.Printf("✓ S3 endpoint is reachable (HTTP %s - expected without credentials)\n", httpCode)
		} else if httpCode == "200" {
			fmt.Println("✓ S3 endpoint is reachable (HTTP 200 OK)")
		} else if httpCode != "" && strings.HasPrefix(httpCode, "4") {
			fmt.Printf("✓ S3 endpoint is reachable (HTTP %s)\n", httpCode)
		} else if httpCode != "" && strings.HasPrefix(httpCode, "5") {
			fmt.Printf("⚠ S3 endpoint returned server error (HTTP %s)\n", httpCode)
		} else if testErr != nil {
			// Connection failed
			fmt.Printf("✗ S3 endpoint connectivity test failed\n")
			fmt.Println("  This could mean:")
			fmt.Println("  - The S3 service is not running")
			fmt.Println("  - The endpoint URL is incorrect")
			fmt.Println("  - Network connectivity issues")
			if IsVerbose() {
				fmt.Printf("  Error details: %v\n", testErr)
			}
		} else {
			fmt.Printf("? S3 endpoint returned unexpected response\n")
			if IsVerbose() {
				fmt.Printf("  Response: %s\n", fullOutput)
			}
		}
	} else {
		fmt.Println("s3.example.com is not configured in CoreDNS")
		fmt.Println("   Run 'mage configureS3DNS' to set up the mapping")
	}

	// Show CSI driver configuration if installed
	namespace := GetNamespace()
	if err := sh.Run("helm", "status", "scality-s3-csi", "-n", namespace); err == nil {
		fmt.Println("\nCSI Driver Configuration:")

		// Get the actual AWS_ENDPOINT_URL from the CSI node pod
		nodeOutput, err := sh.Output("kubectl", "get", "pods", "-n", namespace, "-l", "app=s3-csi-node", "-o", "name")
		if err == nil && nodeOutput != "" {
			podName := strings.TrimPrefix(strings.TrimSpace(nodeOutput), "pod/")
			if podName != "" {
				// Get AWS_ENDPOINT_URL from the pod environment (this is the S3 endpoint URL)
				awsEndpoint, err := sh.Output("kubectl", "exec", "-n", namespace, podName, "-c", "s3-plugin", "--", "printenv", "AWS_ENDPOINT_URL")
				if err == nil {
					fmt.Printf("  S3 endpoint in pod (AWS_ENDPOINT_URL): %s\n", strings.TrimSpace(awsEndpoint))
				} else {
					fmt.Printf("  Expected endpoint: %s\n", GetS3EndpointURL())
				}
			}
		} else {
			fmt.Printf("  Expected endpoint: %s\n", GetS3EndpointURL())
		}
	}

	return nil
}
