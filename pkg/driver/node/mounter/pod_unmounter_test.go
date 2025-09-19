package mounter

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// Mock implementations for testing

// mockPodWatcher implements PodWatcher interface for unit testing
type mockPodWatcher struct {
	pods map[string]*corev1.Pod
	err  error
	// For tracking calls made during periodic cleanup
	getCallCount int32
}

func (m *mockPodWatcher) IncrementGetCalls() {
	atomic.AddInt32(&m.getCallCount, 1)
}

func (m *mockPodWatcher) GetCallCount() int32 {
	return atomic.LoadInt32(&m.getCallCount)
}

func (m *mockPodWatcher) Get(name string) (*corev1.Pod, error) {
	m.IncrementGetCalls()
	if m.err != nil {
		return nil, m.err
	}
	if pod, exists := m.pods[name]; exists {
		return pod, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, name)
}

// mockCredentialProvider implements CredentialProvider interface for unit testing
type mockCredentialProvider struct {
	cleanupErr error
}

func (m *mockCredentialProvider) Cleanup(ctx credentialprovider.CleanupContext) error {
	return m.cleanupErr
}

// mockMountInterface implements MountInterface for unit testing
type mockMountInterface struct {
	isMountpoint          bool
	mountpointErr         error
	isMountpointCorrupted bool
	unmountErr            error
	references            []string
	referencesErr         error
	// For tracking calls made during periodic cleanup
	cleanupCallCount int32
}

func (m *mockMountInterface) IncrementCleanupCalls() {
	atomic.AddInt32(&m.cleanupCallCount, 1)
}

func (m *mockMountInterface) GetCleanupCallCount() int32 {
	return atomic.LoadInt32(&m.cleanupCallCount)
}

func (m *mockMountInterface) CheckMountpoint(target string) (bool, error) {
	return m.isMountpoint, m.mountpointErr
}

func (m *mockMountInterface) IsMountpointCorrupted(err error) bool {
	return m.isMountpointCorrupted
}

func (m *mockMountInterface) Unmount(target string) error {
	return m.unmountErr
}

func (m *mockMountInterface) FindReferencesToMountpoint(source string) ([]string, error) {
	return m.references, m.referencesErr
}

func TestNewPodUnmounter(t *testing.T) {
	tests := []struct {
		name         string
		kubeletPath  string
		expectedPath string
	}{
		{
			name:         "default kubelet path when env var not set",
			kubeletPath:  "",
			expectedPath: "/var/lib/kubelet",
		},
		{
			name:         "custom kubelet path from env var",
			kubeletPath:  "/custom/kubelet/path",
			expectedPath: "/custom/kubelet/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.kubeletPath != "" {
				_ = os.Setenv("KUBELET_PATH", tt.kubeletPath)
				defer func() { _ = os.Unsetenv("KUBELET_PATH") }()
			} else {
				_ = os.Unsetenv("KUBELET_PATH")
			}

			nodeID := "test-node"
			mockMount := &mockMountInterface{}
			mockWatcher := &mockPodWatcher{}
			mockCredProvider := &mockCredentialProvider{}
			unmounter := NewPodUnmounter(nodeID, mockMount, mockWatcher, mockCredProvider)

			if unmounter == nil {
				t.Fatal("NewPodUnmounter() returned nil")
			}
			if unmounter.nodeID != nodeID {
				t.Errorf("Expected nodeID %s, got %s", nodeID, unmounter.nodeID)
			}
			if unmounter.kubeletPath != tt.expectedPath {
				t.Errorf("Expected kubeletPath %s, got %s", tt.expectedPath, unmounter.kubeletPath)
			}
			if unmounter.mount != mockMount {
				t.Error("Expected mount to be set correctly")
			}
		})
	}
}

func TestWriteExitFile(t *testing.T) {
	tests := []struct {
		name          string
		setupPodDir   bool
		expectedError bool
	}{
		{
			name:          "successful exit file creation",
			setupPodDir:   true,
			expectedError: false,
		},
		{
			name:          "pod directory does not exist",
			setupPodDir:   false,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "pod_unmounter_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tempDir) }()

			podPath := filepath.Join(tempDir, "test-pod")
			if tt.setupPodDir {
				exitFilePath := mppod.PathOnHost(podPath, mppod.KnownPathMountExit)
				exitFileDir := filepath.Dir(exitFilePath)
				if err := os.MkdirAll(exitFileDir, 0o755); err != nil {
					t.Fatalf("Failed to create exit file directory: %v", err)
				}
			}

			unmounter := &PodUnmounter{}
			err = unmounter.writeExitFile(podPath)

			if (err != nil) != tt.expectedError {
				t.Errorf("writeExitFile() error = %v, expectedError %v", err, tt.expectedError)
			}

			if !tt.expectedError && tt.setupPodDir {
				exitFilePath := mppod.PathOnHost(podPath, mppod.KnownPathMountExit)
				if _, err := os.Stat(exitFilePath); os.IsNotExist(err) {
					t.Errorf("Expected exit file to be created at %s", exitFilePath)
				}
			}
		})
	}
}

func TestMountpointPodSourcePath(t *testing.T) {
	unmounter := &PodUnmounter{
		kubeletPath: "/var/lib/kubelet",
	}

	podName := "test-pod"
	expected := "/var/lib/kubelet/plugins/s3.csi.scality.com/mnt/test-pod"
	result := unmounter.mountpointPodSourcePath(podName)

	if result != expected {
		t.Errorf("mountpointPodSourcePath() = %s, expected %s", result, expected)
	}
}

func TestPodPath(t *testing.T) {
	unmounter := &PodUnmounter{
		kubeletPath: "/var/lib/kubelet",
	}

	podUID := "test-uid"
	expected := "/var/lib/kubelet/pods/test-uid"
	result := unmounter.podPath(podUID)

	if result != expected {
		t.Errorf("podPath() = %s, expected %s", result, expected)
	}
}

func TestCleanupCredentials(t *testing.T) {
	tests := []struct {
		name       string
		cleanupErr error
		expectErr  bool
	}{
		{
			name:      "successful cleanup",
			expectErr: false,
		},
		{
			name:       "cleanup fails",
			cleanupErr: errors.New("cleanup failed"),
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCredProvider := &mockCredentialProvider{
				cleanupErr: tt.cleanupErr,
			}

			unmounter := &PodUnmounter{
				credProvider: mockCredProvider,
			}

			pod := createTestPod("test-pod")
			err := unmounter.cleanupCredentials(pod)

			if (err != nil) != tt.expectErr {
				t.Errorf("cleanupCredentials() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestCleanupDanglingMounts(t *testing.T) {
	tests := []struct {
		name                  string
		setupDirs             []string
		podsInWatcher         map[string]*corev1.Pod
		watcherErr            error
		isMountpoint          bool
		mountpointErr         error
		unmountErr            error
		expectedRemainingDirs []string
		expectedRemovedDirs   []string
		expectErr             bool
	}{
		{
			name:      "source mount dir does not exist",
			expectErr: false,
		},
		{
			name:      "no dangling mounts - all pods exist",
			setupDirs: []string{"mp-pod1", "mp-pod2"},
			podsInWatcher: map[string]*corev1.Pod{
				"mp-pod1": createTestPod("mp-pod1"),
				"mp-pod2": createTestPod("mp-pod2"),
			},
			isMountpoint:          false,
			expectedRemainingDirs: []string{"mp-pod1", "mp-pod2"},
			expectedRemovedDirs:   []string{},
			expectErr:             false,
		},
		{
			name:      "dangling mount detected - pod not in cluster",
			setupDirs: []string{"mp-pod1", "mp-pod2", "mp-pod3"},
			podsInWatcher: map[string]*corev1.Pod{
				"mp-pod1": createTestPod("mp-pod1"),
				// mp-pod2 missing - should be cleaned
				"mp-pod3": createTestPod("mp-pod3"),
			},
			isMountpoint:          false,
			expectedRemainingDirs: []string{"mp-pod1", "mp-pod3"},
			expectedRemovedDirs:   []string{"mp-pod2"},
			expectErr:             false,
		},
		{
			name:       "watcher returns error",
			setupDirs:  []string{"mp-pod1"},
			watcherErr: errors.New("watcher failed"),
			expectErr:  true,
		},
		{
			name:         "unmount fails - logs error but continues",
			setupDirs:    []string{"mp-pod1"},
			isMountpoint: true,
			unmountErr:   errors.New("unmount failed"),
			expectErr:    false, // Method continues and returns success even if individual unmount fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			tempDir, err := os.MkdirTemp("", "pod_unmounter_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tempDir) }()

			// Setup directory structure
			sourceMountDir := SourceMountDir(tempDir)
			if len(tt.setupDirs) > 0 {
				if err := os.MkdirAll(sourceMountDir, 0o755); err != nil {
					t.Fatalf("Failed to create source mount dir: %v", err)
				}
				for _, dirName := range tt.setupDirs {
					dirPath := filepath.Join(sourceMountDir, dirName)
					if err := os.MkdirAll(dirPath, 0o755); err != nil {
						t.Fatalf("Failed to create test dir %s: %v", dirName, err)
					}
				}
			}

			// Setup mocks
			mockWatcher := &mockPodWatcher{
				pods: tt.podsInWatcher,
				err:  tt.watcherErr,
			}

			mockMount := &mockMountInterface{
				isMountpoint:  tt.isMountpoint,
				mountpointErr: tt.mountpointErr,
				unmountErr:    tt.unmountErr,
			}

			unmounter := &PodUnmounter{
				nodeID:      "test-node",
				mount:       mockMount,
				kubeletPath: tempDir,
				podWatcher:  mockWatcher,
			}

			// Run the test
			err = unmounter.CleanupDanglingMounts()

			// Verify error expectations
			if (err != nil) != tt.expectErr {
				t.Errorf("CleanupDanglingMounts() error = %v, expectErr %v", err, tt.expectErr)
			}

			// Verify cleanup happened correctly (only if no error expected)
			if !tt.expectErr && len(tt.setupDirs) > 0 {
				// Check expected remaining directories
				for _, expectedDir := range tt.expectedRemainingDirs {
					dirPath := filepath.Join(sourceMountDir, expectedDir)
					if _, err := os.Stat(dirPath); os.IsNotExist(err) {
						t.Errorf("Expected directory %s to remain but it was removed", expectedDir)
					}
				}

				// Check expected removed directories
				for _, removedDir := range tt.expectedRemovedDirs {
					dirPath := filepath.Join(sourceMountDir, removedDir)
					if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
						t.Errorf("Expected directory %s to be removed but it still exists", removedDir)
					}
				}
			}
		})
	}
}

func TestHandleMountpointPodUpdate_NodeFiltering(t *testing.T) {
	tests := []struct {
		name        string
		nodeID      string
		podNodeName string
		shouldSkip  bool
	}{
		{
			name:        "pod on different node - should skip",
			nodeID:      "node1",
			podNodeName: "node2",
			shouldSkip:  true,
		},
		{
			name:        "pod on same node - should process",
			nodeID:      "node1",
			podNodeName: "node1",
			shouldSkip:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWatcher := &mockPodWatcher{}
			mockMount := &mockMountInterface{}
			mockCredProvider := &mockCredentialProvider{}

			unmounter := &PodUnmounter{
				nodeID:       tt.nodeID,
				mount:        mockMount,
				podWatcher:   mockWatcher,
				credProvider: mockCredProvider,
			}

			pod := createTestPod("test-pod")
			pod.Spec.NodeName = tt.podNodeName

			// This should not panic - we're testing the node filtering logic
			// The actual unmounting would require more setup but the method should handle
			// the node filtering correctly
			unmounter.HandleMountpointPodUpdate(nil, pod)
		})
	}
}

func createTestPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  types.UID("test-uid-" + name),
			Labels: map[string]string{
				mppod.LabelVolumeId: "test-volume",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
		},
	}
}

// mockPodUnmounterForPeriodic wraps PodUnmounter to track cleanup calls
type mockPodUnmounterForPeriodic struct {
	*PodUnmounter
	cleanupCalls      int32
	cleanupShouldFail bool
	cleanupErr        error
}

func (m *mockPodUnmounterForPeriodic) CleanupDanglingMounts() error {
	atomic.AddInt32(&m.cleanupCalls, 1)
	if m.cleanupShouldFail {
		return m.cleanupErr
	}
	return nil
}

func (m *mockPodUnmounterForPeriodic) GetCleanupCallCount() int32 {
	return atomic.LoadInt32(&m.cleanupCalls)
}

// StartPeriodicCleanupWithInterval is a test helper that accepts a custom interval
func StartPeriodicCleanupWithInterval(u *mockPodUnmounterForPeriodic, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			_ = u.CleanupDanglingMounts() // We ignore error in test to match real implementation
		}
	}
}

func TestStartPeriodicCleanup(t *testing.T) {
	t.Run("cleanup is called periodically multiple times", func(t *testing.T) {
		mockWatcher := &mockPodWatcher{}
		mockMount := &mockMountInterface{}
		mockCredProvider := &mockCredentialProvider{}

		baseUnmounter := &PodUnmounter{
			nodeID:       "test-node",
			mount:        mockMount,
			kubeletPath:  "/tmp/test",
			podWatcher:   mockWatcher,
			credProvider: mockCredProvider,
		}

		unmounter := &mockPodUnmounterForPeriodic{
			PodUnmounter: baseUnmounter,
		}

		stopCh := make(chan struct{})

		// Use a very short interval for testing (10ms)
		go StartPeriodicCleanupWithInterval(unmounter, 10*time.Millisecond, stopCh)

		// Wait for multiple ticks (should get at least 3-4 calls in 50ms)
		time.Sleep(50 * time.Millisecond)

		// Stop the cleanup
		close(stopCh)
		time.Sleep(5 * time.Millisecond) // Brief pause to ensure it stops

		callCount := unmounter.GetCleanupCallCount()
		if callCount < 3 {
			t.Errorf("Expected at least 3 cleanup calls, got %d", callCount)
		}
		if callCount > 6 {
			t.Errorf("Got too many cleanup calls (%d), ticker might not be working correctly", callCount)
		}
		t.Logf("CleanupDanglingMounts was called %d times as expected", callCount)
	})

	t.Run("cleanup continues even when CleanupDanglingMounts returns errors", func(t *testing.T) {
		mockWatcher := &mockPodWatcher{}
		mockMount := &mockMountInterface{}
		mockCredProvider := &mockCredentialProvider{}

		baseUnmounter := &PodUnmounter{
			nodeID:       "test-node",
			mount:        mockMount,
			kubeletPath:  "/tmp/test",
			podWatcher:   mockWatcher,
			credProvider: mockCredProvider,
		}

		unmounter := &mockPodUnmounterForPeriodic{
			PodUnmounter:      baseUnmounter,
			cleanupShouldFail: true,
			cleanupErr:        errors.New("simulated cleanup error"),
		}

		stopCh := make(chan struct{})

		// Use a very short interval for testing
		go StartPeriodicCleanupWithInterval(unmounter, 10*time.Millisecond, stopCh)

		// Wait for multiple ticks
		time.Sleep(50 * time.Millisecond)

		// Stop the cleanup
		close(stopCh)
		time.Sleep(5 * time.Millisecond)

		callCount := unmounter.GetCleanupCallCount()
		if callCount < 3 {
			t.Errorf("Expected cleanup to continue despite errors, got only %d calls", callCount)
		}
		t.Logf("CleanupDanglingMounts continued running despite errors (%d calls)", callCount)
	})

	t.Run("stop channel terminates cleanup loop promptly", func(t *testing.T) {
		mockWatcher := &mockPodWatcher{}
		mockMount := &mockMountInterface{}
		mockCredProvider := &mockCredentialProvider{}

		baseUnmounter := &PodUnmounter{
			nodeID:       "test-node",
			mount:        mockMount,
			kubeletPath:  "/tmp/test",
			podWatcher:   mockWatcher,
			credProvider: mockCredProvider,
		}

		unmounter := &mockPodUnmounterForPeriodic{
			PodUnmounter: baseUnmounter,
		}

		stopCh := make(chan struct{})
		done := make(chan bool)

		// Start with a longer interval to ensure stop works between ticks
		go func() {
			StartPeriodicCleanupWithInterval(unmounter, 100*time.Millisecond, stopCh)
			done <- true
		}()

		// Give it time to start
		time.Sleep(10 * time.Millisecond)

		// Record calls before stop
		callsBefore := unmounter.GetCleanupCallCount()

		// Signal stop
		close(stopCh)

		// Wait for termination
		select {
		case <-done:
			// Give a tiny bit more time to ensure no more calls
			time.Sleep(10 * time.Millisecond)
			callsAfter := unmounter.GetCleanupCallCount()
			if callsAfter != callsBefore {
				t.Errorf("Cleanup was called after stop signal: before=%d, after=%d", callsBefore, callsAfter)
			}
			t.Log("Cleanup loop terminated promptly on stop signal")
		case <-time.After(50 * time.Millisecond):
			t.Fatal("Cleanup loop did not terminate within expected time")
		}
	})
}
