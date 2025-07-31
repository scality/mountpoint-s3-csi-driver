package system

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

// TestIsConnectionError tests the core logic for detecting D-Bus connection failures
// This is pure business logic with no external dependencies
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

// TestSystemdRunner_FactoryCallCount tests that factory is called correctly
// This tests the factory interaction pattern without calling SystemD methods
func TestSystemdRunner_FactoryCallCount(t *testing.T) {
	callCount := 0
	factory := &testFactory{
		createFunc: func() (*SystemdSupervisor, error) {
			callCount++
			// Create a supervisor with mock connection that won't panic
			return &SystemdSupervisor{
				conn: &mockConnection{},
			}, nil
		},
	}

	runner, err := StartSystemdRunner(factory)
	if err != nil {
		t.Fatalf("Failed to create initial runner: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 initial factory call, got %d", callCount)
	}

	// Test recreation by directly calling the recreation method
	err = runner.recreateSupervisor()
	if err != nil {
		t.Errorf("Recreation should succeed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 total factory calls (create + recreate), got %d", callCount)
	}
}

// TestSystemdRunner_RecreationFailure tests error handling when recreation fails
func TestSystemdRunner_RecreationFailure(t *testing.T) {
	callCount := 0
	factory := &testFactory{
		createFunc: func() (*SystemdSupervisor, error) {
			callCount++
			if callCount == 1 {
				return &SystemdSupervisor{
					conn: &mockConnection{},
				}, nil
			}
			return nil, errors.New("recreation failed")
		},
	}

	runner, err := StartSystemdRunner(factory)
	if err != nil {
		t.Fatalf("Failed to create initial runner: %v", err)
	}

	// Test recreation failure
	err = runner.recreateSupervisor()
	if err == nil {
		t.Errorf("Expected recreation to fail")
	}

	expectedErrMsg := "failed to recreate SystemD supervisor: recreation failed"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message %q, got: %v", expectedErrMsg, err)
	}
}

// TestOsSystemdSupervisorFactory_Integration tests the production factory
// This verifies graceful failure on non-SystemD systems
func TestOsSystemdSupervisorFactory_Integration(t *testing.T) {
	factory := OsSystemdSupervisorFactory{}

	// Attempt to create with real OS factory
	_, err := StartSystemdRunner(factory)

	if err == nil {
		t.Log("SystemdRunner created successfully - SystemD is available")
	} else {
		// Verify error is related to SystemD unavailability, not code issues
		errStr := err.Error()
		expectedPatterns := []string{
			"failed to connect to systemd",
			"no such file or directory",
			"connect: connection refused",
		}

		hasExpectedError := false
		for _, pattern := range expectedPatterns {
			if strings.Contains(errStr, pattern) {
				hasExpectedError = true
				break
			}
		}

		if !hasExpectedError {
			t.Errorf("Unexpected error type - may indicate code issue: %v", err)
		} else {
			t.Logf("Expected SystemD unavailability error: %v", err)
		}
	}
}

// testFactory is a simple test factory for testing SystemdRunner logic
type testFactory struct {
	createFunc func() (*SystemdSupervisor, error)
}

func (f *testFactory) Create() (*SystemdSupervisor, error) {
	if f.createFunc != nil {
		return f.createFunc()
	}
	return &SystemdSupervisor{}, nil
}

// mockConnection implements SystemdConnection for testing
type mockConnection struct{}

func (m *mockConnection) ListUnits(ctx context.Context) ([]*Unit, error) {
	return nil, errors.New("mock connection")
}

func (m *mockConnection) StopUnit(ctx context.Context, unitName string) error {
	return nil
}

func (m *mockConnection) StartTransientUnit(ctx context.Context, name string, mode string, props []DbusProperty) (dbus.ObjectPath, error) {
	return "", errors.New("mock connection")
}

func (m *mockConnection) Signal(ch chan<- *dbus.Signal) {}

func (m *mockConnection) Close() error {
	return nil
}
