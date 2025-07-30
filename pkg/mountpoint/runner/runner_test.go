package runner

import (
	"os/exec"
	"testing"
)

func TestDefaultCmdRunner(t *testing.T) {
	tests := []struct {
		name           string
		cmd            *exec.Cmd
		expectedExit   int
		expectError    bool
		errorSubstring string
	}{
		{
			name:         "successful command",
			cmd:          exec.Command("true"),
			expectedExit: 0,
			expectError:  false,
		},
		{
			name:         "failed command",
			cmd:          exec.Command("false"),
			expectedExit: 0,
			expectError:  true,
		},
		{
			name:           "non-existent command",
			cmd:            exec.Command("non-existent-command-xyz"),
			expectedExit:   0,
			expectError:    true,
			errorSubstring: "executable file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode, err := DefaultCmdRunner(tt.cmd)

			if tt.expectError {
				if err == nil {
					t.Errorf("DefaultCmdRunner() expected error but got none")
				}
				if tt.errorSubstring != "" && err != nil {
					if !containsString(err.Error(), tt.errorSubstring) {
						t.Errorf("DefaultCmdRunner() error = %v, expected to contain %q", err, tt.errorSubstring)
					}
				}
			} else {
				if err != nil {
					t.Errorf("DefaultCmdRunner() unexpected error: %v", err)
				}
				if exitCode != tt.expectedExit {
					t.Errorf("DefaultCmdRunner() exitCode = %d, expected %d", exitCode, tt.expectedExit)
				}
			}
		})
	}
}

func TestCmdRunnerInterface(t *testing.T) {
	var mockRunner CmdRunner = func(cmd *exec.Cmd) (ExitCode, error) {
		return 42, nil
	}

	cmd := exec.Command("echo", "test")
	exitCode, err := mockRunner(cmd)
	if err != nil {
		t.Errorf("Mock CmdRunner returned unexpected error: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("Mock CmdRunner returned exitCode = %d, expected 42", exitCode)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) <= len(s) && func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()))
}
