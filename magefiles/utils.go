package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/magefile/mage/sh"
)

// CredentialsConfig represents the structure of integration_config.json
type CredentialsConfig struct {
	Credentials struct {
		Account struct {
			Account1 struct {
				Username    string `json:"username"`
				AccessKey   string `json:"accessKey"`
				SecretKey   string `json:"secretKey"`
				CanonicalID string `json:"canonicalId"`
			} `json:"account1"`
			Account2 struct {
				Username    string `json:"username"`
				AccessKey   string `json:"accessKey"`
				SecretKey   string `json:"secretKey"`
				CanonicalID string `json:"canonicalId"`
			} `json:"account2"`
		} `json:"account"`
	} `json:"credentials"`
}

// DetectClusterType automatically detects if kind or minikube is running
func DetectClusterType() string {
	// Check kubectl context first to ensure we have cluster access
	context, err := sh.Output("kubectl", "config", "current-context")
	if err != nil {
		panic("No kubectl context found. Please ensure kubectl is configured and cluster is accessible.")
	}

	context = strings.TrimSpace(context)

	// Detect based on context name patterns
	if strings.Contains(context, "kind-") {
		fmt.Printf("Detected kind cluster (context: %s)\n", context)
		return "kind"
	}

	if strings.Contains(context, "minikube") {
		fmt.Printf("Detected minikube cluster (context: %s)\n", context)
		return "minikube"
	}

	// If context doesn't match patterns, try to detect by checking available tools
	if err := sh.Run("kind", "get", "clusters"); err == nil {
		fmt.Printf("Found kind clusters, using kind (context: %s)\n", context)
		return "kind"
	}

	if err := sh.Run("minikube", "status"); err == nil {
		fmt.Printf("Found minikube, using minikube (context: %s)\n", context)
		return "minikube"
	}

	panic(fmt.Sprintf("Could not determine cluster type for context '%s'. Please ensure kind or minikube is running.", context))
}

// GetNamespace returns the target namespace for installation
func GetNamespace() string {
	if ns := os.Getenv("CSI_NAMESPACE"); ns != "" {
		return ns
	}
	return "default"
}

// GetS3Config returns S3 configuration (host, port, full URL)
func GetS3Config() (host, port, fullURL string) {
	// Priority 1: Complete URL provided
	if url := os.Getenv("S3_ENDPOINT_URL"); url != "" {
		// Extract protocol from URL
		isHTTPS := strings.HasPrefix(url, "https://")

		// Remove protocol prefix
		urlStr := url
		urlStr = strings.TrimPrefix(urlStr, "http://")
		urlStr = strings.TrimPrefix(urlStr, "https://")

		// Extract host and port
		if strings.Contains(urlStr, ":") {
			parts := strings.Split(urlStr, ":")
			host = parts[0]
			port = parts[1]
		} else {
			// No port specified in URL, use default based on protocol
			host = urlStr
			if isHTTPS {
				port = "443"
			} else {
				port = "80"
			}
		}

		// Return the original URL as provided
		return host, port, url
	}

	// Priority 2: Component-based configuration (S3_HOST + S3_PORT + S3_HTTPS)
	host = os.Getenv("S3_HOST")
	if host == "" {
		host = "localhost"
	}

	port = os.Getenv("S3_PORT")
	if port == "" {
		port = "8000"
	}

	// Determine protocol - default is HTTP unless S3_HTTPS=true
	protocol := "http"
	if os.Getenv("S3_HTTPS") == "true" {
		protocol = "https"
	}

	fullURL = fmt.Sprintf("%s://%s:%s", protocol, host, port)
	return host, port, fullURL
}

// GetS3MappingTarget returns the complete S3 endpoint URL for DNS mapping
func GetS3MappingTarget() string {
	_, _, fullURL := GetS3Config()
	return fullURL
}

// GetS3Host returns just the S3 host/IP for DNS mapping
func GetS3Host() string {
	host, _, _ := GetS3Config()
	return host
}

// GetS3Port returns the S3 port
func GetS3Port() string {
	_, port, _ := GetS3Config()
	return port
}

// GetS3EndpointURL returns the full S3 endpoint URL for CSI driver
func GetS3EndpointURL() string {
	_, port, originalURL := GetS3Config()

	// If S3_ENDPOINT_URL is provided, return it directly
	if url := os.Getenv("S3_ENDPOINT_URL"); url != "" {
		return url
	}

	// Extract protocol from original configuration
	protocol := "http"
	if strings.HasPrefix(originalURL, "https://") {
		protocol = "https"
	}

	return fmt.Sprintf("%s://s3.example.com:%s", protocol, port)
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

// GetCSIChartVersion returns the CSI chart version to install
func GetCSIChartVersion() string {
	if version := os.Getenv("SCALITY_CSI_VERSION"); version != "" {
		return version
	}
	return "" // Empty means no version specified
}

// IsDNSConfigured checks if s3.example.com is already configured in CoreDNS
func IsDNSConfigured() bool {
	// Get the current CoreDNS configuration
	output, err := sh.Output("kubectl", "get", "configmap", "coredns", "-n", "kube-system", "-o", "jsonpath={.data.Corefile}")
	if err != nil {
		// If we can't check, assume not configured
		return false
	}

	// Check if s3.example.com is already in the configuration
	return strings.Contains(output, "s3.example.com")
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

// RunCommand runs a command with verbose output if VERBOSE env var is set
func RunCommand(command string, args ...string) error {
	if IsVerbose() {
		return sh.RunV(command, args...)
	}
	return sh.Run(command, args...)
}

// IsVerbose returns true if verbose mode is enabled
func IsVerbose() bool {
	return os.Getenv("VERBOSE") != ""
}

// RestartCoreDNS restarts CoreDNS deployment and waits for it to be ready
func RestartCoreDNS() error {
	if IsVerbose() {
		fmt.Println("Restarting CoreDNS...")
	}

	if err := RunCommand("kubectl", "rollout", "restart", "deployment", "coredns", "-n", "kube-system"); err != nil {
		return fmt.Errorf("failed to restart CoreDNS: %v", err)
	}

	if IsVerbose() {
		fmt.Println("Waiting for CoreDNS to be ready...")
	}

	if err := RunCommand("kubectl", "rollout", "status", "deployment", "coredns", "-n", "kube-system", "--timeout=60s"); err != nil {
		return fmt.Errorf("CoreDNS restart timed out: %v", err)
	}

	return nil
}

// UpdateCoreDNSHosts updates CoreDNS configuration to add/remove s3.example.com mapping
func UpdateCoreDNSHosts(s3Host string, remove bool) error {
	// Get current config
	currentConfig, err := sh.Output("kubectl", "get", "configmap", "coredns", "-n", "kube-system", "-o", "jsonpath={.data.Corefile}")
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS config: %v", err)
	}

	// Remove existing s3.example.com entries
	lines := strings.Split(currentConfig, "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, "s3.example.com") {
			filtered = append(filtered, line)
		}
	}
	newConfig := strings.Join(filtered, "\n")

	// If not removing, add the new entry
	if !remove {
		if strings.Contains(newConfig, "hosts {") {
			// Add to existing hosts block
			newConfig = strings.ReplaceAll(newConfig, "hosts {", fmt.Sprintf("hosts {\n        %s s3.example.com", s3Host))
		} else {
			// Create new hosts block before final }
			newConfig = strings.ReplaceAll(newConfig, "\n}", fmt.Sprintf("\n    hosts {\n        %s s3.example.com\n        fallthrough\n    }\n}", s3Host))
		}
	}

	// Patch the ConfigMap
	patchData := fmt.Sprintf(`{"data":{"Corefile":%q}}`, newConfig)
	return sh.RunV("kubectl", "patch", "configmap", "coredns", "-n", "kube-system", "--type=merge", "-p", patchData)
}

// ResourceChecker provides robust resource checking with proper error handling
type ResourceChecker struct {
	Namespace   string
	VerboseMode bool
}

// NewResourceChecker creates a new ResourceChecker instance
func NewResourceChecker(namespace string) *ResourceChecker {
	return &ResourceChecker{
		Namespace:   namespace,
		VerboseMode: IsVerbose(),
	}
}

// WaitForResource waits for a Kubernetes resource to meet a specific condition
func (rc *ResourceChecker) WaitForResource(resourceType, name, condition string, timeout time.Duration) error {
	if rc.VerboseMode {
		fmt.Printf("Waiting for %s/%s to meet condition '%s' (timeout: %v)...\n", resourceType, name, condition, timeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Check if resource exists first
	exists, err := rc.ResourceExists(resourceType, name)
	if err != nil {
		return fmt.Errorf("failed to check if %s/%s exists: %v", resourceType, name, err)
	}
	if !exists {
		return fmt.Errorf("%s/%s does not exist", resourceType, name)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Get current status for debugging
			if status, err := rc.GetResourceStatus(resourceType, name); err == nil {
				return fmt.Errorf("timeout waiting for %s/%s to meet condition '%s'. Current status: %s", resourceType, name, condition, status)
			}
			return fmt.Errorf("timeout waiting for %s/%s to meet condition '%s'", resourceType, name, condition)

		case <-ticker.C:
			// Check if condition is met
			if met, err := rc.CheckCondition(resourceType, name, condition); err != nil {
				if rc.VerboseMode {
					fmt.Printf("Error checking condition for %s/%s: %v\n", resourceType, name, err)
				}
				continue
			} else if met {
				if rc.VerboseMode {
					fmt.Printf("✓ %s/%s condition '%s' met\n", resourceType, name, condition)
				}
				return nil
			}
		}
	}
}

// ResourceExists checks if a resource exists
func (rc *ResourceChecker) ResourceExists(resourceType, name string) (bool, error) {
	args := []string{"get", resourceType, name}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}
	args = append(args, "--ignore-not-found=true", "-o", "name")

	output, err := sh.Output("kubectl", args...)
	if err != nil {
		// If kubectl command fails for other reasons, return error
		return false, err
	}

	// If resource exists, output will contain the resource name
	return strings.TrimSpace(output) != "", nil
}

// GetResourceStatus gets the current status of a resource for debugging
func (rc *ResourceChecker) GetResourceStatus(resourceType, name string) (string, error) {
	args := []string{"get", resourceType, name}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}
	args = append(args, "-o", "jsonpath={.status.phase}")

	output, err := sh.Output("kubectl", args...)
	if err != nil {
		// Try alternative status fields
		alternatives := []string{
			"{.status.conditions[?(@.type=='Ready')].status}",
			"{.status.readyReplicas}",
			"{.status}",
		}

		for _, alt := range alternatives {
			args[len(args)-1] = fmt.Sprintf("-o=jsonpath=%s", alt)
			if output, err = sh.Output("kubectl", args...); err == nil && strings.TrimSpace(output) != "" {
				break
			}
		}
	}

	return strings.TrimSpace(output), err
}

// CheckCondition checks if a resource meets a specific condition
func (rc *ResourceChecker) CheckCondition(resourceType, name, condition string) (bool, error) {
	switch condition {
	case "ready":
		return rc.checkReadyCondition(resourceType, name)
	case "bound":
		return rc.checkBoundCondition(resourceType, name)
	case "running":
		return rc.checkRunningCondition(resourceType, name)
	default:
		return rc.checkCustomCondition(resourceType, name, condition)
	}
}

// checkReadyCondition checks if a resource is ready
func (rc *ResourceChecker) checkReadyCondition(resourceType, name string) (bool, error) {
	args := []string{"get", resourceType, name}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}

	switch resourceType {
	case "pod", "pods":
		args = append(args, "-o", "jsonpath={.status.phase}")
		output, err := sh.Output("kubectl", args...)
		return strings.TrimSpace(output) == "Running", err

	case "deployment", "deployments":
		args = append(args, "-o", "jsonpath={.status.readyReplicas}")
		output, err := sh.Output("kubectl", args...)
		if err != nil {
			return false, err
		}
		// Also check desired replicas
		args[len(args)-1] = "-o=jsonpath={.status.replicas}"
		desired, err := sh.Output("kubectl", args...)
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(output) == strings.TrimSpace(desired) && strings.TrimSpace(output) != "0", nil

	default:
		// Generic ready condition check
		args = append(args, "-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		output, err := sh.Output("kubectl", args...)
		return strings.TrimSpace(output) == "True", err
	}
}

// checkBoundCondition checks if a PVC is bound
func (rc *ResourceChecker) checkBoundCondition(resourceType, name string) (bool, error) {
	if resourceType != "pvc" && resourceType != "persistentvolumeclaim" {
		return false, fmt.Errorf("bound condition only applies to PVCs")
	}

	args := []string{"get", "pvc", name}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}
	args = append(args, "-o", "jsonpath={.status.phase}")

	output, err := sh.Output("kubectl", args...)
	return strings.TrimSpace(output) == "Bound", err
}

// checkRunningCondition checks if pods are running
func (rc *ResourceChecker) checkRunningCondition(resourceType, name string) (bool, error) {
	if resourceType != "pod" && resourceType != "pods" {
		return false, fmt.Errorf("running condition only applies to pods")
	}

	args := []string{"get", "pod", name}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}
	args = append(args, "-o", "jsonpath={.status.phase}")

	output, err := sh.Output("kubectl", args...)
	return strings.TrimSpace(output) == "Running", err
}

// checkCustomCondition checks custom conditions using kubectl wait
func (rc *ResourceChecker) checkCustomCondition(resourceType, name, condition string) (bool, error) {
	args := []string{"wait", "--for=" + condition, resourceType + "/" + name}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}
	args = append(args, "--timeout=1s")

	err := sh.Run("kubectl", args...)
	return err == nil, nil
}

// WaitForPodsWithLabel waits for pods matching a label selector to be ready
func (rc *ResourceChecker) WaitForPodsWithLabel(labelSelector string, timeout time.Duration) error {
	if rc.VerboseMode {
		fmt.Printf("Waiting for pods with label '%s' to be ready (timeout: %v)...\n", labelSelector, timeout)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Get current pod status for debugging
			if status := rc.GetPodsStatus(labelSelector); status != "" {
				return fmt.Errorf("timeout waiting for pods with label '%s' to be ready. Current status:\n%s", labelSelector, status)
			}
			return fmt.Errorf("timeout waiting for pods with label '%s' to be ready", labelSelector)

		case <-ticker.C:
			if ready, err := rc.ArePodsReady(labelSelector); err != nil {
				if rc.VerboseMode {
					fmt.Printf("Error checking pod readiness: %v\n", err)
				}
				continue
			} else if ready {
				if rc.VerboseMode {
					fmt.Printf("✓ All pods with label '%s' are ready\n", labelSelector)
				}
				return nil
			}
		}
	}
}

// ArePodsReady checks if all pods matching a label selector are ready
func (rc *ResourceChecker) ArePodsReady(labelSelector string) (bool, error) {
	args := []string{"get", "pods", "-l", labelSelector}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}
	args = append(args, "-o", "jsonpath={.items[*].status.phase}")

	output, err := sh.Output("kubectl", args...)
	if err != nil {
		return false, err
	}

	phases := strings.Fields(strings.TrimSpace(output))
	if len(phases) == 0 {
		return false, fmt.Errorf("no pods found matching label selector '%s'", labelSelector)
	}

	for _, phase := range phases {
		if phase != "Running" {
			return false, nil
		}
	}

	return true, nil
}

// GetPodsStatus gets detailed status of pods for debugging
func (rc *ResourceChecker) GetPodsStatus(labelSelector string) string {
	args := []string{"get", "pods", "-l", labelSelector}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}
	args = append(args, "-o", "wide")

	output, _ := sh.Output("kubectl", args...)
	return output
}

// CountPodsInNamespace counts pods in a specific namespace
func (rc *ResourceChecker) CountPodsInNamespace(namespace string) (int, error) {
	args := []string{"get", "pods", "-n", namespace, "--no-headers"}

	output, err := sh.Output("kubectl", args...)
	if err != nil {
		// If namespace doesn't exist, return 0
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "No resources found") {
			return 0, nil
		}
		return 0, err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0, nil
	}

	return len(lines), nil
}

// NamespaceExists checks if a namespace exists
func (rc *ResourceChecker) NamespaceExists(namespace string) (bool, error) {
	output, err := sh.Output("kubectl", "get", "namespace", namespace, "--ignore-not-found=true", "-o", "name")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

// WaitForPort waits for a TCP port to become available on the given host.
// It polls every second and returns an error if the port is not reachable within the timeout.
func WaitForPort(host string, port int, timeout time.Duration) error {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	fmt.Printf("Waiting for %s to become available...\n", addr)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			_ = conn.Close()
			fmt.Printf("\nServer ready at %s\n", addr)
			return nil
		}
		fmt.Print(".")
		time.Sleep(1 * time.Second)
	}

	fmt.Println()
	return fmt.Errorf("timeout after %v waiting for %s", timeout, addr)
}

// SafeGetResource safely gets resource information without throwing errors
func (rc *ResourceChecker) SafeGetResource(resourceType, name string, outputFormat string) (string, error) {
	args := []string{"get", resourceType, name}
	if rc.Namespace != "" {
		args = append(args, "-n", rc.Namespace)
	}
	args = append(args, "--ignore-not-found=true")
	if outputFormat != "" {
		args = append(args, "-o", outputFormat)
	}

	output, err := sh.Output("kubectl", args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}
