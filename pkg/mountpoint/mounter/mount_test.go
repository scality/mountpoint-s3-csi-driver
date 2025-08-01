package mounter

import (
	"errors"
	"os"
	"testing"

	"k8s.io/mount-utils"
)

// mockMountInterface implements mount.Interface for testing
type mockMountInterface struct {
	mountPoints  []mount.MountPoint
	listError    error
	unmountError error
	unmountCalls []string
}

func (m *mockMountInterface) Mount(source, target, fstype string, options []string) error {
	return nil
}

func (m *mockMountInterface) MountSensitive(source, target, fstype string, options, sensitiveOptions []string) error {
	return nil
}

func (m *mockMountInterface) MountSensitiveWithoutSystemd(source, target, fstype string, options, sensitiveOptions []string) error {
	return nil
}

func (m *mockMountInterface) MountSensitiveWithoutSystemdWithMountFlags(source, target, fstype string, options, sensitiveOptions, mountFlags []string) error {
	return nil
}

func (m *mockMountInterface) Unmount(target string) error {
	m.unmountCalls = append(m.unmountCalls, target)
	return m.unmountError
}

func (m *mockMountInterface) List() ([]mount.MountPoint, error) {
	return m.mountPoints, m.listError
}

func (m *mockMountInterface) IsMountPoint(file string) (bool, error) {
	return false, nil
}

func (m *mockMountInterface) IsLikelyNotMountPoint(file string) (bool, error) {
	return true, nil
}

func (m *mockMountInterface) CanSafelySkipMountPointCheck() bool {
	return false
}

func (m *mockMountInterface) GetMountRefs(pathname string) ([]string, error) {
	return []string{}, nil
}

func TestCheckMountpoint(t *testing.T) {
	tmpDir1, err := os.MkdirTemp("", "mount_test_1")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir1) }()

	tmpDir2, err := os.MkdirTemp("", "mount_test_2")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir2) }()

	tmpDir3, err := os.MkdirTemp("", "mount_test_3")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir3) }()

	tests := []struct {
		name           string
		target         string
		mountPoints    []mount.MountPoint
		listError      error
		expectedResult bool
		expectedError  bool
	}{
		{
			name:   "target is mountpoint-s3 mount",
			target: tmpDir1,
			mountPoints: []mount.MountPoint{
				{Path: tmpDir1, Device: MountpointDeviceName},
				{Path: tmpDir2, Device: "tmpfs"},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:   "target is not mountpoint-s3 mount",
			target: tmpDir2,
			mountPoints: []mount.MountPoint{
				{Path: tmpDir1, Device: MountpointDeviceName},
				{Path: tmpDir2, Device: "tmpfs"},
			},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:   "target not found in mount points",
			target: tmpDir3,
			mountPoints: []mount.MountPoint{
				{Path: tmpDir1, Device: MountpointDeviceName},
			},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:           "list mounts fails",
			target:         tmpDir1,
			listError:      errors.New("failed to list mounts"),
			expectedResult: false,
			expectedError:  true,
		},
		{
			name:           "empty mount points list",
			target:         tmpDir1,
			mountPoints:    []mount.MountPoint{},
			expectedResult: false,
			expectedError:  false,
		},
		{
			name:   "multiple mounts with same path, different devices",
			target: tmpDir3,
			mountPoints: []mount.MountPoint{
				{Path: tmpDir3, Device: "tmpfs"},
				{Path: tmpDir3, Device: MountpointDeviceName},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name:   "non-existing target path",
			target: "/non/existing/path",
			mountPoints: []mount.MountPoint{
				{Path: "/non/existing/path", Device: MountpointDeviceName},
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMounter := &mockMountInterface{
				mountPoints: tt.mountPoints,
				listError:   tt.listError,
			}

			result, err := CheckMountpoint(mockMounter, tt.target)

			if (err != nil) != tt.expectedError {
				t.Errorf("CheckMountpoint() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if result != tt.expectedResult {
				t.Errorf("CheckMountpoint() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}

func TestUnmountTarget(t *testing.T) {
	tests := []struct {
		name          string
		target        string
		unmountError  error
		expectedError bool
	}{
		{
			name:          "successful unmount",
			target:        "/mnt/s3",
			unmountError:  nil,
			expectedError: false,
		},
		{
			name:          "unmount fails",
			target:        "/mnt/s3",
			unmountError:  errors.New("unmount failed"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMounter := &mockMountInterface{
				unmountError: tt.unmountError,
			}

			err := UnmountTarget(mockMounter, tt.target)

			if (err != nil) != tt.expectedError {
				t.Errorf("UnmountTarget() error = %v, expectedError %v", err, tt.expectedError)
			}

			// Verify the unmount was called with correct target
			if len(mockMounter.unmountCalls) != 1 || mockMounter.unmountCalls[0] != tt.target {
				t.Errorf("UnmountTarget() called with %v, expected %v", mockMounter.unmountCalls, []string{tt.target})
			}
		})
	}
}

func TestConstructors(t *testing.T) {
	mockInterface := &mockMountInterface{}
	mounter := NewMounter(mockInterface)
	if mounter == nil || mounter.mountutils == nil {
		t.Fatal("NewMounter() failed to create valid mounter")
	}

	defaultMounter := NewDefaultMounter()
	if defaultMounter == nil || defaultMounter.mountutils == nil {
		t.Fatal("NewDefaultMounter() failed to create valid mounter")
	}
}
