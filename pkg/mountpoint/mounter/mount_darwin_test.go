//go:build darwin

package mounter

import (
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
)

func TestVerifyMountPoint_Darwin(t *testing.T) {
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

func TestOpenFUSEDevice_Darwin(t *testing.T) {
	// On Darwin, FUSE operations are not supported
	_, err := OpenFUSEDevice()
	if err == nil {
		t.Error("OpenFUSEDevice() should return an error on Darwin")
	}

	expectedErrMsg := "FUSE device operations only supported on Linux"
	if err.Error() != expectedErrMsg {
		t.Errorf("OpenFUSEDevice() error = %v, expected %v", err, expectedErrMsg)
	}
}

func TestCloseFUSEDevice_Darwin(t *testing.T) {
	// On Darwin, this should be a no-op - just verify it doesn't panic
	CloseFUSEDevice(0)
	CloseFUSEDevice(-1)
	CloseFUSEDevice(999)
}

func TestCreateMountOptions_Darwin(t *testing.T) {
	args := mountpoint.ParseArgs([]string{})
	options, err := CreateMountOptions(0, "/tmp", args)

	if err == nil {
		t.Error("CreateMountOptions() should return an error on Darwin")
	}

	if options != nil {
		t.Error("CreateMountOptions() should return nil options on Darwin")
	}

	expectedErrMsg := "FUSE mount options only supported on Linux"
	if err.Error() != expectedErrMsg {
		t.Errorf("CreateMountOptions() error = %v, expected %v", err, expectedErrMsg)
	}
}

func TestCreateMountFlags_Darwin(t *testing.T) {
	args := mountpoint.ParseArgs([]string{})
	flags := CreateMountFlags(args)

	if flags != 0 {
		t.Errorf("CreateMountFlags() = %v, expected 0 on Darwin", flags)
	}
}

func TestPerformMount_Darwin(t *testing.T) {
	err := PerformMount("/tmp", []string{}, 0)

	if err == nil {
		t.Error("PerformMount() should return an error on Darwin")
	}

	expectedErrMsg := "mount syscall only supported on Linux"
	if err.Error() != expectedErrMsg {
		t.Errorf("PerformMount() error = %v, expected %v", err, expectedErrMsg)
	}
}
