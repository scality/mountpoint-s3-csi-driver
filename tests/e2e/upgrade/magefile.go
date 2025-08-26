//go:build mage
// +build mage

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

// Upgrade test configuration
var (
	namespace       = getEnv("NAMESPACE", "scality-s3-csi")
	fromVersion     = getEnv("FROM_VERSION", "1.2.0")
	toVersion       = getEnv("TO_VERSION", "local")
	toImage         = getEnv("TO_IMAGE", "")
	s3Endpoint      = getEnv("S3_ENDPOINT_URL", "http://s3.scality.com:8000")
	testDuration    = getEnv("TEST_DURATION", "30")
	account1Key     = os.Getenv("ACCOUNT1_ACCESS_KEY")
	account1Secret  = os.Getenv("ACCOUNT1_SECRET_KEY")
)

// Test buckets
var testBuckets = []string{
	"upgrade-test-bucket-1",
	"upgrade-test-bucket-2",
}

// Upgrade namespace contains all upgrade test targets
type Upgrade mg.Namespace

// All runs the complete upgrade test suite
func (Upgrade) All() error {
	mg.SerialDeps(
		Upgrade.Clean,
		Upgrade.PrepareNamespace,
		Upgrade.InstallV1,
		Upgrade.CreateBuckets,
		Upgrade.CreateWorkloads,
		Upgrade.WriteTestData,
		Upgrade.StartContinuousIO,
		Upgrade.PerformUpgrade,
		Upgrade.VerifyUpgrade,
		Upgrade.RunStabilityTest,
	)
	return nil
}

// Clean removes all test resources
func (Upgrade) Clean() error {
	fmt.Println("🧹 Cleaning up test resources...")
	
	// Delete namespace (will delete all resources within it)
	sh.Run("kubectl", "delete", "namespace", namespace, "--ignore-not-found=true", "--wait=true")
	
	// Clean up any PVs that might be left
	sh.Run("kubectl", "delete", "pv", "-l", "test=upgrade", "--ignore-not-found=true")
	
	return nil
}

// PrepareNamespace creates the namespace and secret
func (Upgrade) PrepareNamespace() error {
	fmt.Printf("📦 Preparing namespace %s...\n", namespace)
	
	// Create namespace
	if err := sh.Run("kubectl", "create", "namespace", namespace); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	
	// Create S3 secret
	fmt.Println("🔐 Creating S3 credentials secret...")
	if account1Key == "" || account1Secret == "" {
		return fmt.Errorf("ACCOUNT1_ACCESS_KEY and ACCOUNT1_SECRET_KEY must be set")
	}
	
	if err := sh.Run("kubectl", "create", "secret", "generic", "s3-secret",
		fmt.Sprintf("--from-literal=access_key_id=%s", account1Key),
		fmt.Sprintf("--from-literal=secret_access_key=%s", account1Secret),
		"--namespace", namespace); err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	
	return nil
}

// InstallV1 installs the v1.2.0 CSI driver
func (Upgrade) InstallV1() error {
	fmt.Printf("📥 Installing CSI driver v%s...\n", fromVersion)
	
	// Strip 'v' prefix if present
	version := strings.TrimPrefix(fromVersion, "v")
	
	// Install using Helm from OCI registry
	if err := sh.Run("helm", "install", "scality-mountpoint-s3-csi-driver",
		fmt.Sprintf("oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver"),
		"--version", version,
		"--namespace", namespace,
		"--set", fmt.Sprintf("s3.endpointUrl=%s", s3Endpoint),
		"--wait", "--timeout", "5m"); err != nil {
		return fmt.Errorf("failed to install v%s: %w", fromVersion, err)
	}
	
	// Verify installation
	if err := verifyDriverReady(); err != nil {
		return fmt.Errorf("driver not ready after installation: %w", err)
	}
	
	fmt.Printf("✅ CSI driver v%s installed successfully\n", fromVersion)
	return nil
}

// CreateBuckets creates the test S3 buckets
func (Upgrade) CreateBuckets() error {
	fmt.Println("🪣 Creating test buckets...")
	
	// Create a Job to create buckets
	jobYaml := fmt.Sprintf(`
apiVersion: batch/v1
kind: Job
metadata:
  name: create-buckets
  namespace: %s
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: bucket-creator
        image: amazon/aws-cli:2.13.0
        env:
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: s3-secret
              key: access_key_id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: s3-secret
              key: secret_access_key
        - name: AWS_DEFAULT_REGION
          value: "us-east-1"
        command:
        - /bin/sh
        - -c
        - |
          for bucket in %s; do
            echo "Creating bucket: $bucket"
            aws s3 mb "s3://$bucket" --endpoint-url %s || echo "Bucket $bucket may already exist"
          done
          echo "All buckets created"
`, namespace, strings.Join(testBuckets, " "), s3Endpoint)
	
	// Apply the job
	if err := kubectlApply(jobYaml); err != nil {
		return fmt.Errorf("failed to create bucket job: %w", err)
	}
	
	// Wait for completion
	if err := sh.Run("kubectl", "wait", "--for=condition=complete",
		"job/create-buckets", "-n", namespace, "--timeout=60s"); err != nil {
		// Show logs on failure
		sh.Run("kubectl", "logs", "job/create-buckets", "-n", namespace)
		return fmt.Errorf("bucket creation failed: %w", err)
	}
	
	// Clean up job
	sh.Run("kubectl", "delete", "job", "create-buckets", "-n", namespace, "--ignore-not-found=true")
	
	fmt.Println("✅ Test buckets created")
	return nil
}

// CreateWorkloads creates the test workloads
func (Upgrade) CreateWorkloads() error {
	fmt.Println("🚀 Creating test workloads...")
	
	// Apply old-workload.yaml
	if err := sh.Run("kubectl", "apply", "-f", "fixtures/old-workload.yaml"); err != nil {
		return fmt.Errorf("failed to create workloads: %w", err)
	}
	
	// Wait for pod to be ready
	if err := sh.Run("kubectl", "wait", "--for=condition=Ready",
		"pod/test-pod-1", "--timeout=300s"); err != nil {
		// Show pod status on failure
		sh.Run("kubectl", "describe", "pod", "test-pod-1")
		sh.Run("kubectl", "get", "events", "--sort-by='.lastTimestamp'")
		return fmt.Errorf("test pod not ready: %w", err)
	}
	
	fmt.Println("✅ Test workloads created and ready")
	return nil
}

// WriteTestData writes test data to the mounted volumes
func (Upgrade) WriteTestData() error {
	fmt.Println("📝 Writing test data...")
	
	// Write test files
	for i := 1; i <= 10; i++ {
		filename := fmt.Sprintf("test-data-%d.txt", i)
		content := fmt.Sprintf("Test data %d written at %s", i, time.Now())
		
		cmd := fmt.Sprintf("echo '%s' > /data/%s", content, filename)
		if err := sh.Run("kubectl", "exec", "test-pod-1", "--", "sh", "-c", cmd); err != nil {
			return fmt.Errorf("failed to write test file %s: %w", filename, err)
		}
	}
	
	// Calculate checksums
	checksum, err := sh.Output("kubectl", "exec", "test-pod-1", "--", 
		"sh", "-c", "find /data -name '*.txt' -exec md5sum {} \\;")
	if err != nil {
		return fmt.Errorf("failed to calculate checksums: %w", err)
	}
	
	// Save checksums for later verification
	if err := os.WriteFile("/tmp/upgrade-test-checksums.txt", []byte(checksum), 0644); err != nil {
		return fmt.Errorf("failed to save checksums: %w", err)
	}
	
	fmt.Println("✅ Test data written and checksums calculated")
	return nil
}

// StartContinuousIO starts a continuous I/O workload
func (Upgrade) StartContinuousIO() error {
	fmt.Println("🔄 Starting continuous I/O workload...")
	
	// Apply io-test-pod.yaml if it exists, otherwise create inline
	if _, err := os.Stat("fixtures/io-test-pod.yaml"); err == nil {
		if err := sh.Run("kubectl", "apply", "-f", "fixtures/io-test-pod.yaml"); err != nil {
			return fmt.Errorf("failed to create I/O test pod: %w", err)
		}
	} else {
		// Create inline I/O pod
		ioPodYaml := `
apiVersion: v1
kind: Pod
metadata:
  name: io-test-pod
  labels:
    test: upgrade
    purpose: continuous-io
spec:
  containers:
  - name: io-writer
    image: busybox:1.36
    command: ["/bin/sh", "-c", "counter=0; while true; do echo \"Write $counter at $(date)\" >> /data/io-test.log; counter=$((counter + 1)); sleep 2; done"]
    volumeMounts:
    - name: s3-volume
      mountPath: /data
  volumes:
  - name: s3-volume
    persistentVolumeClaim:
      claimName: upgrade-test-pvc-1
`
		if err := kubectlApply(ioPodYaml); err != nil {
			return fmt.Errorf("failed to create I/O test pod: %w", err)
		}
	}
	
	// Wait for I/O pod to start
	if err := sh.Run("kubectl", "wait", "--for=condition=Ready",
		"pod/io-test-pod", "--timeout=60s"); err != nil {
		sh.Run("kubectl", "describe", "pod", "io-test-pod")
		return fmt.Errorf("I/O test pod not ready: %w", err)
	}
	
	fmt.Println("✅ Continuous I/O started")
	return nil
}

// PerformUpgrade upgrades the CSI driver to the new version
func (Upgrade) PerformUpgrade() error {
	fmt.Printf("⬆️ Upgrading to version %s...\n", toVersion)
	
	upgradeArgs := []string{
		"upgrade", "scality-mountpoint-s3-csi-driver",
		"--namespace", namespace,
		"--reuse-values",
		"--wait", "--timeout", "5m",
	}
	
	if toVersion == "local" {
		// Upgrade to local chart
		upgradeArgs = append(upgradeArgs, "./charts/scality-mountpoint-s3-csi-driver")
		
		// If toImage is specified, use it
		if toImage != "" {
			parts := strings.Split(toImage, ":")
			if len(parts) == 2 {
				upgradeArgs = append(upgradeArgs,
					"--set", fmt.Sprintf("image.repository=%s", parts[0]),
					"--set", fmt.Sprintf("image.tag=%s", parts[1]))
			}
		}
	} else {
		// Upgrade to specific version from OCI registry
		version := strings.TrimPrefix(toVersion, "v")
		upgradeArgs = append(upgradeArgs,
			fmt.Sprintf("oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver"),
			"--version", version)
	}
	
	if err := sh.Run("helm", upgradeArgs...); err != nil {
		// Show status on failure
		sh.Run("kubectl", "get", "pods", "-n", namespace)
		sh.Run("helm", "status", "scality-mountpoint-s3-csi-driver", "-n", namespace)
		return fmt.Errorf("upgrade failed: %w", err)
	}
	
	// Verify driver is ready after upgrade
	if err := verifyDriverReady(); err != nil {
		return fmt.Errorf("driver not ready after upgrade: %w", err)
	}
	
	fmt.Printf("✅ Successfully upgraded to %s\n", toVersion)
	return nil
}

// VerifyUpgrade verifies the upgrade was successful
func (Upgrade) VerifyUpgrade() error {
	fmt.Println("🔍 Verifying upgrade...")
	
	// Check existing mounts still work
	output, err := sh.Output("kubectl", "exec", "test-pod-1", "--", "df", "-h", "/data")
	if err != nil {
		return fmt.Errorf("failed to check mount: %w", err)
	}
	
	if !strings.Contains(output, "mountpoint-s3") && !strings.Contains(output, "/data") {
		return fmt.Errorf("mount not found after upgrade")
	}
	
	// Verify data integrity
	currentChecksum, err := sh.Output("kubectl", "exec", "test-pod-1", "--",
		"sh", "-c", "find /data -name 'test-data-*.txt' -exec md5sum {} \\;")
	if err != nil {
		return fmt.Errorf("failed to calculate checksums after upgrade: %w", err)
	}
	
	originalChecksum, err := os.ReadFile("/tmp/upgrade-test-checksums.txt")
	if err != nil {
		return fmt.Errorf("failed to read original checksums: %w", err)
	}
	
	if string(originalChecksum) != currentChecksum {
		return fmt.Errorf("data integrity check failed:\nOriginal:\n%s\nCurrent:\n%s", 
			originalChecksum, currentChecksum)
	}
	
	// Verify continuous I/O is still running
	ioLog, err := sh.Output("kubectl", "exec", "io-test-pod", "--", 
		"tail", "-n", "5", "/data/io-test.log")
	if err != nil {
		fmt.Printf("Warning: Could not verify I/O continuity: %v\n", err)
	} else {
		fmt.Printf("Recent I/O log:\n%s\n", ioLog)
	}
	
	// Create new workload to test new mounts
	if err := sh.Run("kubectl", "apply", "-f", "fixtures/new-workload.yaml"); err != nil {
		return fmt.Errorf("failed to create new workload: %w", err)
	}
	
	if err := sh.Run("kubectl", "wait", "--for=condition=Ready",
		"pod/test-pod-2", "--timeout=300s"); err != nil {
		return fmt.Errorf("new pod not ready after upgrade: %w", err)
	}
	
	fmt.Println("✅ Upgrade verification successful")
	fmt.Println("  - Existing mounts still working")
	fmt.Println("  - Data integrity preserved")
	fmt.Println("  - New workloads can be created")
	return nil
}

// RunStabilityTest runs a stability test for the specified duration
func (Upgrade) RunStabilityTest() error {
	duration, _ := time.ParseDuration(fmt.Sprintf("%sm", testDuration))
	fmt.Printf("⏱️ Running stability test for %s...\n", duration)
	
	startTime := time.Now()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for time.Since(startTime) < duration {
		select {
		case <-ticker.C:
			// Check pod health
			pods, err := sh.Output("kubectl", "get", "pods", "-n", namespace, "-o", "wide")
			if err != nil {
				return fmt.Errorf("failed to get pods: %w", err)
			}
			
			// Simple health check - ensure no pods are in error state
			if strings.Contains(pods, "Error") || strings.Contains(pods, "CrashLoop") {
				return fmt.Errorf("pods in error state during stability test:\n%s", pods)
			}
			
			// Write a test file to verify I/O
			testFile := fmt.Sprintf("stability-test-%d.txt", time.Now().Unix())
			cmd := fmt.Sprintf("echo 'Stability test at %s' > /data/%s", time.Now(), testFile)
			if err := sh.Run("kubectl", "exec", "test-pod-1", "--", "sh", "-c", cmd); err != nil {
				return fmt.Errorf("failed to write during stability test: %w", err)
			}
			
			elapsed := time.Since(startTime).Round(time.Second)
			remaining := duration - elapsed
			fmt.Printf("  ⏳ Stability test: %s elapsed, %s remaining...\n", elapsed, remaining)
		}
	}
	
	fmt.Printf("✅ Stability test passed (%s)\n", duration)
	return nil
}

// Helper functions

func verifyDriverReady() error {
	// Check DaemonSet rollout
	if err := sh.Run("kubectl", "rollout", "status", 
		"daemonset/s3-csi-node", "-n", namespace, "--timeout=120s"); err != nil {
		return fmt.Errorf("node DaemonSet not ready: %w", err)
	}
	
	// Check Deployment rollout
	if err := sh.Run("kubectl", "rollout", "status",
		"deployment/s3-csi-controller", "-n", namespace, "--timeout=120s"); err != nil {
		return fmt.Errorf("controller Deployment not ready: %w", err)
	}
	
	// Verify CSI driver is registered
	output, err := sh.Output("kubectl", "get", "csidriver", "s3.csi.scality.com")
	if err != nil || !strings.Contains(output, "s3.csi.scality.com") {
		return fmt.Errorf("CSI driver not registered")
	}
	
	return nil
}

func kubectlApply(yaml string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}