//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// E2E groups end-to-end testing targets for CI and manual use.
type E2E mg.Namespace

const CSIDriverName = "s3.csi.scality.com"

// =============================================================================
// Configuration Helpers
// =============================================================================

// GetE2ENamespace returns the namespace for E2E operations.
// Defaults to "kube-system" (unlike GetNamespace() which defaults to "default" for local dev).
func GetE2ENamespace() string {
	if ns := os.Getenv("CSI_NAMESPACE"); ns != "" {
		return ns
	}
	return "kube-system"
}

// GetCSIImageTag returns the image tag for E2E installs.
// Priority: CSI_IMAGE_TAG > CONTAINER_TAG > "" (let chart default).
func GetCSIImageTag() string {
	if tag := os.Getenv("CSI_IMAGE_TAG"); tag != "" {
		return tag
	}
	if tag := os.Getenv("CONTAINER_TAG"); tag != "" {
		return tag
	}
	return ""
}

// GetCSIImageRepository returns the image repository for E2E installs.
func GetCSIImageRepository() string {
	return os.Getenv("CSI_IMAGE_REPOSITORY")
}

// GetJUnitReportPath returns the JUnit report path from env vars.
// Checks JUNIT_REPORT first, then parses ADDITIONAL_ARGS for --junit-report= (backward compat).
func GetJUnitReportPath() string {
	if path := os.Getenv("JUNIT_REPORT"); path != "" {
		return path
	}
	// Backward compatibility: parse --junit-report from ADDITIONAL_ARGS
	if args := os.Getenv("ADDITIONAL_ARGS"); args != "" {
		for _, arg := range strings.Fields(args) {
			if strings.HasPrefix(arg, "--junit-report=") {
				return strings.TrimPrefix(arg, "--junit-report=")
			}
		}
	}
	return ""
}

// =============================================================================
// Verify Target
// =============================================================================

// IsCSIDriverRegistered checks if the CSI driver is registered in the cluster.
func IsCSIDriverRegistered() (bool, error) {
	output, err := sh.Output("kubectl", "get", "csidrivers", "-o", "name")
	if err != nil {
		return false, fmt.Errorf("failed to get CSI drivers: %v", err)
	}
	return strings.Contains(output, CSIDriverName), nil
}

// verifyCSIInstallation checks CSI driver registration and pod readiness.
func verifyCSIInstallation() error {
	namespace := GetE2ENamespace()

	fmt.Println("Verifying CSI driver installation...")

	// Check CSI driver registration
	registered, err := IsCSIDriverRegistered()
	if err != nil {
		return fmt.Errorf("failed to check CSI driver registration: %v", err)
	}
	if !registered {
		return fmt.Errorf("CSI driver %s is not registered", CSIDriverName)
	}
	fmt.Printf("CSI driver %s is registered\n", CSIDriverName)

	// Check pods are ready
	checker := NewResourceChecker(namespace)

	fmt.Println("Waiting for CSI node pods to be ready...")
	if err := checker.WaitForPodsWithLabel("app=s3-csi-node", 300*time.Second); err != nil {
		if status := checker.GetPodsStatus("app=s3-csi-node"); status != "" {
			fmt.Printf("Node pod status:\n%s\n", status)
		}
		return fmt.Errorf("CSI node pods not ready: %v", err)
	}
	fmt.Println("CSI node pods are ready")

	fmt.Println("Waiting for CSI controller pods to be ready...")
	if err := checker.WaitForPodsWithLabel("app=s3-csi-controller", 120*time.Second); err != nil {
		if status := checker.GetPodsStatus("app=s3-csi-controller"); status != "" {
			fmt.Printf("Controller pod status:\n%s\n", status)
		}
		return fmt.Errorf("CSI controller pods not ready: %v", err)
	}
	fmt.Println("CSI controller pods are ready")

	fmt.Println("CSI driver installation verified successfully")
	return nil
}

// Verify checks that the CSI driver is properly installed and healthy.
func (E2E) Verify() error {
	return verifyCSIInstallation()
}

// =============================================================================
// Install Target
// =============================================================================

// installCSIForE2E installs the CSI driver for E2E/CI use.
// Unlike InstallCSI() (local dev), this accepts image params via env vars,
// defaults to kube-system, and skips DNS configuration.
func installCSIForE2E() error {
	namespace := GetE2ENamespace()
	s3EndpointURL := os.Getenv("S3_ENDPOINT_URL")
	if s3EndpointURL == "" {
		return fmt.Errorf("S3_ENDPOINT_URL environment variable is required")
	}

	// Get credentials
	accessKey := os.Getenv("ACCOUNT1_ACCESS_KEY")
	secretKey := os.Getenv("ACCOUNT1_SECRET_KEY")
	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("ACCOUNT1_ACCESS_KEY and ACCOUNT1_SECRET_KEY environment variables are required.\n" +
			"Load credentials using: source tests/e2e/scripts/load-credentials.sh\n" +
			"Or run mage e2e:all which loads them automatically from integration_config.json")
	}

	imageTag := GetCSIImageTag()
	imageRepo := GetCSIImageRepository()

	fmt.Printf("Installing CSI driver for E2E testing...\n")
	fmt.Printf("  Namespace: %s\n", namespace)
	fmt.Printf("  S3 endpoint: %s\n", s3EndpointURL)
	if imageTag != "" {
		fmt.Printf("  Image tag: %s\n", imageTag)
	}
	if imageRepo != "" {
		fmt.Printf("  Image repository: %s\n", imageRepo)
	}

	// Create namespace idempotently
	fmt.Printf("Creating namespace %s...\n", namespace)
	nsYAML, err := sh.Output("kubectl", "create", "namespace", namespace, "--dry-run=client", "-o", "yaml")
	if err != nil {
		return fmt.Errorf("failed to generate namespace YAML: %v", err)
	}
	if err := pipeToKubectlApply(nsYAML); err != nil {
		return fmt.Errorf("failed to create namespace: %v", err)
	}

	// Create secret idempotently
	fmt.Printf("Creating S3 credentials secret in namespace %s...\n", namespace)
	secretYAML, err := sh.Output("kubectl", "create", "secret", "generic", "s3-secret",
		fmt.Sprintf("--from-literal=access_key_id=%s", accessKey),
		fmt.Sprintf("--from-literal=secret_access_key=%s", secretKey),
		"-n", namespace,
		"--dry-run=client", "-o", "yaml")
	if err != nil {
		return fmt.Errorf("failed to generate secret YAML: %v", err)
	}
	if err := pipeToKubectlApply(secretYAML); err != nil {
		return fmt.Errorf("failed to create secret: %v", err)
	}

	// Build Helm args
	helmArgs := []string{
		"upgrade", "--install", "scality-s3-csi",
		"./charts/scality-mountpoint-s3-csi-driver",
		"--namespace", namespace,
		"--create-namespace",
		"--set", fmt.Sprintf("node.s3EndpointUrl=%s", s3EndpointURL),
		"--wait",
		"--timeout", "300s",
	}

	if imageTag != "" {
		helmArgs = append(helmArgs, "--set", fmt.Sprintf("image.tag=%s", imageTag))
	}
	if imageRepo != "" {
		helmArgs = append(helmArgs, "--set", fmt.Sprintf("image.repository=%s", imageRepo))
	}

	fmt.Println("Running Helm install...")
	if err := sh.RunV("helm", helmArgs...); err != nil {
		return fmt.Errorf("helm install failed: %v", err)
	}

	fmt.Println("Helm installation completed. Verifying...")
	return verifyCSIInstallation()
}

// pipeToKubectlApply pipes YAML content to kubectl apply.
func pipeToKubectlApply(yaml string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Install installs the CSI driver for E2E testing.
func (E2E) Install() error {
	return installCSIForE2E()
}
