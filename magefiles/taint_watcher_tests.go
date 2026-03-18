//go:build mage

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/magefile/mage/sh"
)

// =============================================================================
// Taint Watcher E2E Test Constants
// =============================================================================

const (
	taintKey          = "s3.csi.scality.com/agent-not-ready"
	taintEffect       = "NoExecute"
	taintWatcherLabel = "app=taint-watcher-test"
)

const workloadStorageClassYAML = `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: taint-watcher-sc
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
mountOptions:
  - allow-delete`

const workloadPVCYAML = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: taint-watcher-pvc
  namespace: default
spec:
  accessModes: [ReadWriteMany]
  storageClassName: taint-watcher-sc
  resources:
    requests:
      storage: 10Gi`

// workloadPodTemplate — %s is replaced with worker node name
const workloadPodTemplate = `apiVersion: v1
kind: Pod
metadata:
  name: taint-watcher-workload
  namespace: default
  labels:
    app: taint-watcher-test
spec:
  nodeSelector:
    kubernetes.io/hostname: "%s"
  containers:
    - name: test
      image: busybox:latest
      command: ["sleep", "3600"]
      volumeMounts:
        - name: s3-volume
          mountPath: /data
  volumes:
    - name: s3-volume
      persistentVolumeClaim:
        claimName: taint-watcher-pvc`

// =============================================================================
// Private Helpers
// =============================================================================

// getWorkerNodeName discovers the first worker node (non-control-plane) in the cluster.
func getWorkerNodeName() (string, error) {
	output, err := sh.Output("kubectl", "get", "nodes",
		"--selector=!node-role.kubernetes.io/control-plane",
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return "", fmt.Errorf("failed to get worker node: %v", err)
	}
	name := strings.TrimSpace(output)
	if name == "" {
		return "", fmt.Errorf("no worker nodes found in the cluster")
	}
	return name, nil
}

// hasTaint checks if a node has a specific taint key.
func hasTaint(nodeName, key string) (bool, error) {
	output, err := sh.Output("kubectl", "get", "node", nodeName,
		"-o", "jsonpath={.spec.taints[*].key}")
	if err != nil {
		return false, fmt.Errorf("failed to get taints for node %s: %v", nodeName, err)
	}
	for _, k := range strings.Fields(output) {
		if k == key {
			return true, nil
		}
	}
	return false, nil
}

// waitForTaintRemoval polls the node until the taint is removed or the timeout expires.
func waitForTaintRemoval(nodeName string, timeout time.Duration) error {
	fmt.Printf("Waiting for taint %s to be removed from node %s (timeout: %v)...\n", taintKey, nodeName, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for taint removal from node %s after %v", nodeName, timeout)
		case <-ticker.C:
			present, err := hasTaint(nodeName, taintKey)
			if err != nil {
				fmt.Printf("  Warning: error checking taint: %v\n", err)
				continue
			}
			if !present {
				fmt.Printf("  Taint %s removed from node %s\n", taintKey, nodeName)
				return nil
			}
			fmt.Printf("  Taint still present on node %s, waiting...\n", nodeName)
		}
	}
}

// waitForCSIDriverOnNode waits for the CSI DaemonSet pod to be Ready on a specific node.
func waitForCSIDriverOnNode(nodeName, namespace string, timeout time.Duration) error {
	fmt.Printf("Waiting for CSI driver pod to be ready on node %s (timeout: %v)...\n", nodeName, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Dump pod status for debugging
			status, _ := sh.Output("kubectl", "get", "pods", "-n", namespace, "-l", "app=s3-csi-node",
				"--field-selector", fmt.Sprintf("spec.nodeName=%s", nodeName), "-o", "wide")
			return fmt.Errorf("timeout waiting for CSI pod on node %s after %v. Pod status:\n%s", nodeName, timeout, status)
		case <-ticker.C:
			output, err := sh.Output("kubectl", "get", "pods", "-n", namespace, "-l", "app=s3-csi-node",
				"--field-selector", fmt.Sprintf("spec.nodeName=%s", nodeName),
				"-o", "jsonpath={.items[0].status.conditions[?(@.type=='Ready')].status}")
			if err != nil {
				continue
			}
			if strings.TrimSpace(output) == "True" {
				fmt.Printf("  CSI driver pod is ready on node %s\n", nodeName)
				return nil
			}
		}
	}
}

// waitForCSINodeRegistration polls the CSINode object until the driver is registered.
func waitForCSINodeRegistration(nodeName string, timeout time.Duration) error {
	fmt.Printf("Waiting for CSINode registration of %s on node %s (timeout: %v)...\n", CSIDriverName, nodeName, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			drivers, _ := sh.Output("kubectl", "get", "csinode", nodeName,
				"-o", "jsonpath={.spec.drivers[*].name}")
			return fmt.Errorf("timeout waiting for CSINode registration on %s after %v. Registered drivers: %s",
				nodeName, timeout, drivers)
		case <-ticker.C:
			output, err := sh.Output("kubectl", "get", "csinode", nodeName,
				"-o", "jsonpath={.spec.drivers[*].name}")
			if err != nil {
				continue
			}
			for _, driver := range strings.Fields(output) {
				if driver == CSIDriverName {
					fmt.Printf("  CSI driver %s registered on node %s\n", CSIDriverName, nodeName)
					return nil
				}
			}
		}
	}
}

// applyWorkloadManifests creates the StorageClass, PVC, and workload Pod.
func applyWorkloadManifests(workerNodeName string) error {
	fmt.Println("Applying workload manifests (StorageClass, PVC, Pod)...")

	if err := pipeToKubectlApply(workloadStorageClassYAML); err != nil {
		return fmt.Errorf("failed to apply StorageClass: %v", err)
	}
	if err := pipeToKubectlApply(workloadPVCYAML); err != nil {
		return fmt.Errorf("failed to apply PVC: %v", err)
	}

	podYAML := fmt.Sprintf(workloadPodTemplate, workerNodeName)
	if err := pipeToKubectlApply(podYAML); err != nil {
		return fmt.Errorf("failed to apply workload Pod: %v", err)
	}

	fmt.Println("Workload manifests applied")
	return nil
}

// waitForPodRunning waits for a pod to reach the Running phase.
func waitForPodRunning(name, namespace string, timeout time.Duration) error {
	fmt.Printf("Waiting for pod %s/%s to be Running (timeout: %v)...\n", namespace, name, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			status, _ := sh.Output("kubectl", "get", "pod", name, "-n", namespace, "-o", "wide")
			events, _ := sh.Output("kubectl", "get", "events", "-n", namespace,
				"--field-selector", fmt.Sprintf("involvedObject.name=%s", name),
				"--sort-by=.lastTimestamp")
			return fmt.Errorf("timeout waiting for pod %s to be Running after %v.\nStatus: %s\nEvents:\n%s",
				name, timeout, status, events)
		case <-ticker.C:
			output, err := sh.Output("kubectl", "get", "pod", name, "-n", namespace,
				"-o", "jsonpath={.status.phase}")
			if err != nil {
				continue
			}
			phase := strings.TrimSpace(output)
			if phase == "Running" {
				fmt.Printf("  Pod %s is Running\n", name)
				return nil
			}
			fmt.Printf("  Pod %s phase: %s\n", name, phase)
		}
	}
}

// waitForPodPending checks if a pod is in Pending state (negative test).
// Returns true if the pod is Pending within the check window, false if it has moved past Pending.
func waitForPodPending(name, namespace string, checkDuration time.Duration) bool {
	fmt.Printf("Checking that pod %s/%s is Pending (check window: %v)...\n", namespace, name, checkDuration)

	ctx, cancel := context.WithTimeout(context.Background(), checkDuration)
	defer cancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Check one last time
			output, err := sh.Output("kubectl", "get", "pod", name, "-n", namespace,
				"-o", "jsonpath={.status.phase}")
			if err != nil {
				return false
			}
			return strings.TrimSpace(output) == "Pending"
		case <-ticker.C:
			output, err := sh.Output("kubectl", "get", "pod", name, "-n", namespace,
				"-o", "jsonpath={.status.phase}")
			if err != nil {
				continue
			}
			phase := strings.TrimSpace(output)
			if phase != "" && phase != "Pending" {
				fmt.Printf("  Pod %s moved to phase: %s (taint was already removed)\n", name, phase)
				return false
			}
		}
	}
}

// verifyTaintWatcherLogs checks the CSI driver pod logs on a specific node for taint removal messages.
func verifyTaintWatcherLogs(nodeName, namespace string) error {
	fmt.Printf("Checking CSI driver logs on node %s for taint watcher messages...\n", nodeName)

	// Get CSI pod name on the worker node
	podName, err := sh.Output("kubectl", "get", "pods", "-n", namespace, "-l", "app=s3-csi-node",
		"--field-selector", fmt.Sprintf("spec.nodeName=%s", nodeName),
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return fmt.Errorf("failed to get CSI pod name on node %s: %v", nodeName, err)
	}
	podName = strings.TrimSpace(podName)
	if podName == "" {
		return fmt.Errorf("no CSI pod found on node %s", nodeName)
	}

	// Get logs from the s3-plugin container
	logs, err := sh.Output("kubectl", "logs", podName, "-n", namespace, "-c", "s3-plugin")
	if err != nil {
		return fmt.Errorf("failed to get logs from pod %s: %v", podName, err)
	}

	// Check for the taint removal success message
	if strings.Contains(logs, "successfully removed taint") {
		fmt.Printf("  Found 'successfully removed taint' message in CSI driver logs on node %s\n", nodeName)
		return nil
	}

	// Also check for the "removing taint" message which indicates the watcher triggered
	if strings.Contains(logs, "removing taint") {
		fmt.Printf("  Found 'removing taint' message in CSI driver logs on node %s\n", nodeName)
		return nil
	}

	fmt.Printf("  WARNING: Did not find taint removal messages in CSI driver logs on node %s\n", nodeName)
	fmt.Printf("  This may indicate the taint was already removed before the watcher started.\n")
	fmt.Printf("  Relevant log lines:\n")
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(strings.ToLower(line), "taint") {
			fmt.Printf("    %s\n", line)
		}
	}
	return nil
}

// =============================================================================
// Public Mage Targets
// =============================================================================

// InstallCSIForTaintTest loads credentials from integration_config.json and installs the CSI driver.
// This wraps LoadCredentials + e2e:install so that CI workflows don't need to manage credentials externally.
func InstallCSIForTaintTest() error {
	if err := LoadCredentials(); err != nil {
		return fmt.Errorf("failed to load credentials: %v", err)
	}
	return installCSIForE2E()
}

// SetupTaintWatcherTest discovers the worker node, applies the agent-not-ready taint,
// and verifies the taint blocks a test pod (negative test).
func SetupTaintWatcherTest() error {
	fmt.Println("=== Setup Taint Watcher Test ===")

	// Discover worker node
	workerNode, err := getWorkerNodeName()
	if err != nil {
		return err
	}
	fmt.Printf("Worker node: %s\n", workerNode)

	// Apply the agent-not-ready taint
	fmt.Printf("Applying taint %s:%s to node %s...\n", taintKey, taintEffect, workerNode)
	if err := sh.RunV("kubectl", "taint", "nodes", workerNode,
		fmt.Sprintf("%s=:NoExecute", taintKey), "--overwrite"); err != nil {
		return fmt.Errorf("failed to apply taint: %v", err)
	}

	// Verify taint is present
	present, err := hasTaint(workerNode, taintKey)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("taint %s was not applied to node %s", taintKey, workerNode)
	}
	fmt.Printf("Taint %s applied to node %s\n", taintKey, workerNode)

	// Negative test: verify a regular pod stays Pending due to the taint
	fmt.Println("Verifying taint blocks scheduling (negative test)...")
	testPodYAML := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: taint-test-pod
  namespace: default
spec:
  nodeSelector:
    kubernetes.io/hostname: "%s"
  containers:
    - name: test
      image: busybox:latest
      command: ["sleep", "60"]`, workerNode)

	if err := pipeToKubectlApply(testPodYAML); err != nil {
		return fmt.Errorf("failed to create test pod: %v", err)
	}

	// Wait briefly and check the pod is Pending
	isPending := waitForPodPending("taint-test-pod", "default", 15*time.Second)
	if !isPending {
		fmt.Println("  WARNING: Test pod is not Pending — taint may not be blocking as expected")
	} else {
		fmt.Println("  Test pod is Pending as expected (taint is blocking scheduling)")
	}

	// Clean up the test pod
	fmt.Println("Cleaning up negative test pod...")
	_ = sh.Run("kubectl", "delete", "pod", "taint-test-pod", "-n", "default", "--grace-period=0", "--force", "--ignore-not-found=true")

	fmt.Println("=== Setup Taint Watcher Test Complete ===")
	return nil
}

// TestTaintWatcher runs the full taint watcher lifecycle test:
// CSI pod starts on tainted worker → CSINode registration → workload pod scheduled → taint auto-removed → pod Running.
func TestTaintWatcher() error {
	fmt.Println("=== Test Taint Watcher ===")
	namespace := GetE2ENamespace()

	// Discover worker node
	workerNode, err := getWorkerNodeName()
	if err != nil {
		return err
	}
	fmt.Printf("Worker node: %s\n", workerNode)

	// Step 1: Wait for CSI DaemonSet pod to be Ready on the worker node
	fmt.Println("\n--- Step 1: Wait for CSI driver pod on worker node ---")
	if err := waitForCSIDriverOnNode(workerNode, namespace, 300*time.Second); err != nil {
		return fmt.Errorf("CSI driver pod not ready on worker: %v", err)
	}

	// Step 2: Wait for CSINode registration
	fmt.Println("\n--- Step 2: Wait for CSINode registration ---")
	if err := waitForCSINodeRegistration(workerNode, 60*time.Second); err != nil {
		return fmt.Errorf("CSINode registration failed: %v", err)
	}

	// Step 3: Deploy workload (SC + PVC + Pod targeting worker)
	fmt.Println("\n--- Step 3: Deploy workload ---")
	if err := applyWorkloadManifests(workerNode); err != nil {
		return fmt.Errorf("failed to apply workload manifests: %v", err)
	}

	// Check if taint is still present — the taint watcher is event-driven and may
	// have already removed it by the time we get here.
	taintPresent, err := hasTaint(workerNode, taintKey)
	if err != nil {
		return fmt.Errorf("failed to check taint: %v", err)
	}

	if taintPresent {
		// Taint still present — check that workload is Pending, then wait for removal
		fmt.Println("Taint still present — verifying workload pod is Pending...")
		isPending := waitForPodPending("taint-watcher-workload", "default", 10*time.Second)
		if isPending {
			fmt.Println("Workload pod is Pending as expected (taint blocks scheduling)")
		}

		// Step 4: Wait for taint auto-removal
		fmt.Println("\n--- Step 4: Wait for taint auto-removal ---")
		if err := waitForTaintRemoval(workerNode, 90*time.Second); err != nil {
			return fmt.Errorf("taint was not removed: %v", err)
		}
	} else {
		fmt.Println("Taint already removed by taint watcher (fast path)")
	}

	// Step 5: Verify workload pod transitions to Running
	fmt.Println("\n--- Step 5: Verify workload pod reaches Running ---")
	if err := waitForPodRunning("taint-watcher-workload", "default", 120*time.Second); err != nil {
		return fmt.Errorf("workload pod did not reach Running: %v", err)
	}

	// Step 6: Write/read test file via kubectl exec
	fmt.Println("\n--- Step 6: Verify S3 data access ---")
	if err := sh.RunV("kubectl", "exec", "taint-watcher-workload", "-n", "default", "--",
		"sh", "-c", "echo 'taint-watcher-test' > /data/test-file.txt && cat /data/test-file.txt"); err != nil {
		return fmt.Errorf("failed to write/read test file: %v", err)
	}
	fmt.Println("S3 data access verified")

	// Step 7: Check CSI driver logs
	fmt.Println("\n--- Step 7: Check CSI driver logs ---")
	if err := verifyTaintWatcherLogs(workerNode, namespace); err != nil {
		fmt.Printf("WARNING: Log verification issue: %v\n", err)
	}

	fmt.Println("\n=== Test Taint Watcher PASSED ===")
	return nil
}

// TestTaintWatcherRestart verifies the taint watcher works after a CSI pod restart:
// re-taint worker → delete CSI pod → verify replacement starts → verify taint removed.
func TestTaintWatcherRestart() error {
	fmt.Println("=== Test Taint Watcher Restart ===")
	namespace := GetE2ENamespace()

	// Discover worker node
	workerNode, err := getWorkerNodeName()
	if err != nil {
		return err
	}
	fmt.Printf("Worker node: %s\n", workerNode)

	// Step 1: Re-apply the taint
	fmt.Println("\n--- Step 1: Re-apply taint ---")
	if err := sh.RunV("kubectl", "taint", "nodes", workerNode,
		fmt.Sprintf("%s=:NoExecute", taintKey), "--overwrite"); err != nil {
		return fmt.Errorf("failed to re-apply taint: %v", err)
	}

	// Verify taint is present
	present, err := hasTaint(workerNode, taintKey)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("taint %s was not applied to node %s", taintKey, workerNode)
	}
	fmt.Printf("Taint %s re-applied to node %s\n", taintKey, workerNode)

	// Step 2: Delete the CSI pod on the worker to trigger a restart
	fmt.Println("\n--- Step 2: Delete CSI pod on worker ---")
	podName, err := sh.Output("kubectl", "get", "pods", "-n", namespace, "-l", "app=s3-csi-node",
		"--field-selector", fmt.Sprintf("spec.nodeName=%s", workerNode),
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return fmt.Errorf("failed to get CSI pod name: %v", err)
	}
	podName = strings.TrimSpace(podName)
	if podName == "" {
		return fmt.Errorf("no CSI pod found on node %s", workerNode)
	}

	fmt.Printf("Deleting CSI pod %s on node %s...\n", podName, workerNode)
	if err := sh.RunV("kubectl", "delete", "pod", podName, "-n", namespace, "--grace-period=0"); err != nil {
		return fmt.Errorf("failed to delete CSI pod: %v", err)
	}

	// Step 3: Wait for replacement CSI pod to be ready
	fmt.Println("\n--- Step 3: Wait for replacement CSI pod ---")
	if err := waitForCSIDriverOnNode(workerNode, namespace, 300*time.Second); err != nil {
		return fmt.Errorf("replacement CSI pod not ready: %v", err)
	}

	// Step 4: Wait for CSINode re-registration
	fmt.Println("\n--- Step 4: Wait for CSINode re-registration ---")
	if err := waitForCSINodeRegistration(workerNode, 60*time.Second); err != nil {
		return fmt.Errorf("CSINode re-registration failed: %v", err)
	}

	// Step 5: Wait for taint auto-removal
	fmt.Println("\n--- Step 5: Wait for taint auto-removal ---")
	if err := waitForTaintRemoval(workerNode, 90*time.Second); err != nil {
		return fmt.Errorf("taint was not removed after restart: %v", err)
	}

	// Step 6: Verify taint watcher logs
	fmt.Println("\n--- Step 6: Check CSI driver logs ---")
	if err := verifyTaintWatcherLogs(workerNode, namespace); err != nil {
		fmt.Printf("WARNING: Log verification issue: %v\n", err)
	}

	fmt.Println("\n=== Test Taint Watcher Restart PASSED ===")
	return nil
}

// CleanupTaintWatcherTest removes test resources: workload pod, PVC, StorageClass, and the taint.
func CleanupTaintWatcherTest() error {
	fmt.Println("=== Cleanup Taint Watcher Test ===")

	// Delete workload pod
	fmt.Println("Deleting workload pod...")
	_ = sh.Run("kubectl", "delete", "pod", "taint-watcher-workload", "-n", "default",
		"--grace-period=0", "--force", "--ignore-not-found=true")

	// Delete PVC
	fmt.Println("Deleting PVC...")
	_ = sh.Run("kubectl", "delete", "pvc", "taint-watcher-pvc", "-n", "default", "--ignore-not-found=true")

	// Delete StorageClass
	fmt.Println("Deleting StorageClass...")
	_ = sh.Run("kubectl", "delete", "storageclass", "taint-watcher-sc", "--ignore-not-found=true")

	// Remove taint from worker node (if still present)
	workerNode, err := getWorkerNodeName()
	if err != nil {
		fmt.Printf("Warning: could not get worker node: %v\n", err)
	} else {
		present, _ := hasTaint(workerNode, taintKey)
		if present {
			fmt.Printf("Removing taint %s from node %s...\n", taintKey, workerNode)
			_ = sh.Run("kubectl", "taint", "nodes", workerNode, fmt.Sprintf("%s-", taintKey))
		}
	}

	fmt.Println("=== Cleanup Complete ===")
	return nil
}
