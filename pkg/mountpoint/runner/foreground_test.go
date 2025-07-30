package runner

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
)

func TestRunInForeground_Validation(t *testing.T) {
	tests := []struct {
		name        string
		opts        ForegroundOptions
		expectedErr error
	}{
		{
			name: "missing binary path",
			opts: ForegroundOptions{
				BucketName: "test-bucket",
				Fd:         3,
			},
			expectedErr: ErrMissingBinaryPath,
		},
		{
			name: "missing bucket name",
			opts: ForegroundOptions{
				BinaryPath: "/usr/bin/mount-s3",
				Fd:         3,
			},
			expectedErr: ErrMissingBucketName,
		},
		{
			name: "invalid file descriptor",
			opts: ForegroundOptions{
				BinaryPath: "/usr/bin/mount-s3",
				BucketName: "test-bucket",
				Fd:         -1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := RunInForeground(tt.opts)

			if tt.expectedErr != nil {
				if err == nil {
					t.Errorf("RunInForeground() expected error %v but got none", tt.expectedErr)
				} else if !errors.Is(err, tt.expectedErr) {
					t.Errorf("RunInForeground() error = %v, expected %v", err, tt.expectedErr)
				}
			} else if tt.name == "invalid file descriptor" {
				if err == nil {
					t.Errorf("RunInForeground() expected error for invalid fd but got none")
				}
			}
		})
	}
}

func TestRunInForeground_MockRunner(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-fuse")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	fd := int(tmpFile.Fd())

	tests := []struct {
		name         string
		mockExitCode ExitCode
		mockError    error
		mockStderr   string
		expectedExit ExitCode
		expectError  bool
	}{
		{
			name:         "successful execution",
			mockExitCode: 0,
			mockError:    nil,
			expectedExit: 0,
			expectError:  false,
		},
		{
			name:         "failed execution",
			mockExitCode: 1,
			mockError:    errors.New("command failed"),
			mockStderr:   "mount error details",
			expectedExit: 1,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := func(cmd *exec.Cmd) (ExitCode, error) {
				if len(cmd.Args) < 3 {
					t.Errorf("Expected at least 3 args (binary, bucket, mountpoint), got %d", len(cmd.Args))
				}

				if cmd.Args[1] != "test-bucket" {
					t.Errorf("Expected bucket name 'test-bucket', got %q", cmd.Args[1])
				}

				if cmd.Args[2] != "/dev/fd/3" {
					t.Errorf("Expected mountpoint '/dev/fd/3', got %q", cmd.Args[2])
				}

				hasArgForeground := false
				for _, arg := range cmd.Args[3:] {
					if arg == "--foreground" {
						hasArgForeground = true
						break
					}
				}
				if !hasArgForeground {
					t.Errorf("Expected --foreground argument to be present")
				}

				if len(cmd.ExtraFiles) != 1 {
					t.Errorf("Expected 1 extra file, got %d", len(cmd.ExtraFiles))
				}

				// Write to stderr if this is a failed execution test
				if tt.mockError != nil && cmd.Stderr != nil {
					_, _ = cmd.Stderr.Write([]byte(tt.mockStderr))
				}

				return tt.mockExitCode, tt.mockError
			}

			args := mountpoint.ParseArgs([]string{})
			opts := ForegroundOptions{
				BinaryPath: "/usr/bin/mount-s3",
				BucketName: "test-bucket",
				Fd:         fd,
				Args:       args,
				Env:        []string{"TEST=1"},
				CmdRunner:  mockRunner,
			}

			exitCode, stderr, err := RunInForeground(opts)

			if tt.expectError {
				if err == nil {
					t.Errorf("RunInForeground() expected error but got none")
				}
				if exitCode != tt.expectedExit {
					t.Errorf("RunInForeground() exitCode = %d, expected %d", exitCode, tt.expectedExit)
				}
				if len(stderr) == 0 {
					t.Errorf("RunInForeground() expected stderr output but got none")
				}
			} else {
				if err != nil {
					t.Errorf("RunInForeground() unexpected error: %v", err)
				}
				if exitCode != tt.expectedExit {
					t.Errorf("RunInForeground() exitCode = %d, expected %d", exitCode, tt.expectedExit)
				}
				if stderr != nil {
					t.Errorf("RunInForeground() expected no stderr but got: %s", stderr)
				}
			}
		})
	}
}

func TestRunInForeground_DefaultCmdRunner(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-fuse")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	fd := int(tmpFile.Fd())
	args := mountpoint.ParseArgs([]string{"--debug"})

	opts := ForegroundOptions{
		BinaryPath: "/usr/bin/mount-s3",
		BucketName: "test-bucket",
		Fd:         fd,
		Args:       args,
	}

	_, _, err = RunInForeground(opts)

	if err == nil {
		t.Errorf("RunInForeground() with non-existent binary should fail")
	}
}

func TestRunInForeground_ArgsHandling(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-fuse")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	fd := int(tmpFile.Fd())

	mockRunner := func(cmd *exec.Cmd) (ExitCode, error) {
		hasDebug := false
		hasForeground := false

		for _, arg := range cmd.Args {
			if arg == "--debug" {
				hasDebug = true
			}
			if arg == "--foreground" {
				hasForeground = true
			}
		}

		if !hasDebug {
			t.Errorf("Expected --debug argument to be preserved")
		}
		if !hasForeground {
			t.Errorf("Expected --foreground argument to be added")
		}

		return 0, nil
	}

	args := mountpoint.ParseArgs([]string{"--debug"})
	opts := ForegroundOptions{
		BinaryPath: "/usr/bin/mount-s3",
		BucketName: "test-bucket",
		Fd:         fd,
		Args:       args,
		CmdRunner:  mockRunner,
	}

	_, _, err = RunInForeground(opts)
	if err != nil {
		t.Errorf("RunInForeground() unexpected error: %v", err)
	}
}
