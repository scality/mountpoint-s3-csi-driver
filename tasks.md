# S3CSI-32: Make S3 Endpoint URL Mandatory

## Goals
- Make the AWS S3 endpoint URL (AWS_ENDPOINT_URL) a mandatory parameter
- Enforce validation at both Helm deployment time and driver runtime
- Prevent service startup if the endpoint URL is not configured
- Add appropriate tests to verify this behavior

## Functional Requirements
1. Helm chart must fail installation if S3 endpoint URL is not provided
2. Driver must fail to start if AWS_ENDPOINT_URL environment variable is not set
3. Error messages must be clear and actionable
4. Implementation must include sufficient test coverage

## Task Dashboard

| Phase | Task | Description | Status | Depends On |
|-------|------|-------------|--------|------------|
| 1     | 1    | Helm Chart Changes | ✅ Done |            |
| 1     | 1.1  | Update values.yaml comment to indicate S3 endpoint URL is required | ✅ Done |            |
| 1     | 1.2  | Modify node.yaml to use required function | ✅ Done |            |
| 2     | 2    | Go Code Changes | ✅ Done |            |
| 2     | 2.1  | Add validation in driver.go | ✅ Done | 1.2        |
| 3     | 3    | Testing | ✅ Done |            |
| 3     | 3.1  | Add unit tests for Go validation | ✅ Done | 2.1        |
| 3     | 3.2  | Add GitHub workflow for validation | ✅ Done | 2.1        |
| 4     | 4    | Documentation | ✅ Done |            |
| 4     | 4.1  | Update README to note required S3 endpoint URL | ✅ Done | 1.1, 2.1  |
| 5     | 5    | S3 Endpoint Health Check | ⬜ To Do |            |
| 5     | 5.1  | Implement S3ConnectivityChecker component | ⬜ To Do | 2.1        |
| 5     | 5.2  | Add non-authenticated S3 endpoint check | ⬜ To Do | 5.1        |
| 5     | 5.3  | Add connectivity check during driver startup | ⬜ To Do | 5.2        |
| 5     | 5.4  | Implement periodic health checks | ⬜ To Do | 5.3        |
| 5     | 5.5  | Add liveness probe integration | ⬜ To Do | 5.4        |
| 5     | 5.6  | Add unit and integration tests for health checker | ⬜ To Do | 5.5        |
| 5     | 5.7  | Update documentation for health checks | ⬜ To Do | 5.6        |


---
## Plan Context (Jira: S3CSI-32)

# Complete Plan: Make S3 Endpoint URL Mandatory

## 1. Implementation Changes

### A. Helm Chart Changes
- Update `charts/scality-mountpoint-s3-csi-driver/values.yaml`:
  ```yaml
  # AWS S3 endpoint URL to use for all volume mounts (REQUIRED)
  s3EndpointUrl: ""
  ```

- Modify `charts/scality-mountpoint-s3-csi-driver/templates/node.yaml`:
  ```yaml
  - name: AWS_ENDPOINT_URL
    value: {{ required "S3 endpoint URL (.Values.node.s3EndpointUrl) must be provided for the CSI driver to function" .Values.node.s3EndpointUrl }}
  ```

### B. Go Code Validation
- Add validation in `pkg/driver/driver.go` during driver initialization:
  ```go
  func NewDriver(endpoint string, mpVersion string, nodeID string) (*Driver, error) {
      // Existing initialization code...

      // Validate that AWS_ENDPOINT_URL is set
      if os.Getenv(envprovider.EnvEndpointURL) == "" {
          return nil, fmt.Errorf("AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function")
      }

      // Rest of the existing initialization code...
  }
  ```

## 2. Testing Strategy

### A. Unit Tests
- Add test in `pkg/driver/driver_test.go`:
  ```go
  func TestNewDriverValidatesEndpointURL(t *testing.T) {
      // Clear environment variables
      os.Unsetenv(envprovider.EnvEndpointURL)
      
      // Attempt to create driver without endpoint URL
      _, err := NewDriver("unix:///tmp/test.sock", "1.0.0", "test-node")
      
      // Verify it fails with the expected error
      if err == nil {
          t.Fatal("Expected driver creation to fail without AWS_ENDPOINT_URL")
      }
      if !strings.Contains(err.Error(), "AWS_ENDPOINT_URL environment variable must be set") {
          t.Fatalf("Unexpected error message: %v", err)
      }
      
      // Set the environment variable
      os.Setenv(envprovider.EnvEndpointURL, "https://test-endpoint.example.com")
      
      // Now driver creation should succeed
      drv, err := NewDriver("unix:///tmp/test.sock", "1.0.0", "test-node")
      if err != nil {
          t.Fatalf("Driver creation failed with endpoint URL set: %v", err)
      }
      if drv == nil {
          t.Fatal("Driver is nil despite successful creation")
      }
  }
  ```

### B. GitHub Workflow Test
- Create `.github/workflows/endpoint-url-validation.yml`:
  ```yaml
  name: S3 Endpoint URL Validation

  on:
    push:
      branches: [ main ]
    pull_request:
      branches: [ main ]

  jobs:
    validate-endpoint-url:
      runs-on: ubuntu-latest
      steps:
        - name: Check out repository
          uses: actions/checkout@v4

        - name: Setup environment
          uses: ./.github/actions/e2e-setup-common
          with:
            ref: ${{ github.ref }}

        # Test 1: Verify Helm chart fails without endpoint URL
        - name: Test Helm Chart Validation
          run: |
            echo "Testing Helm chart validation for S3 endpoint URL..."
            if helm install test-release ./charts/scality-mountpoint-s3-csi-driver --dry-run 2>&1 | grep -q "S3 endpoint URL .* must be provided"; then
              echo "✅ Helm validation working: Install failed without endpoint URL"
            else
              echo "❌ Test failed: Helm install did not fail when S3 endpoint URL was missing"
              exit 1
            fi

        # Test 2: Compile and test the driver without endpoint URL  
        - name: Build Driver
          run: make build

        - name: Test Driver Fails Without Endpoint URL
          run: |
            echo "Testing driver startup validation..."
            unset AWS_ENDPOINT_URL
            export CSI_NODE_NAME=test-node
            if ./bin/scality-csi-driver --endpoint unix:///tmp/test.sock 2>&1 | grep -q "AWS_ENDPOINT_URL environment variable must be set"; then
              echo "✅ Driver validation working: Startup failed without endpoint URL"
            else
              echo "❌ Test failed: Driver started when AWS_ENDPOINT_URL was missing"
              exit 1
            fi
```

## 5. S3 Endpoint Health Check Implementation Plan

### Overview
The S3 Endpoint Health Check will verify that the provided AWS_ENDPOINT_URL points to a valid S3-compatible endpoint, without requiring credentials. This allows detection of misconfigured endpoints and network issues early.

### 5.1. Implement S3ConnectivityChecker Component
Create a new package `pkg/driver/healthcheck` with an S3 connectivity checker component:

```go
// pkg/driver/healthcheck/s3_checker.go
package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// S3ConnectivityChecker periodically checks S3 endpoint connectivity
type S3ConnectivityChecker struct {
	endpointURL string
	client      *http.Client
	status      ConnectivityStatus
	mu          sync.RWMutex
	interval    time.Duration
	stopCh      chan struct{}
}

type ConnectivityStatus struct {
	Healthy      bool
	LastChecked  time.Time
	LastError    string
	ResponseTime time.Duration
}

// NewS3ConnectivityChecker creates a new S3 connectivity checker
func NewS3ConnectivityChecker(endpointURL string, checkInterval time.Duration) *S3ConnectivityChecker {
	return &S3ConnectivityChecker{
		endpointURL: endpointURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		interval: checkInterval,
		stopCh:   make(chan struct{}),
	}
}

// CurrentStatus returns the current connectivity status
func (c *S3ConnectivityChecker) CurrentStatus() ConnectivityStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// UpdateStatus updates the connectivity status
func (c *S3ConnectivityChecker) updateStatus(healthy bool, err error, responseTime time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	
	c.status = ConnectivityStatus{
		Healthy:      healthy,
		LastChecked:  time.Now(),
		LastError:    errMsg,
		ResponseTime: responseTime,
	}
}
```

### 5.2. Add Non-Authenticated S3 Endpoint Check
Implement the core functionality that checks for S3 compatibility without requiring authentication:

```go
// pkg/driver/healthcheck/s3_checker.go (continued)

// CheckConnectivity performs a single connectivity check
func (c *S3ConnectivityChecker) CheckConnectivity(ctx context.Context) error {
	startTime := time.Now()
	
	// Parse the endpoint URL
	parsedURL, err := url.Parse(c.endpointURL)
	if err != nil {
		c.updateStatus(false, fmt.Errorf("invalid endpoint URL: %w", err), 0)
		return err
	}
	
	// Construct a HEAD request to the endpoint
	// This will fail with 403 Forbidden (which is expected without credentials)
	// but will succeed in connecting to a valid S3 endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.endpointURL, nil)
	if err != nil {
		c.updateStatus(false, fmt.Errorf("failed to create request: %w", err), 0)
		return err
	}
	
	resp, err := c.client.Do(req)
	responseTime := time.Since(startTime)
	
	// Network or connection errors
	if err != nil {
		c.updateStatus(false, fmt.Errorf("connection failed: %w", err), responseTime)
		return err
	}
	defer resp.Body.Close()
	
	// Check if the response has S3-specific headers or status codes
	// Even a 403 Forbidden response is acceptable as it indicates
	// we reached a real S3 endpoint that requires auth
	isS3Compatible := c.isS3CompatibleResponse(resp)
	
	if !isS3Compatible {
		err = fmt.Errorf("endpoint doesn't appear to be S3-compatible: status %d", resp.StatusCode)
		c.updateStatus(false, err, responseTime)
		return err
	}
	
	// Success - we got a response that looks like S3
	c.updateStatus(true, nil, responseTime)
	return nil
}

// isS3CompatibleResponse checks if the HTTP response appears to be from an S3-compatible service
func (c *S3ConnectivityChecker) isS3CompatibleResponse(resp *http.Response) bool {
	// For S3 endpoints, these are acceptable status codes when not authenticated:
	// - 403 Forbidden: We hit a valid endpoint but don't have credentials
	// - 400 Bad Request: Some S3 implementations when request is incomplete
	// - 405 Method Not Allowed: Some S3 implementations for HEAD requests
	validStatusCodes := map[int]bool{
		403: true,
		400: true,
		405: true,
	}
	
	// Check for S3-specific headers (different S3 implementations may have different headers)
	hasS3Headers := resp.Header.Get("x-amz-request-id") != "" || // AWS S3
		resp.Header.Get("x-amz-id-2") != "" || // AWS S3
		resp.Header.Get("Server") == "AmazonS3" || // AWS S3
		resp.Header.Get("x-scal-request-id") != "" || // Scality S3
		resp.Header.Get("Server") == "Zenko" // Zenko/Scality

	return validStatusCodes[resp.StatusCode] || hasS3Headers
}
```

### 5.3. Add Connectivity Check During Driver Startup
Integrate the connectivity checker during driver initialization:

```go
// pkg/driver/driver.go modifications

import (
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/healthcheck"
	// ...existing imports
)

type Driver struct {
	// ...existing fields
	S3HealthChecker *healthcheck.S3ConnectivityChecker
}

func NewDriver(endpoint string, mpVersion string, nodeID string) (*Driver, error) {
	// Validate that AWS_ENDPOINT_URL is set
	endpointURL := os.Getenv(envprovider.EnvEndpointURL)
	if endpointURL == "" {
		return nil, fmt.Errorf("AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function")
	}
	
	// Check S3 endpoint connectivity
	healthChecker := healthcheck.NewS3ConnectivityChecker(endpointURL, 30*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := healthChecker.CheckConnectivity(ctx); err != nil {
		klog.Warningf("S3 endpoint connectivity check failed: %v. Driver will continue but verify your S3 endpoint configuration.", err)
		// Note: We only warn but don't fail to provide flexibility in air-gapped environments
	} else {
		klog.Infof("Successfully verified S3 endpoint connectivity to %s", endpointURL)
	}
	
	// ...existing driver initialization
	
	driver := &Driver{
		// ...existing fields
		S3HealthChecker: healthChecker,
	}
	
	return driver, nil
}
```

### 5.4. Implement Periodic Health Checks
Add periodic health check functionality to continuously monitor endpoint connectivity:

```go
// pkg/driver/healthcheck/s3_checker.go (continued)

// Start begins periodic connectivity checks
func (c *S3ConnectivityChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	
	// Perform initial check
	_ = c.CheckConnectivity(ctx)
	
	for {
		select {
		case <-ticker.C:
			_ = c.CheckConnectivity(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop halts periodic connectivity checks
func (c *S3ConnectivityChecker) Stop() {
	close(c.stopCh)
}
```

Integrate it with driver startup:

```go
// pkg/driver/driver.go modifications

func (d *Driver) Run() error {
	// ...existing setup code
	
	// Start periodic health checks in background
	go d.S3HealthChecker.Start(context.Background())
	
	// ...existing server start code
}

func (d *Driver) Stop() {
	klog.Infof("Stopping server")
	
	// Stop health checker
	if d.S3HealthChecker != nil {
		d.S3HealthChecker.Stop()
	}
	
	// ...existing stop code
}
```

### 5.5. Add Liveness Probe Integration
Extend the HTTP health server to expose S3 connectivity status:

```go
// pkg/driver/node/server.go modifications

import (
	"net/http"
	"encoding/json"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/healthcheck"
	// ...existing imports
)

type S3NodeServer struct {
	// ...existing fields
	healthChecker *healthcheck.S3ConnectivityChecker
}

func NewS3NodeServer(nodeID string, mounter mounter.Mounter, healthChecker *healthcheck.S3ConnectivityChecker) *S3NodeServer {
	// ...existing code
	server := &S3NodeServer{
		// ...existing fields
		healthChecker: healthChecker,
	}
	
	// Add S3 health endpoint handler
	http.HandleFunc("/s3-health", server.handleS3HealthCheck)
	
	return server
}

func (ns *S3NodeServer) handleS3HealthCheck(w http.ResponseWriter, r *http.Request) {
	if ns.healthChecker == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unavailable",
			"error":  "health checker not initialized",
		})
		return
	}
	
	status := ns.healthChecker.CurrentStatus()
	w.Header().Set("Content-Type", "application/json")
	
	if !status.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	json.NewEncoder(w).Encode(status)
}
```

### 5.6. Add Unit and Integration Tests
Create comprehensive tests for the S3 endpoint health checker:

```go
// pkg/driver/healthcheck/s3_checker_test.go
package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestS3ConnectivityChecker_CheckConnectivity(t *testing.T) {
	// Test with mocked S3 server that returns 403 Forbidden with S3 headers
	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-amz-request-id", "test-request-id")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer s3Server.Close()
	
	checker := NewS3ConnectivityChecker(s3Server.URL, 1*time.Second)
	err := checker.CheckConnectivity(context.Background())
	
	if err != nil {
		t.Fatalf("Expected successful check with mock S3 server, got error: %v", err)
	}
	
	status := checker.CurrentStatus()
	if !status.Healthy {
		t.Fatalf("Expected healthy status, got: %+v", status)
	}
	
	// Test with non-S3 server
	nonS3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer nonS3Server.Close()
	
	checker = NewS3ConnectivityChecker(nonS3Server.URL, 1*time.Second)
	err = checker.CheckConnectivity(context.Background())
	
	if err == nil {
		t.Fatal("Expected error with non-S3 server, got success")
	}
	
	status = checker.CurrentStatus()
	if status.Healthy {
		t.Fatalf("Expected unhealthy status with non-S3 server, got: %+v", status)
	}
	
	// Test with unreachable server
	checker = NewS3ConnectivityChecker("http://invalid.example.com:12345", 1*time.Second)
	err = checker.CheckConnectivity(context.Background())
	
	if err == nil {
		t.Fatal("Expected error with unreachable server, got success")
	}
	
	status = checker.CurrentStatus()
	if status.Healthy {
		t.Fatalf("Expected unhealthy status with unreachable server, got: %+v", status)
	}
}
```

### 5.7. Update Documentation
Update documentation to include information about the S3 health checks:

In README.md and other relevant documentation, add:

```markdown
## S3 Endpoint Health Checks

The Scality S3 CSI Driver includes S3 endpoint health monitoring capabilities:

1. **Startup Validation**: When the driver starts, it performs a basic connectivity check to verify the S3 endpoint is reachable.

2. **Periodic Health Checks**: The driver continuously monitors S3 endpoint health in the background.

3. **Health Endpoint**: A dedicated `/s3-health` HTTP endpoint is available for monitoring systems to check S3 connectivity status.

### Monitoring S3 Health

You can check the current S3 endpoint health status using:

```bash
curl http://<node-ip>:9808/s3-health
```

Example response:
```json
{
  "Healthy": true,
  "LastChecked": "2023-05-09T22:15:30Z",
  "LastError": "",
  "ResponseTime": 235000000
}
```

The health check is designed to verify S3 compatibility without requiring authentication credentials, making it suitable for infrastructure monitoring.