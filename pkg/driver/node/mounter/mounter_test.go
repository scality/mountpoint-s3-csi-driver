package mounter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
)

func TestMountS3Path(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "uses environment variable when set",
			envValue: "/custom/path/mount-s3",
			expected: "/custom/path/mount-s3",
		},
		{
			name:     "uses default when environment variable is empty",
			envValue: "",
			expected: defaultMountS3Path,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Save original env value
			originalEnv := os.Getenv(MountS3PathEnv)
			defer func() {
				if originalEnv != "" {
					_ = os.Setenv(MountS3PathEnv, originalEnv)
				} else {
					_ = os.Unsetenv(MountS3PathEnv)
				}
			}()

			// Set test env value
			if tt.envValue != "" {
				_ = os.Setenv(MountS3PathEnv, tt.envValue)
			} else {
				_ = os.Unsetenv(MountS3PathEnv)
			}

			result := MountS3Path()
			if result != tt.expected {
				t.Errorf("MountS3Path() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSourceMountDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		kubeletPath string
		expected    string
	}{
		{
			name:        "standard kubelet path",
			kubeletPath: "/var/lib/kubelet",
			expected:    filepath.Join("/var/lib/kubelet", "plugins", constants.DriverName, "mnt"),
		},
		{
			name:        "custom kubelet path",
			kubeletPath: "/custom/kubelet",
			expected:    filepath.Join("/custom/kubelet", "plugins", constants.DriverName, "mnt"),
		},
		{
			name:        "root path",
			kubeletPath: "/",
			expected:    filepath.Join("/", "plugins", constants.DriverName, "mnt"),
		},
		{
			name:        "empty path",
			kubeletPath: "",
			expected:    filepath.Join("", "plugins", constants.DriverName, "mnt"),
		},
		{
			name:        "path with trailing slash",
			kubeletPath: "/var/lib/kubelet/",
			expected:    filepath.Join("/var/lib/kubelet/", "plugins", constants.DriverName, "mnt"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := SourceMountDir(tt.kubeletPath)
			if result != tt.expected {
				t.Errorf("SourceMountDir(%q) = %q, want %q", tt.kubeletPath, result, tt.expected)
			}

			// Verify the result uses the driver name constant
			expectedWithConstant := filepath.Join(tt.kubeletPath, "plugins", constants.DriverName, "mnt")
			if result != expectedWithConstant {
				t.Errorf("SourceMountDir() should use constants.DriverName, got %q, want %q", result, expectedWithConstant)
			}
		})
	}
}

func TestSourceMountDirUsesDriverNameConstant(t *testing.T) {
	t.Parallel()

	kubeletPath := "/var/lib/kubelet"
	result := SourceMountDir(kubeletPath)

	// Verify it contains the driver name from constants
	expectedPath := filepath.Join(kubeletPath, "plugins", constants.DriverName, "mnt")
	if result != expectedPath {
		t.Errorf("SourceMountDir() = %q, want %q (using constants.DriverName)", result, expectedPath)
	}

	// Verify it uses Driver Name
	expectedPathWithConstant := filepath.Join(kubeletPath, "plugins", constants.DriverName, "mnt")
	if result != expectedPathWithConstant {
		t.Error("SourceMountDir() should use constants.DriverName for driver name")
	}

	// Verify the path components are correct
	expectedComponents := []string{kubeletPath, "plugins", constants.DriverName, "mnt"}
	expectedJoined := filepath.Join(expectedComponents...)
	if result != expectedJoined {
		t.Errorf("SourceMountDir() path components incorrect, got %q, want %q", result, expectedJoined)
	}
}
