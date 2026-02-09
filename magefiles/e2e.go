//go:build mage

package main

import (
	"fmt"
	"os"
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
