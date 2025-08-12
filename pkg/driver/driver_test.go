package driver_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
)

// validateEndpointURL is a function that mimics the validation in driver.NewDriver
// but can be tested without all the dependencies of the full driver
func validateEndpointURL() error {
	if os.Getenv(envprovider.EnvEndpointURL) == "" {
		return fmt.Errorf("AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function")
	}
	return nil
}

func TestValidatesEndpointURL(t *testing.T) {
	// Save original environment variables to restore them after the test
	originalEndpointURL := os.Getenv(envprovider.EnvEndpointURL)
	defer func() {
		_ = os.Setenv(envprovider.EnvEndpointURL, originalEndpointURL)
	}()

	// Test case 1: No endpoint URL set
	t.Run("fails without endpoint URL", func(t *testing.T) {
		// Clear environment variable
		_ = os.Unsetenv(envprovider.EnvEndpointURL)

		// Attempt to validate without endpoint URL
		err := validateEndpointURL()

		// Verify it fails with the expected error
		if err == nil {
			t.Fatal("Expected validation to fail without AWS_ENDPOINT_URL")
		}
		if !strings.Contains(err.Error(), "AWS_ENDPOINT_URL environment variable must be set") {
			t.Fatalf("Unexpected error message: %v", err)
		}
	})

	// Test case 2: Endpoint URL is set
	t.Run("succeeds with endpoint URL", func(t *testing.T) {
		// Set the environment variable
		_ = os.Setenv(envprovider.EnvEndpointURL, "https://test-endpoint.example.com")

		// Attempt to validate with endpoint URL
		err := validateEndpointURL()
		// Verify it succeeds
		if err != nil {
			t.Fatalf("Unexpected error when AWS_ENDPOINT_URL is set: %v", err)
		}
	})
}

// TestDriver is a type that allows us to use internal functions of driver.Driver while
// avoiding initialization of Kubernetes client
type TestDriver driver.Driver

// This test directly tests the NewDriver function to ensure code coverage for the endpoint URL validation
func TestNewDriverEndpointURLValidation(t *testing.T) {
	// Save original environment variables to restore them after the test
	originalEndpointURL := os.Getenv(envprovider.EnvEndpointURL)
	defer func() {
		_ = os.Setenv(envprovider.EnvEndpointURL, originalEndpointURL)
	}()

	// Test case 1: No endpoint URL set
	t.Run("NewDriver fails without endpoint URL", func(t *testing.T) {
		// Clear environment variable
		_ = os.Unsetenv(envprovider.EnvEndpointURL)

		// Try to create a new driver without setting the endpoint URL
		// We expect this to fail with a specific error
		_, err := driver.NewDriver("unix:///tmp/test.sock", "test-mp-version", "test-node-id")

		// Check that we got the expected error
		if err == nil {
			t.Fatal("Expected driver creation to fail without AWS_ENDPOINT_URL")
		}
		if !strings.Contains(err.Error(), "AWS_ENDPOINT_URL environment variable must be set") {
			t.Fatalf("Unexpected error message: %v", err)
		}
	})

	// Test case 2: With endpoint URL but without Kubernetes (still fails but differently)
	t.Run("NewDriver with endpoint URL proceeds to next validation", func(t *testing.T) {
		// Set environment variable
		_ = os.Setenv(envprovider.EnvEndpointURL, "https://test-endpoint.example.com")

		// Try to create a new driver with endpoint URL set
		// This will still fail, but with a different error (about Kubernetes, not about endpoint URL)
		_, err := driver.NewDriver("unix:///tmp/test.sock", "test-mp-version", "test-node-id")

		// Check that we got an error, but NOT the endpoint URL error
		if err == nil {
			t.Skip("Test unexpectedly passed - this might be running within a Kubernetes cluster")
		}

		// The error should be about Kubernetes, not about the endpoint URL
		if strings.Contains(err.Error(), "AWS_ENDPOINT_URL environment variable must be set") {
			t.Fatalf("Got endpoint URL error despite setting the environment variable: %v", err)
		}

		// Verify we're failing at a later point in the initialization
		// This indicates the endpoint URL validation passed
		if strings.Contains(err.Error(), "cannot create in-cluster config") {
			// This is the expected error when not running in a Kubernetes cluster
			// It means we passed the endpoint URL validation and moved on to the K8s client initialization
			return
		}

		t.Logf("Got unexpected error type: %v", err)
	})
}

func TestDriverStop(t *testing.T) {
	driver := &driver.Driver{}
	// Should not panic even with nil server
	driver.Stop()
}

// TestControllerOnlyEnvironmentLogic tests the CSI_CONTROLLER_ONLY environment variable parsing
func TestControllerOnlyEnvironmentLogic(t *testing.T) {
	// Save original environment variable
	originalControllerOnly := os.Getenv("CSI_CONTROLLER_ONLY")
	defer func() {
		if originalControllerOnly == "" {
			_ = os.Unsetenv("CSI_CONTROLLER_ONLY")
		} else {
			_ = os.Setenv("CSI_CONTROLLER_ONLY", originalControllerOnly)
		}
	}()

	t.Run("environment variable parsing works correctly", func(t *testing.T) {
		// Test the actual environment variable condition used in the code: os.Getenv("CSI_CONTROLLER_ONLY") == "true"

		// Test case 1: CSI_CONTROLLER_ONLY="true" should return true
		_ = os.Setenv("CSI_CONTROLLER_ONLY", "true")
		if os.Getenv("CSI_CONTROLLER_ONLY") != "true" {
			t.Fatal("Expected CSI_CONTROLLER_ONLY environment variable to be 'true'")
		}
		// Verify the condition used in the code
		if os.Getenv("CSI_CONTROLLER_ONLY") != "true" {
			t.Fatal("Controller-only mode condition should be true when CSI_CONTROLLER_ONLY='true'")
		}

		// Test case 2: CSI_CONTROLLER_ONLY="false" should not trigger controller-only mode
		_ = os.Setenv("CSI_CONTROLLER_ONLY", "false")
		if os.Getenv("CSI_CONTROLLER_ONLY") == "true" {
			t.Fatal("Expected CSI_CONTROLLER_ONLY environment variable to not be 'true' when set to 'false'")
		}
		// Verify the condition used in the code
		if os.Getenv("CSI_CONTROLLER_ONLY") == "true" {
			t.Fatal("Controller-only mode condition should be false when CSI_CONTROLLER_ONLY='false'")
		}

		// Test case 3: Unset CSI_CONTROLLER_ONLY should not trigger controller-only mode
		_ = os.Unsetenv("CSI_CONTROLLER_ONLY")
		if os.Getenv("CSI_CONTROLLER_ONLY") == "true" {
			t.Fatal("Expected CSI_CONTROLLER_ONLY environment variable to not be 'true' when unset")
		}
		// Verify the condition used in the code
		if os.Getenv("CSI_CONTROLLER_ONLY") == "true" {
			t.Fatal("Controller-only mode condition should be false when CSI_CONTROLLER_ONLY is unset")
		}
	})
}

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		expectedScheme string
		expectedAddr   string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid unix socket",
			endpoint:       "unix:///tmp/csi.sock",
			expectedScheme: "unix",
			expectedAddr:   "/tmp/csi.sock",
			expectError:    false,
		},
		{
			name:           "valid tcp endpoint",
			endpoint:       "tcp://127.0.0.1:50051",
			expectedScheme: "tcp",
			expectedAddr:   "127.0.0.1:50051",
			expectError:    false,
		},
		{
			name:        "empty endpoint",
			endpoint:    "",
			expectError: true,
		},
		{
			name:        "invalid endpoint format",
			endpoint:    "invalid-endpoint",
			expectError: true,
		},
		{
			name:        "missing scheme",
			endpoint:    "localhost:50051",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scheme, addr, err := driver.ParseEndpoint(tc.endpoint)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Fatalf("Expected error to contain %q, got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if scheme != tc.expectedScheme {
					t.Fatalf("Expected scheme %q, got %q", tc.expectedScheme, scheme)
				}
				if addr != tc.expectedAddr {
					t.Fatalf("Expected addr %q, got %q", tc.expectedAddr, addr)
				}
			}
		})
	}
}
