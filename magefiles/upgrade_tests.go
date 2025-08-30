package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/magefile/mage/sh"
)

const defaultUpgradeTestNamespace = "upgrade-test"

// SetupUpgradeTests creates test pods with PVC mounts before upgrade
func SetupUpgradeTests() error {
	namespace := getUpgradeTestNamespace()
	testType := getUpgradeTestType()

	fmt.Println("===========================================")
	fmt.Println("Setting up CSI Driver Upgrade Tests")
	fmt.Printf("Namespace: %s\n", namespace)
	fmt.Printf("Test Type: %s\n", testType)
	fmt.Println("===========================================")

	// Create namespace
	fmt.Println("Creating test namespace...")
	if err := sh.Run("kubectl", "create", "namespace", namespace); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create namespace: %v", err)
		}
		fmt.Printf("Note: Namespace %s already exists\n", namespace)
	}

	// Setup tests based on type
	if testType == "all" || testType == "static" {
		if err := setupStaticTest(namespace); err != nil {
			return fmt.Errorf("static test setup failed: %v", err)
		}
	}

	if testType == "all" || testType == "dynamic" {
		if err := setupDynamicTest(namespace); err != nil {
			return fmt.Errorf("dynamic test setup failed: %v", err)
		}
	}

	// Wait for pods to be ready
	fmt.Println("\nWaiting for test pods to be ready...")
	timeout := getUpgradeTestTimeout()
	if err := sh.Run("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "test", "-n", namespace, "--timeout="+timeout); err != nil {
		return fmt.Errorf("pods not ready: %v", err)
	}

	// Give pods extra time to fully initialize
	fmt.Println("Allowing pods to fully initialize...")
	time.Sleep(10 * time.Second)

	// Capture mount state before upgrade
	fmt.Println("\nCapturing mount state before upgrade...")
	pods := getTestPods(testType)

	for _, pod := range pods {
		fmt.Printf("Capturing state for %s...\n", pod)

		// Use a simple approach - no external package dependencies
		data, err := captureBeforeUpgrade(pod, namespace)
		if err != nil {
			return fmt.Errorf("failed to capture state for %s: %v", pod, err)
		}

		// Store in temp file for persistence across upgrade
		storeFile := fmt.Sprintf("/tmp/upgrade-test-%s.json", pod)
		if err := saveMountDataSimple(storeFile, data); err != nil {
			return fmt.Errorf("failed to store mount data: %v", err)
		}

		fmt.Printf("✓ State captured for %s\n", pod)
		fmt.Printf("  - Mount PID: %s\n", data["mountPID"])
		fmt.Printf("  - Mount ID: %s\n", data["mountID"])
		fmt.Printf("  - Writer PID: %s\n", data["writerPID"])
	}

	fmt.Println("\n✅ Upgrade test setup complete!")
	fmt.Println("You can now upgrade the CSI driver.")

	return nil
}

// VerifyUpgradeTests verifies mounts survived the upgrade without remounting
func VerifyUpgradeTests() error {
	namespace := getUpgradeTestNamespace()
	testType := getUpgradeTestType()

	fmt.Println("===========================================")
	fmt.Println("Verifying CSI Driver Upgrade Tests")
	fmt.Printf("Namespace: %s\n", namespace)
	fmt.Printf("Test Type: %s\n", testType)
	fmt.Println("===========================================")

	pods := getTestPods(testType)
	allPassed := true

	for _, pod := range pods {
		fmt.Printf("\nVerifying %s...\n", pod)

		// Load stored mount data
		storeFile := fmt.Sprintf("/tmp/upgrade-test-%s.json", pod)
		beforeData, err := loadMountDataSimple(storeFile)
		if err != nil {
			fmt.Printf("❌ Failed to load pre-upgrade data for %s: %v\n", pod, err)
			allPassed = false
			continue
		}

		// Verify mount continuity
		if err := verifyAfterUpgrade(beforeData, pod, namespace); err != nil {
			fmt.Printf("❌ Verification failed for %s: %v\n", pod, err)
			allPassed = false
		}
	}

	if !allPassed {
		return fmt.Errorf("some upgrade tests failed")
	}

	fmt.Println("\n✅ All upgrade tests passed!")
	return nil
}

// CleanupUpgradeTests removes all test resources
func CleanupUpgradeTests() error {
	namespace := getUpgradeTestNamespace()

	fmt.Println("Cleaning up upgrade test resources...")

	// Delete namespace (removes all resources within it)
	if err := sh.Run("kubectl", "delete", "namespace", namespace, "--ignore-not-found"); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	// Clean up temp files
	_ = sh.Run("rm", "-f", "/tmp/upgrade-test-*.json")

	fmt.Println("✓ Cleanup complete")
	return nil
}

// Helper functions

func setupStaticTest(namespace string) error {
	fmt.Println("\nSetting up static provisioning test...")

	// Create PV first
	bucketName := os.Getenv("UPGRADE_TEST_BUCKET")
	if bucketName == "" {
		bucketName = "upgrade-test-static-bucket"
	}

	pvManifest := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolume
metadata:
  name: upgrade-test-static-pv
spec:
  capacity:
    storage: 5Gi
  accessModes:
    - ReadWriteMany
  mountOptions:
    - allow-delete
    - cache /tmp/cache
  csi:
    driver: s3.csi.scality.com
    volumeHandle: %s
    volumeAttributes:
      bucketName: %s
`, bucketName, bucketName)

	if err := applyManifest(pvManifest); err != nil {
		return fmt.Errorf("failed to create PV: %v", err)
	}

	// Create PVC and Pod
	manifest := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: static-pvc
  namespace: %s
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
  volumeName: upgrade-test-static-pv
---
apiVersion: v1
kind: Pod
metadata:
  name: static-test-pod
  namespace: %s
  labels:
    test: upgrade-static
spec:
  containers:
  - name: test
    image: ubuntu
    command: ["/bin/bash", "-c"]
    args:
    - |
      apt-get update && apt-get install -y procps
      echo "Static test pod started at $(date)" > /data/startup.log

      # Keep pod running
      while true; do
        sleep 10
      done
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: static-pvc
`, namespace, namespace)

	if err := applyManifest(manifest); err != nil {
		return fmt.Errorf("failed to create static test: %v", err)
	}

	fmt.Println("✓ Static test resources created")
	return nil
}

func setupDynamicTest(namespace string) error {
	fmt.Println("\nSetting up dynamic provisioning test...")

	// Create StorageClass, PVC and Pod
	manifest := fmt.Sprintf(`
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-upgrade-test
provisioner: s3.csi.scality.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: s3-secret
  csi.storage.k8s.io/provisioner-secret-namespace: %s
  csi.storage.k8s.io/controller-publish-secret-name: s3-secret
  csi.storage.k8s.io/controller-publish-secret-namespace: %s
mountOptions:
  - allow-delete
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: dynamic-pvc
  namespace: %s
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi-upgrade-test
  resources:
    requests:
      storage: 5Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: dynamic-test-pod
  namespace: %s
  labels:
    test: upgrade-dynamic
spec:
  containers:
  - name: test
    image: ubuntu
    command: ["/bin/bash", "-c"]
    args:
    - |
      apt-get update && apt-get install -y procps
      echo "Dynamic test pod started at $(date)" > /data/startup.log

      # Keep pod running
      while true; do
        sleep 10
      done
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: dynamic-pvc
`, namespace, namespace, namespace, namespace)

	if err := applyManifest(manifest); err != nil {
		return fmt.Errorf("failed to create dynamic test: %v", err)
	}

	fmt.Println("✓ Dynamic test resources created")
	return nil
}

func captureBeforeUpgrade(podName, namespace string) (map[string]string, error) {
	data := make(map[string]string)
	data["podName"] = podName
	data["namespace"] = namespace
	data["captureTime"] = time.Now().Format(time.RFC3339)

	// 1. Start continuous writer with open file handle
	script := `
# Install required packages if not present
which pgrep > /dev/null || (apt-get update && apt-get install -y procps)

# Create writer script
cat > /tmp/writer.sh << 'EOF'
#!/bin/bash
exec 3> /data/lock-file.txt
echo $$ > /tmp/writer.pid

while true; do
    timestamp=$(date +%s.%N)
    echo "$timestamp" >&3
    echo "$timestamp" >> /data/continuous.log
    sleep 0.1
done
EOF

chmod +x /tmp/writer.sh
nohup /tmp/writer.sh > /tmp/writer.out 2>&1 &
sleep 2
cat /tmp/writer.pid
`

	writerPID, err := kubectlExec(namespace, podName, []string{"bash", "-c", script})
	if err != nil {
		return nil, fmt.Errorf("failed to start continuous writer: %v", err)
	}
	data["writerPID"] = strings.TrimSpace(writerPID)

	// 2. Get mountpoint-s3 process PID
	pidCmd := "ps aux | grep '[m]ountpoint-s3' | awk '{print $2}' | head -1"
	pid, err := kubectlExec(namespace, podName, []string{"bash", "-c", pidCmd})
	if err != nil {
		return nil, fmt.Errorf("failed to get mount PID: %v", err)
	}
	data["mountPID"] = strings.TrimSpace(pid)

	if data["mountPID"] == "" {
		return nil, fmt.Errorf("mountpoint-s3 process not found")
	}

	// 3. Get process start time
	startCmd := fmt.Sprintf("stat -c %%Y /proc/%s 2>/dev/null || echo 'no-process'", data["mountPID"])
	startTime, err := kubectlExec(namespace, podName, []string{"bash", "-c", startCmd})
	if err != nil {
		return nil, fmt.Errorf("failed to get process start time: %v", err)
	}
	data["processStartTime"] = strings.TrimSpace(startTime)

	// 4. Get mount ID from mountinfo
	mountCmd := "grep '/data' /proc/self/mountinfo | awk '{print $1}' | head -1"
	mountID, err := kubectlExec(namespace, podName, []string{"bash", "-c", mountCmd})
	if err != nil {
		return nil, fmt.Errorf("failed to get mount ID: %v", err)
	}
	data["mountID"] = strings.TrimSpace(mountID)

	// 5. Get inode number of mount point
	inode, err := kubectlExec(namespace, podName, []string{"stat", "-c", "%i", "/data"})
	if err != nil {
		return nil, fmt.Errorf("failed to get inode: %v", err)
	}
	data["inodeNumber"] = strings.TrimSpace(inode)

	// 6. Write initial test data
	testData := fmt.Sprintf("Pre-upgrade data written at: %s", time.Now().Format(time.RFC3339))
	writeCmd := fmt.Sprintf("echo '%s' > /data/test.txt", testData)
	if _, err := kubectlExec(namespace, podName, []string{"bash", "-c", writeCmd}); err != nil {
		return nil, fmt.Errorf("failed to write test data: %v", err)
	}

	return data, nil
}

func verifyAfterUpgrade(beforeData map[string]string, podName, namespace string) error {
	fmt.Printf("\n=== Mount Continuity Verification ===\n")
	fmt.Printf("Pod: %s/%s\n", namespace, podName)
	fmt.Printf("Mount Path: /data\n")
	fmt.Printf("Original capture time: %s\n", beforeData["captureTime"])
	fmt.Println("-------------------------------------")

	results := []string{}
	failed := false

	// 1. Check if mount process PID is unchanged
	pidCmd := "ps aux | grep '[m]ountpoint-s3' | awk '{print $2}' | head -1"
	currentPID, err := kubectlExec(namespace, podName, []string{"bash", "-c", pidCmd})
	if err != nil {
		results = append(results, "❌ Cannot check mount process")
		failed = true
	} else {
		currentPID = strings.TrimSpace(currentPID)
		if currentPID != beforeData["mountPID"] {
			results = append(results, fmt.Sprintf("❌ Mount PID changed: %s → %s (REMOUNTED!)",
				beforeData["mountPID"], currentPID))
			failed = true
		} else {
			results = append(results, fmt.Sprintf("✅ Mount PID unchanged: %s", currentPID))
		}
	}

	// 2. Check process start time (if PID exists and unchanged)
	if !failed && beforeData["mountPID"] != "" {
		startCmd := fmt.Sprintf("stat -c %%Y /proc/%s 2>/dev/null || echo 'no-process'", beforeData["mountPID"])
		currentStart, err := kubectlExec(namespace, podName, []string{"bash", "-c", startCmd})
		if err != nil {
			results = append(results, "❌ Cannot check process start time")
			failed = true
		} else {
			currentStart = strings.TrimSpace(currentStart)
			if currentStart != beforeData["processStartTime"] {
				results = append(results, "❌ Process restarted (REMOUNTED!)")
				failed = true
			} else {
				results = append(results, "✅ Process start time unchanged")
			}
		}
	}

	// 3. Check mount ID
	mountCmd := "grep '/data' /proc/self/mountinfo | awk '{print $1}' | head -1"
	currentMountID, err := kubectlExec(namespace, podName, []string{"bash", "-c", mountCmd})
	if err != nil {
		results = append(results, "❌ Cannot check mount ID")
		failed = true
	} else {
		currentMountID = strings.TrimSpace(currentMountID)
		if currentMountID != beforeData["mountID"] {
			results = append(results, fmt.Sprintf("❌ Mount ID changed: %s → %s (REMOUNTED!)",
				beforeData["mountID"], currentMountID))
			failed = true
		} else {
			results = append(results, fmt.Sprintf("✅ Mount ID unchanged: %s", currentMountID))
		}
	}

	// 4. Check if open file handle is still valid
	if beforeData["writerPID"] != "" {
		fdCmd := fmt.Sprintf("ls -l /proc/%s/fd/3 2>/dev/null | grep lock-file || echo 'no-fd'", beforeData["writerPID"])
		fdCheck, err := kubectlExec(namespace, podName, []string{"bash", "-c", fdCmd})

		if err != nil || !strings.Contains(fdCheck, "lock-file") {
			results = append(results, "❌ Open file handle broken (REMOUNTED!)")
			failed = true
		} else {
			results = append(results, "✅ Open file handle still valid")
		}
	}

	// 5. Check for gaps in continuous write log
	logCmd := "tail -100 /data/continuous.log 2>/dev/null || echo 'no-log'"
	logData, err := kubectlExec(namespace, podName, []string{"bash", "-c", logCmd})

	if err != nil || logData == "no-log" {
		results = append(results, "❌ Continuous log not found")
		failed = true
	} else {
		maxGap := checkForGaps(logData)
		if maxGap > 2.0 { // Allow up to 2 second gap
			results = append(results, fmt.Sprintf("❌ Found %.2f second gap in writes (DISRUPTED!)", maxGap))
			failed = true
		} else {
			results = append(results, fmt.Sprintf("✅ No significant gaps (max: %.2fs)", maxGap))
		}
	}

	// 6. Test current read/write capability
	testFile := fmt.Sprintf("/data/post-upgrade-%d.txt", time.Now().Unix())
	testContent := fmt.Sprintf("Post-upgrade write test at %s", time.Now().Format(time.RFC3339))
	writeCmd := fmt.Sprintf("echo '%s' > %s && cat %s", testContent, testFile, testFile)
	writeResult, err := kubectlExec(namespace, podName, []string{"bash", "-c", writeCmd})

	if err != nil || !strings.Contains(writeResult, testContent) {
		results = append(results, "❌ Cannot read/write to mount")
		failed = true
	} else {
		results = append(results, "✅ Read/write operations working")
	}

	// 7. Verify pre-upgrade data still exists
	preDataCmd := "cat /data/test.txt 2>/dev/null || echo 'no-file'"
	preData, err := kubectlExec(namespace, podName, []string{"bash", "-c", preDataCmd})

	if err != nil || !strings.Contains(preData, "Pre-upgrade data") {
		results = append(results, "❌ Pre-upgrade data lost")
		failed = true
	} else {
		results = append(results, "✅ Pre-upgrade data intact")
	}

	// Print all results
	for _, result := range results {
		fmt.Println(result)
	}
	fmt.Println("-------------------------------------")

	if failed {
		fmt.Println("❌ RESULT: Mount was disrupted during upgrade")
		return fmt.Errorf("mount was disrupted during upgrade")
	}

	fmt.Println("✅ VERIFIED: Mount survived upgrade without remounting!")
	return nil
}

func applyManifest(manifest string) error {
	// Write manifest to temp file and apply
	tempFile := fmt.Sprintf("/tmp/manifest-%d.yaml", time.Now().UnixNano())
	if err := os.WriteFile(tempFile, []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("failed to write temp manifest: %v", err)
	}
	defer func() { _ = os.Remove(tempFile) }()

	return sh.Run("kubectl", "apply", "-f", tempFile)
}

func kubectlExec(namespace, podName string, command []string) (string, error) {
	args := []string{"exec", "-n", namespace, podName, "--"}
	args = append(args, command...)
	return sh.Output("kubectl", args...)
}

func getUpgradeTestNamespace() string {
	if ns := os.Getenv("UPGRADE_TEST_NAMESPACE"); ns != "" {
		return ns
	}
	return defaultUpgradeTestNamespace
}

func getUpgradeTestType() string {
	testType := os.Getenv("UPGRADE_TEST_TYPE")
	if testType == "" {
		return "all"
	}
	return testType
}

func getUpgradeTestTimeout() string {
	timeout := os.Getenv("UPGRADE_TEST_TIMEOUT")
	if timeout == "" {
		return "120s"
	}
	return timeout
}

func getTestPods(testType string) []string {
	var pods []string
	if testType == "all" || testType == "static" {
		pods = append(pods, "static-test-pod")
	}
	if testType == "all" || testType == "dynamic" {
		pods = append(pods, "dynamic-test-pod")
	}
	return pods
}

func saveMountDataSimple(filePath string, data map[string]string) error {
	// Simple key=value format
	var lines []string
	for k, v := range data {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	content := strings.Join(lines, "\n")
	return os.WriteFile(filePath, []byte(content), 0o644)
}

func loadMountDataSimple(filePath string) (map[string]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	data := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			data[parts[0]] = parts[1]
		}
	}

	return data, nil
}

func checkForGaps(logData string) float64 {
	lines := strings.Split(strings.TrimSpace(logData), "\n")
	if len(lines) < 2 {
		return 0
	}

	var maxGap float64
	for i := 1; i < len(lines); i++ {
		if lines[i-1] == "" || lines[i] == "" {
			continue
		}

		prev, err1 := strconv.ParseFloat(strings.TrimSpace(lines[i-1]), 64)
		curr, err2 := strconv.ParseFloat(strings.TrimSpace(lines[i]), 64)

		if err1 != nil || err2 != nil {
			continue // Skip malformed timestamps
		}

		gap := curr - prev
		if gap > maxGap {
			maxGap = gap
		}
	}

	return maxGap
}
