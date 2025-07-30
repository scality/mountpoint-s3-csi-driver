//go:build linux

package mounter

import (
	"os"
	"syscall"
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
)

func TestVerifyMountPoint(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		expectedError bool
	}{
		{
			name:          "existing directory",
			path:          "/tmp",
			expectedError: false,
		},
		{
			name:          "non-existing path",
			path:          "/non/existing/path",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyMountPoint(tt.path)
			if (err != nil) != tt.expectedError {
				t.Errorf("VerifyMountPoint() error = %v, expectedError %v", err, tt.expectedError)
			}
		})
	}
}

func TestOpenAndCloseFUSEDevice(t *testing.T) {
	// Skip if /dev/fuse doesn't exist (common in containers)
	if _, err := os.Stat("/dev/fuse"); os.IsNotExist(err) {
		t.Skip("Skipping test because /dev/fuse doesn't exist")
	}

	fd, err := OpenFUSEDevice()
	if err != nil {
		t.Fatalf("OpenFUSEDevice() failed: %v", err)
	}

	if fd <= 0 {
		t.Errorf("OpenFUSEDevice() returned invalid fd: %d", fd)
	}

	CloseFUSEDevice(fd)

	// Verify the fd is actually closed by attempting to use it
	var stat syscall.Stat_t
	err = syscall.Fstat(fd, &stat)
	if err == nil {
		t.Error("CloseFUSEDevice() did not close the file descriptor")
	}
}

func TestCreateMountOptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mount_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name          string
		fd            int
		target        string
		args          mountpoint.Args
		expectedError bool
		expectOptions []string
	}{
		{
			name:          "basic mount options",
			fd:            3,
			target:        tmpDir,
			args:          mountpoint.ParseArgs([]string{}),
			expectedError: false,
			expectOptions: []string{"fd=3", "default_permissions"},
		},
		{
			name:          "with allow-other flag",
			fd:            3,
			target:        tmpDir,
			args:          mountpoint.ParseArgs([]string{mountpoint.ArgAllowOther}),
			expectedError: false,
			expectOptions: []string{"fd=3", "default_permissions", "allow_other"},
		},
		{
			name:          "with allow-root flag",
			fd:            3,
			target:        tmpDir,
			args:          mountpoint.ParseArgs([]string{mountpoint.ArgAllowRoot}),
			expectedError: false,
			expectOptions: []string{"fd=3", "default_permissions", "allow_other"},
		},
		{
			name:          "non-existing target",
			fd:            3,
			target:        "/non/existing/path",
			args:          mountpoint.ParseArgs([]string{}),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options, err := CreateMountOptions(tt.fd, tt.target, tt.args)

			if (err != nil) != tt.expectedError {
				t.Errorf("CreateMountOptions() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if !tt.expectedError {
				// Check if all expected options are present
				for _, expectedOption := range tt.expectOptions {
					found := false
					for _, option := range options {
						if option == expectedOption {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("CreateMountOptions() missing expected option: %s, got: %v", expectedOption, options)
					}
				}

				// Verify fd option is present and correct
				found := false
				for _, option := range options {
					if option == "fd=3" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("CreateMountOptions() should include fd option, got: %v", options)
				}
			}
		})
	}
}

func TestCreateMountFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          mountpoint.Args
		expectedFlags uintptr
	}{
		{
			name:          "default flags",
			args:          mountpoint.ParseArgs([]string{}),
			expectedFlags: syscall.MS_NODEV | syscall.MS_NOSUID | syscall.MS_NOATIME,
		},
		{
			name:          "with read-only flag",
			args:          mountpoint.ParseArgs([]string{mountpoint.ArgReadOnly}),
			expectedFlags: syscall.MS_NODEV | syscall.MS_NOSUID | syscall.MS_NOATIME | syscall.MS_RDONLY,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := CreateMountFlags(tt.args)

			if flags != tt.expectedFlags {
				t.Errorf("CreateMountFlags() = %v, expected %v", flags, tt.expectedFlags)
			}
		})
	}
}

func TestPerformMount(t *testing.T) {
	// This test cannot perform actual mount operations without root privileges
	// and appropriate setup, so we'll test the function signature and basic validation
	tests := []struct {
		name        string
		target      string
		options     []string
		flags       uintptr
		expectError bool
	}{
		{
			name:        "basic parameters",
			target:      "/mnt/test",
			options:     []string{"fd=3", "default_permissions"},
			flags:       syscall.MS_NODEV,
			expectError: true, // Expected to fail due to permissions/setup
		},
		{
			name:        "empty target",
			target:      "",
			options:     []string{},
			flags:       0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PerformMount(tt.target, tt.options, tt.flags)

			if (err != nil) != tt.expectError {
				// For permission-related errors, just verify that the function
				// doesn't panic and returns an error as expected
				if err != nil && tt.expectError {
					// This is expected - we can't actually mount without proper setup
					return
				}
				t.Errorf("PerformMount() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}
