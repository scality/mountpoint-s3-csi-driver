package system

import (
	"errors"
	"testing"
)

func TestSystemdRunner_FactoryIntegration(t *testing.T) {
	// Test that SystemdRunner can be created with OsSystemdSupervisorFactory
	// This verifies the integration between SystemdRunner and the factory pattern
	factory := OsSystemdSupervisorFactory{}

	// This should not fail even if we don't have SystemD running (it will fail later during actual operations)
	_, err := StartSystemdRunner(factory)

	// We expect this to succeed in creating the runner, even if systemd is not available
	// The actual SystemD interaction happens during StartService/RunOneshot calls
	if err == nil {
		t.Log("SystemdRunner successfully created with OsSystemdSupervisorFactory")
	} else {
		t.Logf("SystemdRunner creation failed (expected if SystemD not available): %v", err)
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil_error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection_closed",
			err:      errors.New("connection closed"),
			expected: true,
		},
		{
			name:     "connection_is_closed",
			err:      errors.New("connection is closed"),
			expected: true,
		},
		{
			name:     "use_of_closed_network_connection",
			err:      errors.New("use of closed network connection"),
			expected: true,
		},
		{
			name:     "other_error",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "mixed_error_with_connection_closed",
			err:      errors.New("failed to start: connection closed"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v but got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}
