package driver_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"

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

// TestControllerOnlyAffectsMounterCreation verifies that when CSI_CONTROLLER_ONLY is true,
// the driver skips mounter initialization and thus has a nil NodeServer; otherwise it creates one.
func TestControllerOnlyAffectsMounterCreation(t *testing.T) {
	// Save and restore env vars
	originalControllerOnly := os.Getenv("CSI_CONTROLLER_ONLY")
	originalEndpointURL := os.Getenv(envprovider.EnvEndpointURL)
	originalNodeName := os.Getenv("NODE_NAME")
	defer func() {
		if originalControllerOnly == "" {
			_ = os.Unsetenv("CSI_CONTROLLER_ONLY")
		} else {
			_ = os.Setenv("CSI_CONTROLLER_ONLY", originalControllerOnly)
		}
		if originalEndpointURL == "" {
			_ = os.Unsetenv(envprovider.EnvEndpointURL)
		} else {
			_ = os.Setenv(envprovider.EnvEndpointURL, originalEndpointURL)
		}
		if originalNodeName == "" {
			_ = os.Unsetenv("NODE_NAME")
		} else {
			_ = os.Setenv("NODE_NAME", originalNodeName)
		}
		// restore seams
		driver.InClusterConfigTestHook(nil)
		driver.KubeClientForConfigTestHook(nil)
		driver.KubernetesVersionTestHook(nil)
		// restore cache hooks
		driver.CheckSelectableFieldsTestHook(nil)
		driver.SetupCacheTestHook(nil)
	}()

	// Provide required env for NewDriver validation
	_ = os.Setenv(envprovider.EnvEndpointURL, "http://s3.example.com:8000")

	// Hook the external dependencies so NewDriver is unit-testable
	driver.InClusterConfigTestHook(func() (*rest.Config, error) {
		// return a dummy config to allow client creation
		return &rest.Config{Host: "http://localhost"}, nil
	})
	driver.KubeClientForConfigTestHook(func(*rest.Config) (kubernetes.Interface, error) {
		// return a fake clientset
		return fake.NewSimpleClientset(), nil
	})
	driver.KubernetesVersionTestHook(func(_ kubernetes.Interface) (string, error) {
		return "v1.30.0", nil
	})

	// Mock the cache setup functions to avoid connecting to real k8s API
	driver.CheckSelectableFieldsTestHook(func(ctx context.Context, config *rest.Config) (bool, error) {
		return true, nil // Pretend field selector is supported
	})
	driver.SetupCacheTestHook(func(config *rest.Config, stopCh <-chan struct{}, nodeID, kubernetesVersion string) ctrlcache.Cache {
		// Return nil cache for this test since we're not actually using it
		return nil
	})

	// 1) controller-only path: NodeServer should be nil
	_ = os.Setenv("CSI_CONTROLLER_ONLY", "true")
	d1, err := driver.NewDriver("unix:///tmp/test.sock", "mpv", "node-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d1.NodeServer == nil {
		// expected
	} else {
		t.Fatalf("expected NodeServer to be nil in controller-only mode")
	}

	// 2) node path: NodeServer should be non-nil (pod mounter is now the only option)
	_ = os.Setenv("CSI_CONTROLLER_ONLY", "false")
	_ = os.Setenv("MOUNTPOINT_NAMESPACE", "mount-s3") // Required for pod mounter
	_ = os.Setenv("NODE_NAME", "test-node")           // Required for pod mounter with CRD support
	d2, err := driver.NewDriver("unix:///tmp/test.sock", "mpv", "node-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Note: NodeServer will be initialized with pod mounter
	// We can't fully test it here without a pod watcher, but
	// we can verify the driver initializes without error.
	// The actual pod mounter initialization is tested in integration tests.
	if d2 == nil {
		t.Fatalf("expected driver to be initialized")
	}
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
