//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
// Infrastructure Targets
// =============================================================================

const dockerComposeDir = ".github/scality-storage-deployment"

// DeployS3 starts CloudServer via docker compose and waits for port 8000.
func (E2E) DeployS3() error {
	return deployS3()
}

func deployS3() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	composeDir := filepath.Join(wd, dockerComposeDir)

	// Create logs directory
	logsDir := filepath.Join(composeDir, "logs", "s3")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create logs directory: %v", err)
	}

	fmt.Println("Starting CloudServer via docker compose...")
	cmd := exec.Command("docker", "compose", "--profile", "s3", "up", "-d", "--quiet-pull")
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Pass through environment (including CLOUDSERVER_TAG)
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %v", err)
	}

	fmt.Println("Waiting for CloudServer to be ready on port 8000...")
	return WaitForPort("localhost", 8000, 30*time.Second)
}

// StopS3 stops CloudServer via docker compose.
func (E2E) StopS3() error {
	return stopS3()
}

func stopS3() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	composeDir := filepath.Join(wd, dockerComposeDir)

	fmt.Println("Stopping CloudServer via docker compose...")
	cmd := exec.Command("docker", "compose", "--profile", "s3", "down")
	cmd.Dir = composeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down failed: %v", err)
	}

	fmt.Println("CloudServer stopped")
	return nil
}

// PullImages pulls container images and downloads Go dependencies in parallel.
// Reads CSI_IMAGE_REPOSITORY, CSI_IMAGE_TAG, and CLOUDSERVER_TAG env vars.
// Skips individual pulls if the corresponding env var is empty.
func (E2E) PullImages() error {
	return pullImages()
}

func pullImages() error {
	type task struct {
		name string
		fn   func() error
	}

	var tasks []task

	// CSI driver image
	if repo := GetCSIImageRepository(); repo != "" {
		if tag := GetCSIImageTag(); tag != "" {
			image := fmt.Sprintf("%s:%s", repo, tag)
			tasks = append(tasks, task{
				name: "CSI driver image",
				fn: func() error {
					return sh.Run("docker", "pull", image)
				},
			})
		}
	}

	// CloudServer image
	if csTag := os.Getenv("CLOUDSERVER_TAG"); csTag != "" {
		image := fmt.Sprintf("ghcr.io/scality/cloudserver:%s", csTag)
		tasks = append(tasks, task{
			name: "CloudServer image",
			fn: func() error {
				return sh.Run("docker", "pull", image)
			},
		})
	}

	// Go mod download for e2e tests
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	e2eDir := filepath.Join(wd, "tests", "e2e")
	tasks = append(tasks, task{
		name: "Go dependencies",
		fn: func() error {
			cmd := exec.Command("go", "mod", "download")
			cmd.Dir = e2eDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
	})

	if len(tasks) == 0 {
		fmt.Println("No images to pull (env vars not set)")
		return nil
	}

	fmt.Printf("Starting %d parallel tasks...\n", len(tasks))

	var wg sync.WaitGroup
	errs := make([]error, len(tasks))

	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, t task) {
			defer wg.Done()
			fmt.Printf("  Starting: %s\n", t.name)
			if err := t.fn(); err != nil {
				errs[idx] = fmt.Errorf("%s failed: %v", t.name, err)
				fmt.Printf("  Failed: %s\n", t.name)
			} else {
				fmt.Printf("  Done: %s\n", t.name)
			}
		}(i, t)
	}

	wg.Wait()

	// Collect errors
	var failures []string
	for _, err := range errs {
		if err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("parallel pulls failed:\n  %s", strings.Join(failures, "\n  "))
	}

	fmt.Println("All parallel tasks completed successfully")
	return nil
}

// =============================================================================
// CI DNS Target
// =============================================================================

// ConfigureCIDNS configures CoreDNS to resolve s3.scality.com to the CI runner's IP.
// Requires S3_HOST_IP environment variable. This is separate from local-dev ConfigureS3DNS
// which maps s3.example.com.
func (E2E) ConfigureCIDNS() error {
	return configureCIDNS()
}

func configureCIDNS() error {
	hostIP := os.Getenv("S3_HOST_IP")
	if hostIP == "" {
		return fmt.Errorf("S3_HOST_IP environment variable is required")
	}

	fmt.Printf("Configuring CoreDNS: s3.scality.com -> %s\n", hostIP)

	// Get current CoreDNS config
	currentConfig, err := sh.Output("kubectl", "get", "configmap", "coredns", "-n", "kube-system", "-o", "jsonpath={.data.Corefile}")
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS config: %v", err)
	}

	// Remove any existing s3.scality.com entries
	lines := strings.Split(currentConfig, "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.Contains(line, "s3.scality.com") {
			filtered = append(filtered, line)
		}
	}
	newConfig := strings.Join(filtered, "\n")

	// Add s3.scality.com entry
	if strings.Contains(newConfig, "hosts {") {
		newConfig = strings.ReplaceAll(newConfig, "hosts {", fmt.Sprintf("hosts {\n        %s s3.scality.com", hostIP))
	} else {
		// Insert hosts block before the "ready" directive
		newConfig = strings.ReplaceAll(newConfig, "ready", fmt.Sprintf("hosts {\n        %s s3.scality.com\n        fallthrough\n    }\n    ready", hostIP))
	}

	// Patch the ConfigMap
	patchData := fmt.Sprintf(`{"data":{"Corefile":%q}}`, newConfig)
	if err := sh.RunV("kubectl", "patch", "configmap", "coredns", "-n", "kube-system", "--type=merge", "-p", patchData); err != nil {
		return fmt.Errorf("failed to patch CoreDNS configmap: %v", err)
	}

	// Restart CoreDNS
	if err := RestartCoreDNS(); err != nil {
		return err
	}

	// Verify DNS resolution from within a pod
	fmt.Println("Verifying DNS resolution for s3.scality.com...")
	if err := sh.RunV("kubectl", "run", "dns-test", "--image=busybox:1.28", "--rm", "-it", "--restart=Never", "--", "nslookup", "s3.scality.com"); err != nil {
		return fmt.Errorf("DNS verification failed: %v", err)
	}

	fmt.Printf("CoreDNS configured: s3.scality.com -> %s\n", hostIP)
	return nil
}

// =============================================================================
// CRD and Compliance Targets
// =============================================================================

// ApplyCRDs applies the CSI driver CRDs from the Helm chart directory.
func (E2E) ApplyCRDs() error {
	fmt.Println("Applying CRDs...")
	if err := sh.RunV("kubectl", "apply", "-f", "./charts/scality-mountpoint-s3-csi-driver/crds/"); err != nil {
		return fmt.Errorf("failed to apply CRDs: %v", err)
	}
	fmt.Println("CRDs applied successfully")
	return nil
}

// csiComplianceSkipPatterns is the set of CSI compliance test patterns to skip.
// These match the Makefile's CSI_SKIP_PATTERNS.
const csiComplianceSkipPatterns = "ValidateVolumeCapabilities|Node Service|SingleNodeWriter|" +
	"should not fail when requesting to create a volume with already existing name and same capacity|" +
	"should fail when requesting to create a volume with already existing name and different capacity|" +
	"should not fail when creating volume with maximum-length name|" +
	"should return appropriate values.*no optional values added"

// ComplianceTest runs CSI compliance (sanity) tests against the deployed S3 backend.
// Loads credentials from integration_config.json, sets AWS_ENDPOINT_URL from S3_ENDPOINT_URL.
func (E2E) ComplianceTest() error {
	return complianceTest()
}

func complianceTest() error {
	// Load credentials
	if err := LoadCredentials(); err != nil {
		return fmt.Errorf("failed to load credentials: %v", err)
	}

	s3EndpointURL := os.Getenv("S3_ENDPOINT_URL")
	if s3EndpointURL == "" {
		return fmt.Errorf("S3_ENDPOINT_URL environment variable is required")
	}

	fmt.Printf("Running CSI compliance tests against %s...\n", s3EndpointURL)

	cmd := exec.Command("go", "test", "-v", "./tests/sanity/...",
		fmt.Sprintf("-ginkgo.skip=%s", csiComplianceSkipPatterns))
	cmd.Env = append(os.Environ(), fmt.Sprintf("AWS_ENDPOINT_URL=%s", s3EndpointURL))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("CSI compliance tests failed: %v", err)
	}

	fmt.Println("CSI compliance tests passed")
	return nil
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
			"Run mage e2e:all which loads them automatically from integration_config.json")
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

// =============================================================================
// Uninstall Targets
// =============================================================================

// uninstallCSIForE2E removes the CSI driver with configurable cleanup options.
func uninstallCSIForE2E(deleteNamespace, force bool) error {
	namespace := GetE2ENamespace()

	fmt.Printf("Uninstalling CSI driver from namespace %s...\n", namespace)

	// Uninstall Helm release
	if err := sh.Run("helm", "status", "scality-s3-csi", "-n", namespace); err != nil {
		fmt.Println("Helm release scality-s3-csi not found, skipping Helm uninstall")
	} else {
		if err := sh.RunV("helm", "uninstall", "scality-s3-csi", "-n", namespace); err != nil {
			if force {
				fmt.Printf("Warning: Helm uninstall failed: %v (continuing in force mode)\n", err)
			} else {
				return fmt.Errorf("helm uninstall failed: %v", err)
			}
		} else {
			fmt.Println("Helm release uninstalled successfully")
		}
	}

	// Delete secret
	if err := sh.Run("kubectl", "get", "secret", "s3-secret", "-n", namespace); err == nil {
		if err := sh.Run("kubectl", "delete", "secret", "s3-secret", "-n", namespace); err != nil {
			fmt.Printf("Warning: Failed to delete secret: %v\n", err)
		} else {
			fmt.Println("S3 credentials secret deleted")
		}
	}

	// Delete namespace if requested and it's not kube-system
	if deleteNamespace && namespace != "kube-system" {
		fmt.Printf("Deleting namespace %s...\n", namespace)
		if err := sh.Run("kubectl", "delete", "namespace", namespace, "--timeout=60s"); err != nil {
			if force {
				fmt.Printf("Warning: Failed to delete namespace: %v (continuing in force mode)\n", err)
			} else {
				return fmt.Errorf("failed to delete namespace %s: %v", namespace, err)
			}
		} else {
			fmt.Printf("Namespace %s deleted\n", namespace)
		}
	}

	// Force: delete CSI driver registration
	if force {
		output, _ := sh.Output("kubectl", "get", "csidrivers", "-o", "name")
		if strings.Contains(output, CSIDriverName) {
			fmt.Printf("Deleting CSI driver registration %s...\n", CSIDriverName)
			if err := sh.Run("kubectl", "delete", "csidriver", CSIDriverName); err != nil {
				fmt.Printf("Warning: Failed to delete CSI driver registration: %v\n", err)
			} else {
				fmt.Println("CSI driver registration deleted")
			}
		}
	}

	fmt.Println("Uninstallation complete")
	return nil
}

// Uninstall removes the CSI driver (Helm release + secret).
func (E2E) Uninstall() error {
	return uninstallCSIForE2E(false, false)
}

// UninstallClean removes the CSI driver and deletes custom namespace (not kube-system).
func (E2E) UninstallClean() error {
	return uninstallCSIForE2E(true, false)
}

// UninstallForce force-removes the CSI driver, including driver registration.
func (E2E) UninstallForce() error {
	return uninstallCSIForE2E(true, true)
}

// =============================================================================
// Ginkgo Test Runner
// =============================================================================

// runGinkgoTests invokes Ginkgo to run E2E tests.
func runGinkgoTests(s3EndpointURL, junitReportPath string) error {
	if s3EndpointURL == "" {
		s3EndpointURL = os.Getenv("S3_ENDPOINT_URL")
	}
	if s3EndpointURL == "" {
		return fmt.Errorf("S3_ENDPOINT_URL environment variable is required")
	}

	// Resolve tests/e2e directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	e2eDir := filepath.Join(wd, "tests", "e2e")
	if _, err := os.Stat(e2eDir); os.IsNotExist(err) {
		return fmt.Errorf("tests/e2e directory not found: %s", e2eDir)
	}

	// Find ginkgo binary
	ginkgoBin, err := findGinkgo()
	if err != nil {
		return err
	}

	// Build ginkgo command arguments
	args := []string{
		"--procs=8",
		"-timeout=15m",
		"-v",
	}

	// Add JUnit report if specified
	if junitReportPath == "" {
		junitReportPath = GetJUnitReportPath()
	}
	if junitReportPath != "" {
		// Create output directory if needed
		reportDir := filepath.Dir(junitReportPath)
		if reportDir != "." {
			if err := os.MkdirAll(reportDir, 0o755); err != nil {
				return fmt.Errorf("failed to create JUnit report directory: %v", err)
			}
		}
		args = append(args, fmt.Sprintf("--junit-report=%s", junitReportPath))
		fmt.Printf("JUnit report will be written to: %s\n", junitReportPath)
	}

	// Add test packages and passthrough args
	args = append(args, "./...", "--", fmt.Sprintf("--s3-endpoint-url=%s", s3EndpointURL))

	// Resolve KUBECONFIG
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return fmt.Errorf("KUBECONFIG file not found: %s", kubeconfig)
	}

	fmt.Printf("Running Ginkgo E2E tests...\n")
	fmt.Printf("  Test directory: %s\n", e2eDir)
	fmt.Printf("  S3 endpoint: %s\n", s3EndpointURL)
	fmt.Printf("  KUBECONFIG: %s\n", kubeconfig)

	// Execute ginkgo
	cmd := exec.Command(ginkgoBin, args...)
	cmd.Dir = e2eDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Check for JUnit report files even on failure
		if junitReportPath != "" {
			fmt.Println("Checking for JUnit report files after test failure:")
			_ = filepath.Walk(e2eDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && strings.HasSuffix(path, ".xml") {
					fmt.Printf("  Found: %s\n", path)
				}
				return nil
			})
		}
		return fmt.Errorf("ginkgo tests failed: %v", err)
	}

	fmt.Println("Ginkgo E2E tests completed successfully")
	return nil
}

// findGinkgo locates the ginkgo binary in PATH or $GOPATH/bin.
func findGinkgo() (string, error) {
	// Check PATH first
	if path, err := exec.LookPath("ginkgo"); err == nil {
		return path, nil
	}

	// Check $GOPATH/bin
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	ginkgoPath := filepath.Join(gopath, "bin", "ginkgo")
	if _, err := os.Stat(ginkgoPath); err == nil {
		return ginkgoPath, nil
	}

	return "", fmt.Errorf("ginkgo binary not found in PATH or $GOPATH/bin.\n" +
		"Install it with: go install github.com/onsi/ginkgo/v2/ginkgo@latest")
}

// GoTest runs Ginkgo E2E tests without verification checks.
func (E2E) GoTest() error {
	return runGinkgoTests("", "")
}

// =============================================================================
// Orchestration Targets
// =============================================================================

// Test verifies the CSI driver installation then runs Ginkgo E2E tests.
func (E2E) Test() error {
	if err := verifyCSIInstallation(); err != nil {
		return fmt.Errorf("verification failed, cannot proceed with tests: %v", err)
	}
	return runGinkgoTests("", "")
}

// All loads credentials, installs the CSI driver, and runs E2E tests.
func (E2E) All() error {
	fmt.Println("Starting full E2E workflow: load credentials -> install -> test")

	// Load credentials from integration_config.json
	if err := LoadCredentials(); err != nil {
		return fmt.Errorf("failed to load credentials: %v", err)
	}

	// Install CSI driver
	if err := installCSIForE2E(); err != nil {
		return fmt.Errorf("CSI driver installation failed: %v", err)
	}

	// Run tests
	if err := runGinkgoTests("", ""); err != nil {
		return fmt.Errorf("E2E tests failed: %v", err)
	}

	fmt.Println("Full E2E workflow completed successfully")
	return nil
}

// =============================================================================
// Event Capture Targets
// =============================================================================

const (
	captureScript  = "tests/e2e/scripts/capture-events-and-logs.sh"
	captureDir     = "artifacts/k8s-debug"
	capturePIDFile = "capture.pid"
	captureArchive = "artifacts/k8s-debug-capture.tar.gz"
	s3LogsSource   = ".github/scality-storage-deployment/logs/s3"
	s3LogsDest     = "artifacts/logs/s3"
)

// StartCapture starts background Kubernetes event and log capture for CI diagnostics.
func (E2E) StartCapture() error {
	return startCapture()
}

func startCapture() error {
	if err := os.MkdirAll(captureDir, 0o755); err != nil {
		return fmt.Errorf("failed to create capture directory: %v", err)
	}

	fmt.Println("Starting Kubernetes event and log capture...")
	cmd := exec.Command("./"+captureScript, captureDir, "start")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start capture script: %v", err)
	}

	pid := cmd.Process.Pid
	if err := os.WriteFile(capturePIDFile, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return fmt.Errorf("failed to write PID file: %v", err)
	}

	fmt.Printf("Capture started (PID %d)\n", pid)
	return nil
}

// StopCapture stops event capture, collects final snapshots, and compresses artifacts.
func (E2E) StopCapture() error {
	return stopCapture()
}

func stopCapture() error {
	// Kill the capture process if running
	if data, err := os.ReadFile(capturePIDFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if proc, err := os.FindProcess(pid); err == nil {
				if err := proc.Signal(syscall.SIGTERM); err != nil {
					fmt.Printf("Warning: could not stop capture process (PID %d): %v\n", pid, err)
				} else {
					fmt.Printf("Stopped capture process (PID %d)\n", pid)
				}
			}
		}
	}

	// Run the capture script in stop mode to take final snapshots
	fmt.Println("Taking final cluster state snapshot...")
	cmd := exec.Command("./"+captureScript, captureDir, "stop")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: capture stop returned error: %v\n", err)
	}

	// Compress K8s debug data
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		return fmt.Errorf("failed to create artifacts directory: %v", err)
	}
	fmt.Println("Compressing capture data...")
	if err := sh.Run("tar", "-czf", captureArchive, "-C", "artifacts", "k8s-debug/"); err != nil {
		fmt.Printf("Warning: failed to compress capture data: %v\n", err)
	}

	// Copy S3 logs to artifacts directory
	if err := os.MkdirAll(s3LogsDest, 0o755); err != nil {
		fmt.Printf("Warning: failed to create S3 logs directory: %v\n", err)
	} else {
		if err := sh.Run("cp", "-r", s3LogsSource+"/.", s3LogsDest+"/"); err != nil {
			fmt.Printf("Warning: no S3 logs to copy (or copy failed): %v\n", err)
		} else {
			fmt.Println("S3 logs copied to artifacts")
		}
	}

	// Clean up PID file
	os.Remove(capturePIDFile)

	fmt.Println("Capture stopped and artifacts collected")
	return nil
}
