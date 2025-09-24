package mounter

import (
	"errors"
	"os"
	"testing"

	"k8s.io/mount-utils"
)

// mockMountInterface implements mount.Interface for testing
type mockMountInterface struct {
	mountPoints     []mount.MountPoint
	listError       error
	unmountError    error
	unmountCalls    []string
	mountRefs       map[string][]string
	getMountRefsErr error
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
	if m.getMountRefsErr != nil {
		return nil, m.getMountRefsErr
	}
	if m.mountRefs != nil {
		if refs, ok := m.mountRefs[pathname]; ok {
			return refs, nil
		}
	}
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

func TestFindReferencesToMountpoint(t *testing.T) {
	tests := []struct {
		name               string
		source             string
		mountRefs          map[string][]string
		getMountRefsErr    error
		expectedReferences []string
		expectedError      bool
	}{
		{
			name:               "no references found",
			source:             "/mnt/source",
			mountRefs:          map[string][]string{"/mnt/source": {}},
			expectedReferences: []string{},
			expectedError:      false,
		},
		{
			name:               "single bind mount reference",
			source:             "/mnt/source",
			mountRefs:          map[string][]string{"/mnt/source": {"/mnt/bind1"}},
			expectedReferences: []string{"/mnt/bind1"},
			expectedError:      false,
		},
		{
			name:               "multiple bind mount references",
			source:             "/mnt/source",
			mountRefs:          map[string][]string{"/mnt/source": {"/mnt/bind1", "/mnt/bind2"}},
			expectedReferences: []string{"/mnt/bind1", "/mnt/bind2"},
			expectedError:      false,
		},
		{
			name:               "three bind mount references",
			source:             "/mnt/source",
			mountRefs:          map[string][]string{"/mnt/source": {"/mnt/bind1", "/mnt/bind2", "/mnt/bind3"}},
			expectedReferences: []string{"/mnt/bind1", "/mnt/bind2", "/mnt/bind3"},
			expectedError:      false,
		},
		{
			name:               "GetMountRefs returns error",
			source:             "/mnt/source",
			getMountRefsErr:    errors.New("failed to get mount refs"),
			expectedReferences: nil,
			expectedError:      true,
		},
		{
			name:               "path not in map returns empty list",
			source:             "/mnt/unknown",
			mountRefs:          map[string][]string{"/mnt/source": {"/mnt/bind1"}},
			expectedReferences: []string{},
			expectedError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMounter := &mockMountInterface{
				mountRefs:       tt.mountRefs,
				getMountRefsErr: tt.getMountRefsErr,
			}

			mounter := NewMounter(mockMounter)
			references, err := mounter.FindReferencesToMountpoint(tt.source)

			if (err != nil) != tt.expectedError {
				t.Errorf("FindReferencesToMountpoint() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if tt.expectedError {
				return // Don't check references if we expected an error
			}

			// Compare slices
			if len(references) != len(tt.expectedReferences) {
				t.Errorf("FindReferencesToMountpoint() references count = %d, expected %d", len(references), len(tt.expectedReferences))
				return
			}

			// Create a map for easier comparison since order might not matter
			expectedMap := make(map[string]bool)
			for _, ref := range tt.expectedReferences {
				expectedMap[ref] = true
			}

			for _, ref := range references {
				if !expectedMap[ref] {
					t.Errorf("FindReferencesToMountpoint() found unexpected reference %s", ref)
				}
				delete(expectedMap, ref)
			}

			// Check if any expected references were missed
			for missed := range expectedMap {
				t.Errorf("FindReferencesToMountpoint() missed expected reference %s", missed)
			}
		})
	}
}
