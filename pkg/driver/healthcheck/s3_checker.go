// Package healthcheck provides S3 endpoint health checking functionality
package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// ConnectivityStatus represents the current status of an S3 endpoint connectivity check
type ConnectivityStatus struct {
	Healthy      bool          `json:"healthy"`
	LastChecked  time.Time     `json:"lastChecked"`
	LastError    string        `json:"lastError,omitempty"`
	ResponseTime time.Duration `json:"responseTime"`
}

// S3ConnectivityChecker periodically checks S3 endpoint connectivity
type S3ConnectivityChecker struct {
	endpointURL string
	client      *http.Client
	status      ConnectivityStatus
	mu          sync.RWMutex
	interval    time.Duration
	stopCh      chan struct{}
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

// updateStatus updates the connectivity status
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

	// Log status changes
	if healthy {
		klog.V(4).Infof("S3 endpoint health check: healthy (response time: %v)", responseTime)
	} else {
		klog.Warningf("S3 endpoint health check: unhealthy - %s", errMsg)
	}
}

// CheckConnectivity performs a single connectivity check
// This method is designed to work without authentication credentials
func (c *S3ConnectivityChecker) CheckConnectivity(ctx context.Context) error {
	startTime := time.Now()

	// Parse the endpoint URL to validate it
	_, err := url.Parse(c.endpointURL)
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

// Start begins periodic connectivity checks
func (c *S3ConnectivityChecker) Start(ctx context.Context) {
	klog.Infof("Starting S3 endpoint health checks every %v for %s", c.interval, c.endpointURL)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Perform initial check
	if err := c.CheckConnectivity(ctx); err != nil {
		klog.Warningf("Initial S3 endpoint health check failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			// Perform periodic check
			if err := c.CheckConnectivity(ctx); err != nil {
				klog.V(4).Infof("Periodic S3 endpoint health check failed: %v", err)
			}
		case <-c.stopCh:
			klog.Infof("Stopping S3 endpoint health checks for %s", c.endpointURL)
			return
		case <-ctx.Done():
			klog.Infof("Context canceled, stopping S3 endpoint health checks for %s", c.endpointURL)
			return
		}
	}
}

// Stop halts periodic connectivity checks
func (c *S3ConnectivityChecker) Stop() {
	select {
	case <-c.stopCh:
		// Already closed
		return
	default:
		close(c.stopCh)
	}
}
