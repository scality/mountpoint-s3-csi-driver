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
	var jqCmd string
	if remove {
		// Remove s3.example.com entries from CoreDNS configuration
		jqCmd = `.data.Corefile |= gsub("\\n.*s3\\.example\\.com"; "")`
	} else {
		// First remove any existing s3.example.com entries, then add the new one
		removeCmd := `.data.Corefile |= gsub("\\n.*s3\\.example\\.com"; "")`
		addCmd := fmt.Sprintf(`.data.Corefile |= gsub("hosts \\{"; "hosts {\n           %s s3.example.com")`, s3Host)
		jqCmd = fmt.Sprintf("(%s) | (%s)", removeCmd, addCmd)
	}

	// Apply the configuration
	shellCmd := fmt.Sprintf(
		`kubectl get configmap coredns -n kube-system -o json | jq '%s' | kubectl apply -f -`,
		jqCmd)

	return RunCommand("sh", "-c", shellCmd)
}
