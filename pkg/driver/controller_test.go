package driver

import (
	"context"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	controllerCredProvider "github.com/scality/mountpoint-s3-csi-driver/pkg/driver/controller/credentialprovider"
)

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
			name: "valid request with driver credentials",
			req: &csi.CreateVolumeRequest{
				Name:       "test-volume",
				Parameters: map[string]string{
					// No secret parameters = driver credentials
				},
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1024 * 1024 * 1024, // 1Gi
				},
			},
			expectedError: codes.OK,
		},
		{
			name: "valid request with provisioner secret",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume-with-secret",
				Parameters: map[string]string{
					"csi.storage.k8s.io/provisioner-secret-name":      "test-secret",
					"csi.storage.k8s.io/provisioner-secret-namespace": "default",
				},
			},
			setupSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						constants.AccessKeyIDField:     []byte("AKIATEST"),
						constants.SecretAccessKeyField: []byte("test-secret-key"),
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
			},
			expectedError: codes.InvalidArgument,
			errorContains: "provisioner secret name provided but namespace is missing",
		},
		{
			name: "with volume capabilities",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
				},
			},
			expectedError: codes.OK,
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
			name: "with node publish secret",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume-node-secret",
				Parameters: map[string]string{
					"csi.storage.k8s.io/node-publish-secret-name":      "node-secret",
					"csi.storage.k8s.io/node-publish-secret-namespace": "kube-system",
				},
			},
			setupSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "node-secret",
						Namespace: "kube-system",
					},
					Data: map[string][]byte{
						constants.AccessKeyIDField:     []byte("AKIANODE"),
						constants.SecretAccessKeyField: []byte("node-secret-key"),
					},
				},
			},
			expectedError: codes.OK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake Kubernetes client with any required secrets
			fakeClient := fake.NewSimpleClientset()
			for _, secret := range tc.setupSecrets {
				_, err := fakeClient.CoreV1().Secrets(secret.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test secret: %v", err)
				}
			}

			// Create driver with controller credential provider
			driver := &Driver{
				controllerCredProvider: controllerCredProvider.New(fakeClient),
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

				// Check credentials were stored if secret parameters were provided
				if tc.req.Parameters != nil {
					if secretName := tc.req.Parameters["csi.storage.k8s.io/provisioner-secret-name"]; secretName != "" {
						if resp.Volume.VolumeContext["provisioner-secret-name"] != secretName {
							t.Fatalf("Expected provisioner-secret-name %q in volume context, got %q",
								secretName, resp.Volume.VolumeContext["provisioner-secret-name"])
						}
					}
					if secretName := tc.req.Parameters["csi.storage.k8s.io/node-publish-secret-name"]; secretName != "" {
						if resp.Volume.VolumeContext["node-publish-secret-name"] != secretName {
							t.Fatalf("Expected node-publish-secret-name %q in volume context, got %q",
								secretName, resp.Volume.VolumeContext["node-publish-secret-name"])
						}
					}
				}
			}
		})
	}
}

func TestValidateCreateVolumeRequest(t *testing.T) {
	tests := []struct {
		name        string
		req         *csi.CreateVolumeRequest
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
			name: "empty volume name",
			req: &csi.CreateVolumeRequest{
				Name: "",
			},
			expectError: true,
			errorMsg:    "volume name is required",
		},
		{
			name: "valid request",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
			},
			expectError: false,
		},
		{
			name: "with valid capabilities",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing access mode",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: nil,
					},
				},
			},
			expectError: true,
			errorMsg:    "volume capability access mode is required",
		},
		{
			name: "single node writer (not supported)",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "S3 volumes only support multi-node access modes",
		},
		{
			name: "single node reader only (not supported)",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				VolumeCapabilities: []*csi.VolumeCapability{
					{
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "S3 volumes only support multi-node access modes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCreateVolumeRequest(tc.req)
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
