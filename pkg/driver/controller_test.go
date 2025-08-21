package driver

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	controllerCredProvider "github.com/scality/mountpoint-s3-csi-driver/pkg/driver/controller/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/s3client"
)

// Mock S3 client for testing
type mockS3Client struct {
	createBucketFunc func(ctx context.Context, bucket string) error
	deleteBucketFunc func(ctx context.Context, bucket string) error
}

func (m *mockS3Client) CreateBucket(ctx context.Context, bucket string) error {
	if m.createBucketFunc != nil {
		return m.createBucketFunc(ctx, bucket)
	}
	return nil
}

func (m *mockS3Client) DeleteBucket(ctx context.Context, bucket string) error {
	if m.deleteBucketFunc != nil {
		return m.deleteBucketFunc(ctx, bucket)
	}
	return nil
}

func TestCreateVolume(t *testing.T) {
	tests := []struct {
		name          string
		req           *csi.CreateVolumeRequest
		setupSecrets  []*corev1.Secret
		expectedError codes.Code
		errorContains string
	}{
		{
			name:          "nil request",
			req:           nil,
			expectedError: codes.InvalidArgument,
			errorContains: "request is nil",
		},
		{
			name: "missing volume name",
			req: &csi.CreateVolumeRequest{
				Name: "",
			},
			expectedError: codes.InvalidArgument,
			errorContains: "volume name is required",
		},
		{
			name: "missing volume capabilities",
			req: &csi.CreateVolumeRequest{
				Name:               "test-volume",
				VolumeCapabilities: nil, // or could be empty slice: []*csi.VolumeCapability{}
			},
			expectedError: codes.InvalidArgument,
			errorContains: "volume capabilities are required",
		},
		{
			name: "valid request with driver credentials",
			req: &csi.CreateVolumeRequest{
				Name:       "test-volume",
				Parameters: map[string]string{
					// No secret parameters = driver credentials
				},
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1024 * 1024 * 1024, // 1Gi
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
						},
					},
				},
			},
			expectedError: codes.OK,
		},
		{
			name: "valid request with CSI provisioner secrets",
			req: &csi.CreateVolumeRequest{
				Name:       "test-volume-with-secret",
				Parameters: map[string]string{
					// Parameters are empty in CSI spec - provisioner resolves secrets
				},
				Secrets: map[string]string{
					// CSI provisioner provides credential values directly
					constants.AccessKeyIDField:     "AKIATEST",
					constants.SecretAccessKeyField: "test-secret-key",
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
						},
					},
				},
			},
			expectedError: codes.OK,
		},
		{
			name: "invalid StorageClass parameters",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				Parameters: map[string]string{
					"csi.storage.k8s.io/provisioner-secret-name": "test-secret",
					// Missing namespace - should error
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
						},
					},
				},
			},
			expectedError: codes.InvalidArgument,
			errorContains: "provisioner secret name provided but namespace is missing",
		},
		{
			name: "invalid single-node access mode",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume-single-node",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			expectedError: codes.InvalidArgument,
			errorContains: "S3 volumes only support multi-node access modes",
		},
		{
			name: "missing volume capability access mode",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume-nil-access-mode",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: nil, // This should trigger the validation error
					},
				},
			},
			expectedError: codes.InvalidArgument,
			errorContains: "volume capability access mode is required",
		},
		{
			name: "with CSI node-publish secrets",
			req: &csi.CreateVolumeRequest{
				Name:       "test-volume-node-secret",
				Parameters: map[string]string{
					// Parameters are empty in CSI spec - provisioner resolves secrets
				},
				Secrets: map[string]string{
					// CSI provisioner provides credential values directly
					constants.AccessKeyIDField:     "AKIANODE",
					constants.SecretAccessKeyField: "node-secret-key",
				},
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
						},
					},
				},
			},
			expectedError: codes.OK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment variables for S3 client
			_ = os.Setenv("AWS_ENDPOINT_URL", "http://s3.example.com")
			_ = os.Setenv("AWS_REGION", "us-east-1")
			defer func() {
				_ = os.Unsetenv("AWS_ENDPOINT_URL")
				_ = os.Unsetenv("AWS_REGION")
			}()

			// Create a fake Kubernetes client with any required secrets
			fakeClient := fake.NewSimpleClientset()
			for _, secret := range tc.setupSecrets {
				_, err := fakeClient.CoreV1().Secrets(secret.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test secret: %v", err)
				}
			}

			// Create mock S3 client
			mockS3 := &mockS3Client{}

			// Create driver with controller credential provider and mock S3 client factory
			driver := &Driver{
				controllerCredProvider: controllerCredProvider.New(fakeClient),
				testS3ClientFactory: func(ctx context.Context, awsConfig *aws.Config) (s3client.Client, error) {
					return mockS3, nil
				},
			}

			// Call CreateVolume
			resp, err := driver.CreateVolume(context.Background(), tc.req)

			// Check error
			if tc.expectedError != codes.OK {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Error is not a gRPC status error: %v", err)
				}
				if st.Code() != tc.expectedError {
					t.Fatalf("Expected error code %v, got %v", tc.expectedError, st.Code())
				}
				if tc.errorContains != "" && !strings.Contains(st.Message(), tc.errorContains) {
					t.Fatalf("Expected error to contain %q, got %q", tc.errorContains, st.Message())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				// Validate response
				if resp == nil {
					t.Fatal("Response is nil")
				}
				if resp.Volume == nil {
					t.Fatal("Volume is nil")
				}
				if resp.Volume.VolumeId == "" {
					t.Fatal("Volume ID is empty")
				}
				if !strings.HasPrefix(resp.Volume.VolumeId, "csi-s3-") {
					t.Fatalf("Volume ID %q doesn't have expected prefix", resp.Volume.VolumeId)
				}

				// Check volume context
				if resp.Volume.VolumeContext == nil {
					t.Fatal("Volume context is nil")
				}
				if resp.Volume.VolumeContext["dynamicProvisioning"] != "true" {
					t.Fatal("Volume context missing dynamicProvisioning flag")
				}
				if resp.Volume.VolumeContext["bucketName"] != resp.Volume.VolumeId {
					t.Fatalf("Bucket name %q doesn't match volume ID %q",
						resp.Volume.VolumeContext["bucketName"], resp.Volume.VolumeId)
				}

				// Check authentication source is set correctly based on CSI secrets
				expectedAuthSource := "driver"
				if len(tc.req.Secrets) > 0 {
					expectedAuthSource = "secret"
				}
				if resp.Volume.VolumeContext["authenticationSource"] != expectedAuthSource {
					t.Fatalf("Expected authenticationSource %q, got %q",
						expectedAuthSource, resp.Volume.VolumeContext["authenticationSource"])
				}
			}
		})
	}
}

func TestGenerateVolumeID(t *testing.T) {
	// Test multiple generations to ensure uniqueness and UUID-based format
	generated := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id := generateVolumeID()

		if !strings.HasPrefix(id, "csi-s3-") {
			t.Fatalf("Volume ID %q doesn't have expected prefix", id)
		}

		if generated[id] {
			t.Fatalf("Duplicate volume ID generated: %s", id)
		}
		generated[id] = true

		// Expect UUID v4 format suffix (contains hyphens) after prefix
		suffix := strings.TrimPrefix(id, "csi-s3-")
		if len(suffix) == 0 || !strings.Contains(suffix, "-") {
			t.Fatalf("Volume ID %q does not appear to be UUID-based", id)
		}
	}
}

func TestCreateVolumeAuthenticationSource(t *testing.T) {
	tests := []struct {
		name               string
		csiSecrets         map[string]string
		expectedAuthSource string
	}{
		{
			name:               "no CSI secrets - use driver credentials",
			csiSecrets:         nil,
			expectedAuthSource: "driver",
		},
		{
			name: "CSI secrets provided - use secret credentials",
			csiSecrets: map[string]string{
				constants.AccessKeyIDField:     "AKIATEST",
				constants.SecretAccessKeyField: "test-secret-key",
			},
			expectedAuthSource: "secret",
		},
		{
			name: "CSI secrets with session token - use secret credentials",
			csiSecrets: map[string]string{
				constants.AccessKeyIDField:     "AKIATEST",
				constants.SecretAccessKeyField: "test-secret-key",
				constants.SessionTokenField:    "session-token-123",
			},
			expectedAuthSource: "secret",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment variables for S3 client
			_ = os.Setenv("AWS_ENDPOINT_URL", "http://s3.example.com")
			_ = os.Setenv("AWS_REGION", "us-east-1")
			defer func() {
				_ = os.Unsetenv("AWS_ENDPOINT_URL")
				_ = os.Unsetenv("AWS_REGION")
			}()

			// Create a fake Kubernetes client (no secrets needed for CSI approach)
			fakeClient := fake.NewSimpleClientset()

			// Create mock S3 client
			mockS3 := &mockS3Client{}

			// Create driver with controller credential provider and mock S3 client factory
			driver := &Driver{
				controllerCredProvider: controllerCredProvider.New(fakeClient),
				testS3ClientFactory: func(ctx context.Context, awsConfig *aws.Config) (s3client.Client, error) {
					return mockS3, nil
				},
			}

			// Create request with CSI secrets
			req := &csi.CreateVolumeRequest{
				Name:       "test-volume",
				Parameters: map[string]string{}, // Empty - CSI provisioner doesn't pass secret names
				Secrets:    tc.csiSecrets,       // CSI provisioner provides credential values
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
						},
					},
				},
			}

			// Call CreateVolume
			resp, err := driver.CreateVolume(context.Background(), req)
			// Check error
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify AuthenticationSource is set correctly
			authSource := resp.Volume.VolumeContext["authenticationSource"]
			if authSource != tc.expectedAuthSource {
				t.Fatalf("Expected authenticationSource %q, got %q", tc.expectedAuthSource, authSource)
			}
		})
	}
}

func TestDeleteVolume(t *testing.T) {
	tests := []struct {
		name          string
		req           *csi.DeleteVolumeRequest
		expectedError codes.Code
		errorContains string
	}{
		{
			name:          "nil request",
			req:           nil,
			expectedError: codes.InvalidArgument,
			errorContains: "request is nil",
		},
		{
			name: "missing volume ID",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "",
			},
			expectedError: codes.InvalidArgument,
			errorContains: "volume ID is required",
		},
		{
			name: "valid delete request",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "csi-s3-1640995200-a1b2c3d",
			},
			expectedError: codes.OK,
		},
		{
			name: "idempotent delete - non-existent volume",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "csi-s3-nonexistent-volume",
			},
			expectedError: codes.OK,
		},
		{
			name: "volume created by CreateVolume",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "csi-s3-1640995200-xyz123",
			},
			expectedError: codes.OK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up environment variables for S3 client
			_ = os.Setenv("AWS_ENDPOINT_URL", "http://s3.example.com")
			_ = os.Setenv("AWS_REGION", "us-east-1")
			defer func() {
				_ = os.Unsetenv("AWS_ENDPOINT_URL")
				_ = os.Unsetenv("AWS_REGION")
			}()

			// Create a fake Kubernetes client
			fakeClient := fake.NewSimpleClientset()

			// Create mock S3 client
			mockS3 := &mockS3Client{}

			// Create driver with controller credential provider and mock S3 client factory
			driver := &Driver{
				controllerCredProvider: controllerCredProvider.New(fakeClient),
				testS3ClientFactory: func(ctx context.Context, awsConfig *aws.Config) (s3client.Client, error) {
					return mockS3, nil
				},
			}

			// Call DeleteVolume
			resp, err := driver.DeleteVolume(context.Background(), tc.req)

			// Check error
			if tc.expectedError != codes.OK {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Error is not a gRPC status error: %v", err)
				}
				if st.Code() != tc.expectedError {
					t.Fatalf("Expected error code %v, got %v", tc.expectedError, st.Code())
				}
				if tc.errorContains != "" && !strings.Contains(st.Message(), tc.errorContains) {
					t.Fatalf("Expected error to contain %q, got %q", tc.errorContains, st.Message())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				// Validate response
				if resp == nil {
					t.Fatal("Response is nil")
				}
				// DeleteVolumeResponse should be empty for successful deletion
			}
		})
	}
}

func TestValidateDeleteVolumeRequest(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.DeleteVolumeRequest
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil request",
			req:         nil,
			expectError: true,
			errorMsg:    "request is nil",
		},
		{
			name: "empty volume ID",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "",
			},
			expectError: true,
			errorMsg:    "volume ID is required",
		},
		{
			name: "valid request",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "csi-s3-1640995200-a1b2c3d",
			},
			expectError: false,
		},
		{
			name: "valid request with secrets (future-proofing)",
			req: &csi.DeleteVolumeRequest{
				VolumeId: "csi-s3-1640995200-xyz123",
				Secrets: map[string]string{
					"accessKeyID":     "AKIATEST",
					"secretAccessKey": "test-secret-key",
				},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDeleteVolumeRequest(tc.req)
			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
					t.Fatalf("Expected error to contain %q, got %q", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestGenerateVolumeIDFormat tests that generated volume IDs follow the expected UUID format
func TestGenerateVolumeIDFormat(t *testing.T) {
	// Generate multiple IDs to ensure they follow UUID format consistently
	for i := 0; i < 5; i++ {
		id := generateVolumeID()

		if !strings.HasPrefix(id, "csi-s3-") {
			t.Fatalf("Volume ID %q doesn't have expected prefix", id)
		}

		// Extract UUID portion after prefix
		uuidPart := strings.TrimPrefix(id, "csi-s3-")
		if len(uuidPart) == 0 {
			t.Fatalf("Volume ID %q is missing UUID portion", id)
		}

		// UUID should contain hyphens and be in standard format
		if !strings.Contains(uuidPart, "-") {
			t.Fatalf("Volume ID %q UUID portion %q does not appear to be valid UUID format", id, uuidPart)
		}

		// Verify UUID-like length (UUIDs are typically 36 characters with hyphens)
		if len(uuidPart) != 36 {
			t.Fatalf("Volume ID %q UUID portion %q has unexpected length %d, expected 36", id, uuidPart, len(uuidPart))
		}
	}
}

func TestCreateS3Client(t *testing.T) {
	tests := []struct {
		name            string
		setupEnv        func()
		mockFactory     func(ctx context.Context, awsConfig *aws.Config) (s3client.Client, error)
		inputConfig     *aws.Config
		expectedSuccess bool
		errorContains   string
	}{
		{
			name: "successful S3 client creation",
			setupEnv: func() {
				_ = os.Setenv("AWS_ENDPOINT_URL", "https://s3.example.com")
				_ = os.Setenv("AWS_REGION", "us-west-2")
			},
			mockFactory: nil, // Use real S3 client creation
			inputConfig: &aws.Config{
				Region:      "us-west-2",
				Credentials: credentials.NewStaticCredentialsProvider("AKIATEST", "test-secret-key", ""),
			},
			expectedSuccess: true,
		},
		{
			name: "test factory override",
			setupEnv: func() {
				_ = os.Setenv("AWS_ENDPOINT_URL", "https://s3.example.com")
				_ = os.Setenv("AWS_REGION", "us-east-1")
			},
			mockFactory: func(ctx context.Context, awsConfig *aws.Config) (s3client.Client, error) {
				return &mockS3Client{}, nil
			},
			inputConfig: &aws.Config{
				Region:      "us-east-1",
				Credentials: credentials.NewStaticCredentialsProvider("AKIATEST", "test-secret-key", ""),
			},
			expectedSuccess: true,
		},
		{
			name: "test factory error",
			setupEnv: func() {
				_ = os.Setenv("AWS_ENDPOINT_URL", "https://s3.example.com")
				_ = os.Setenv("AWS_REGION", "eu-west-1")
			},
			mockFactory: func(ctx context.Context, awsConfig *aws.Config) (s3client.Client, error) {
				return nil, fmt.Errorf("mock factory error")
			},
			inputConfig: &aws.Config{
				Region:      "eu-west-1",
				Credentials: credentials.NewStaticCredentialsProvider("AKIATEST", "test-secret-key", ""),
			},
			expectedSuccess: false,
			errorContains:   "mock factory error",
		},
		{
			name: "missing endpoint URL environment variable",
			setupEnv: func() {
				// Don't set AWS_ENDPOINT_URL
				_ = os.Setenv("AWS_REGION", "ap-southeast-1")
			},
			mockFactory: nil,
			inputConfig: &aws.Config{
				Region:      "ap-southeast-1",
				Credentials: credentials.NewStaticCredentialsProvider("AKIATEST", "test-secret-key", ""),
			},
			expectedSuccess: true, // Should still work without endpoint URL
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clean environment
			_ = os.Unsetenv("AWS_ENDPOINT_URL")
			_ = os.Unsetenv("AWS_REGION")

			// Setup test environment
			tc.setupEnv()

			defer func() {
				_ = os.Unsetenv("AWS_ENDPOINT_URL")
				_ = os.Unsetenv("AWS_REGION")
			}()

			// Create driver with test factory if provided
			driver := &Driver{}
			if tc.mockFactory != nil {
				driver.testS3ClientFactory = tc.mockFactory
			}

			// Call createS3Client
			client, err := driver.createS3Client(context.Background(), tc.inputConfig)

			// Check results
			if tc.expectedSuccess {
				if err != nil {
					t.Fatalf("Expected success but got error: %v", err)
				}
				if client == nil {
					t.Fatal("Expected client but got nil")
				}
			} else {
				if err == nil {
					t.Fatalf("Expected error but got success")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Fatalf("Expected error to contain %q, got %q", tc.errorContains, err.Error())
				}
			}
		})
	}
}
