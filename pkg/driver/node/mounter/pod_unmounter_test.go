package mounter

import (
	"os"
	"path/filepath"
	"testing"

	mpmounter "github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint/mounter"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
)

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
			mount := &mpmounter.Mounter{}
			unmounter := NewPodUnmounter(nodeID, mount, nil, nil)

			if unmounter == nil {
				t.Fatal("NewPodUnmounter() returned nil")
			}
			if unmounter.nodeID != nodeID {
				t.Errorf("Expected nodeID %s, got %s", nodeID, unmounter.nodeID)
			}
			if unmounter.kubeletPath != tt.expectedPath {
				t.Errorf("Expected kubeletPath %s, got %s", tt.expectedPath, unmounter.kubeletPath)
			}
			if unmounter.mount != mount {
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
