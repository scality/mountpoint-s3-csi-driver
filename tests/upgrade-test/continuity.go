// Package upgradetest provides mount continuity verification for CSI driver upgrades.
package upgradetest

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// MountContinuityData captures mount details before upgrade
type MountContinuityData struct {
	PodName          string    `json:"podName"`
	Namespace        string    `json:"namespace"`
	MountPath        string    `json:"mountPath"`
	TestType         TestType  `json:"testType"`
	MountPID         string    `json:"mountPID"`         // PID of mountpoint-s3 process
	ProcessStartTime string    `json:"processStartTime"` // When mount process started
	MountID          string    `json:"mountID"`          // Linux mount ID
	InodeNumber      string    `json:"inodeNumber"`      // Inode of mount point
	WriterPID        string    `json:"writerPID"`        // PID of continuous writer
	CaptureTime      time.Time `json:"captureTime"`      // When data was captured
}

// ExecFunc is a function type for executing commands in pods
type ExecFunc func(namespace, podName string, command []string) (string, error)

// CaptureBeforeUpgrade captures mount state before upgrade to verify continuity later
func CaptureBeforeUpgrade(execFunc ExecFunc, podName, namespace string) (*MountContinuityData, error) {
	data := &MountContinuityData{
		PodName:     podName,
		Namespace:   namespace,
		MountPath:   "/data",
		CaptureTime: time.Now(),
	}

	fmt.Printf("Capturing mount state for %s...\n", podName)

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

	writerPID, err := execFunc(namespace, podName, []string{"bash", "-c", script})
	if err != nil {
		return nil, fmt.Errorf("failed to start continuous writer: %v", err)
	}
	data.WriterPID = strings.TrimSpace(writerPID)

	// 2. Get mountpoint-s3 process PID
	pidCmd := "ps aux | grep '[m]ountpoint-s3' | awk '{print $2}' | head -1"
	pid, err := execFunc(namespace, podName, []string{"bash", "-c", pidCmd})
	if err != nil {
		return nil, fmt.Errorf("failed to get mount PID: %v", err)
	}
	data.MountPID = strings.TrimSpace(pid)

	if data.MountPID == "" {
		return nil, fmt.Errorf("mountpoint-s3 process not found")
	}

	// 3. Get process start time
	startCmd := fmt.Sprintf("stat -c %%Y /proc/%s 2>/dev/null || echo 'no-process'", data.MountPID)
	startTime, err := execFunc(namespace, podName, []string{"bash", "-c", startCmd})
	if err != nil {
		return nil, fmt.Errorf("failed to get process start time: %v", err)
	}
	data.ProcessStartTime = strings.TrimSpace(startTime)

	// 4. Get mount ID from mountinfo
	mountCmd := "grep '/data' /proc/self/mountinfo | awk '{print $1}' | head -1"
	mountID, err := execFunc(namespace, podName, []string{"bash", "-c", mountCmd})
	if err != nil {
		return nil, fmt.Errorf("failed to get mount ID: %v", err)
	}
	data.MountID = strings.TrimSpace(mountID)

	// 5. Get inode number of mount point
	inode, err := execFunc(namespace, podName, []string{"stat", "-c", "%i", "/data"})
	if err != nil {
		return nil, fmt.Errorf("failed to get inode: %v", err)
	}
	data.InodeNumber = strings.TrimSpace(inode)

	// 6. Write initial test data
	testData := fmt.Sprintf("Pre-upgrade data written at: %s", time.Now().Format(time.RFC3339))
	writeCmd := fmt.Sprintf("echo '%s' > /data/test.txt", testData)
	if _, err := execFunc(namespace, podName, []string{"bash", "-c", writeCmd}); err != nil {
		return nil, fmt.Errorf("failed to write test data: %v", err)
	}

	fmt.Printf("✓ Captured mount state:\n")
	fmt.Printf("  - Mount PID: %s\n", data.MountPID)
	fmt.Printf("  - Mount ID: %s\n", data.MountID)
	fmt.Printf("  - Writer PID: %s\n", data.WriterPID)
	fmt.Printf("  - Inode: %s\n", data.InodeNumber)

	return data, nil
}

// VerifyAfterUpgrade checks if mount survived without remounting
func VerifyAfterUpgrade(execFunc ExecFunc, beforeData *MountContinuityData) error {
	fmt.Printf("\n=== Mount Continuity Verification ===\n")
	fmt.Printf("Pod: %s/%s\n", beforeData.Namespace, beforeData.PodName)
	fmt.Printf("Mount Path: %s\n", beforeData.MountPath)
	fmt.Printf("Original capture time: %s\n", beforeData.CaptureTime.Format(time.RFC3339))
	fmt.Println("-------------------------------------")

	results := []string{}
	failed := false

	// 1. Check if mount process PID is unchanged
	pidCmd := "ps aux | grep '[m]ountpoint-s3' | awk '{print $2}' | head -1"
	currentPID, err := execFunc(beforeData.Namespace, beforeData.PodName,
		[]string{"bash", "-c", pidCmd})
	if err != nil {
		results = append(results, "❌ Cannot check mount process")
		failed = true
	} else {
		currentPID = strings.TrimSpace(currentPID)
		if currentPID != beforeData.MountPID {
			results = append(results, fmt.Sprintf("❌ Mount PID changed: %s → %s (REMOUNTED!)",
				beforeData.MountPID, currentPID))
			failed = true
		} else {
			results = append(results, fmt.Sprintf("✅ Mount PID unchanged: %s", currentPID))
		}
	}

	// 2. Check process start time (if PID exists and unchanged)
	if !failed && beforeData.MountPID != "" {
		startCmd := fmt.Sprintf("stat -c %%Y /proc/%s 2>/dev/null || echo 'no-process'", beforeData.MountPID)
		currentStart, err := execFunc(beforeData.Namespace, beforeData.PodName,
			[]string{"bash", "-c", startCmd})
		if err != nil {
			results = append(results, "❌ Cannot check process start time")
			failed = true
		} else {
			currentStart = strings.TrimSpace(currentStart)
			if currentStart != beforeData.ProcessStartTime {
				results = append(results, "❌ Process restarted (REMOUNTED!)")
				failed = true
			} else {
				results = append(results, "✅ Process start time unchanged")
			}
		}
	}

	// 3. Check mount ID
	mountCmd := "grep '/data' /proc/self/mountinfo | awk '{print $1}' | head -1"
	currentMountID, err := execFunc(beforeData.Namespace, beforeData.PodName,
		[]string{"bash", "-c", mountCmd})
	if err != nil {
		results = append(results, "❌ Cannot check mount ID")
		failed = true
	} else {
		currentMountID = strings.TrimSpace(currentMountID)
		if currentMountID != beforeData.MountID {
			results = append(results, fmt.Sprintf("❌ Mount ID changed: %s → %s (REMOUNTED!)",
				beforeData.MountID, currentMountID))
			failed = true
		} else {
			results = append(results, fmt.Sprintf("✅ Mount ID unchanged: %s", currentMountID))
		}
	}

	// 4. Check if open file handle is still valid
	if beforeData.WriterPID != "" {
		fdCmd := fmt.Sprintf("ls -l /proc/%s/fd/3 2>/dev/null | grep lock-file || echo 'no-fd'", beforeData.WriterPID)
		fdCheck, err := execFunc(beforeData.Namespace, beforeData.PodName,
			[]string{"bash", "-c", fdCmd})

		if err != nil || !strings.Contains(fdCheck, "lock-file") {
			results = append(results, "❌ Open file handle broken (REMOUNTED!)")
			failed = true
		} else {
			results = append(results, "✅ Open file handle still valid")
		}
	}

	// 5. Check for gaps in continuous write log
	logCmd := "tail -100 /data/continuous.log 2>/dev/null || echo 'no-log'"
	logData, err := execFunc(beforeData.Namespace, beforeData.PodName,
		[]string{"bash", "-c", logCmd})

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
	writeResult, err := execFunc(beforeData.Namespace, beforeData.PodName,
		[]string{"bash", "-c", writeCmd})

	if err != nil || !strings.Contains(writeResult, testContent) {
		results = append(results, "❌ Cannot read/write to mount")
		failed = true
	} else {
		results = append(results, "✅ Read/write operations working")
	}

	// 7. Verify pre-upgrade data still exists
	preDataCmd := "cat /data/test.txt 2>/dev/null || echo 'no-file'"
	preData, err := execFunc(beforeData.Namespace, beforeData.PodName,
		[]string{"bash", "-c", preDataCmd})

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

// checkForGaps analyzes continuous log for time gaps
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

// SaveMountData saves mount continuity data to a file
func SaveMountData(data *MountContinuityData, filePath string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0o644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

// LoadMountData loads mount continuity data from a file
func LoadMountData(filePath string) (*MountContinuityData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var mountData MountContinuityData
	if err := json.Unmarshal(data, &mountData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return &mountData, nil
}

// WaitForPodReady waits for a pod to become ready
func WaitForPodReady(execFunc ExecFunc, namespace, podName string, timeout time.Duration) error {
	fmt.Printf("Waiting for pod %s to be ready...\n", podName)

	start := time.Now()
	for time.Since(start) < timeout {
		// Check pod phase
		phaseCmd := fmt.Sprintf("kubectl get pod %s -n %s -o jsonpath='{.status.phase}' 2>/dev/null", podName, namespace)
		phase, err := execFunc("", "", []string{"bash", "-c", phaseCmd})

		if err == nil && strings.TrimSpace(phase) == "Running" {
			// Double check by trying to exec into pod
			if _, err := execFunc(namespace, podName, []string{"echo", "ready"}); err == nil {
				fmt.Printf("✓ Pod %s is ready\n", podName)
				return nil
			}
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("pod %s not ready after %v", podName, timeout)
}

// VerifyBasicMount performs basic mount verification without continuity checks
func VerifyBasicMount(execFunc ExecFunc, namespace, podName string) error {
	fmt.Printf("Verifying basic mount functionality for %s...\n", podName)

	// 1. Check mount point exists
	if _, err := execFunc(namespace, podName, []string{"ls", "-la", "/data"}); err != nil {
		return fmt.Errorf("mount point /data not accessible: %v", err)
	}

	// 2. Check if it's a FUSE mount
	mountCmd := "mount | grep '/data'"
	mountInfo, err := execFunc(namespace, podName, []string{"bash", "-c", mountCmd})
	if err != nil || !strings.Contains(mountInfo, "fuse") {
		return fmt.Errorf("mount is not FUSE: %s", mountInfo)
	}

	// 3. Test write capability
	testFile := fmt.Sprintf("/data/basic-test-%d.txt", time.Now().Unix())
	testContent := "Basic mount test"
	writeCmd := fmt.Sprintf("echo '%s' > %s && cat %s", testContent, testFile, testContent)
	result, err := execFunc(namespace, podName, []string{"bash", "-c", writeCmd})

	if err != nil || !strings.Contains(result, testContent) {
		return fmt.Errorf("write test failed: %v", err)
	}

	fmt.Printf("✓ Basic mount verification passed for %s\n", podName)
	return nil
}
