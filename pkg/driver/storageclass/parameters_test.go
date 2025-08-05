package storageclass

import (
	"testing"
)

func TestParseAndValidate(t *testing.T) {
	tests := []struct {
		name       string
		parameters map[string]string
		expected   *Parameters
		shouldErr  bool
	}{
		{
			name:       "nil parameters - driver credentials",
			parameters: nil,
			expected: &Parameters{
				AuthTier: DriverCredentials,
			},
			shouldErr: false,
		},
		{
			name:       "empty parameters - driver credentials",
			parameters: map[string]string{},
			expected: &Parameters{
				AuthTier: DriverCredentials,
			},
			shouldErr: false,
		},
		{
			name: "static secret parameters - secret credentials",
			parameters: map[string]string{
				ProvisionerSecretNameKey:      "tenant-a-creds",
				ProvisionerSecretNamespaceKey: "tenant-a",
			},
			expected: &Parameters{
				ProvisionerSecretName:      "tenant-a-creds",
				ProvisionerSecretNamespace: "tenant-a",
				AuthTier:                   SecretCredentials,
			},
			shouldErr: false,
		},
		{
			name: "secret parameters - secret credentials",
			parameters: map[string]string{
				ProvisionerSecretNameKey:      "web-app-storage-creds",
				ProvisionerSecretNamespaceKey: "production",
			},
			expected: &Parameters{
				ProvisionerSecretName:      "web-app-storage-creds",
				ProvisionerSecretNamespace: "production",
				AuthTier:                   SecretCredentials,
			},
			shouldErr: false,
		},
		{
			name: "mixed secret parameters - secret credentials",
			parameters: map[string]string{
				ProvisionerSecretNameKey:      "app-specific-creds",
				ProvisionerSecretNamespaceKey: "default",
			},
			expected: &Parameters{
				ProvisionerSecretName:      "app-specific-creds",
				ProvisionerSecretNamespace: "default",
				AuthTier:                   SecretCredentials,
			},
			shouldErr: false,
		},
		{
			name: "secret name without namespace - should error",
			parameters: map[string]string{
				ProvisionerSecretNameKey: "tenant-a-creds",
			},
			expected:  nil,
			shouldErr: true,
		},
		{
			name: "secret namespace without name - should error",
			parameters: map[string]string{
				ProvisionerSecretNamespaceKey: "tenant-a",
			},
			expected:  nil,
			shouldErr: true,
		},
		{
			name: "unsupported parameters are stripped",
			parameters: map[string]string{
				ProvisionerSecretNameKey:      "test-creds",
				ProvisionerSecretNamespaceKey: "default",
				NodePublishSecretNameKey:      "test-creds",
				NodePublishSecretNamespaceKey: "default",
				"customParam":                 "ignored-value",
				"anotherParam":                "also-ignored",
			},
			expected: &Parameters{
				ProvisionerSecretName:      "test-creds",
				ProvisionerSecretNamespace: "default",
				NodePublishSecretName:      "test-creds",
				NodePublishSecretNamespace: "default",
				AuthTier:                   SecretCredentials,
			},
			shouldErr: false,
		},
		{
			name: "whitespace trimming",
			parameters: map[string]string{
				ProvisionerSecretNameKey:      "  test-creds  ",
				ProvisionerSecretNamespaceKey: "  default  ",
			},
			expected: &Parameters{
				ProvisionerSecretName:      "test-creds",
				ProvisionerSecretNamespace: "default",
				AuthTier:                   SecretCredentials,
			},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAndValidate(tt.parameters)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("Expected result but got nil")
				return
			}

			if result.ProvisionerSecretName != tt.expected.ProvisionerSecretName {
				t.Errorf("Expected ProvisionerSecretName %q, got %q", tt.expected.ProvisionerSecretName, result.ProvisionerSecretName)
			}

			if result.ProvisionerSecretNamespace != tt.expected.ProvisionerSecretNamespace {
				t.Errorf("Expected ProvisionerSecretNamespace %q, got %q", tt.expected.ProvisionerSecretNamespace, result.ProvisionerSecretNamespace)
			}

			if result.AuthTier != tt.expected.AuthTier {
				t.Errorf("Expected AuthTier %v, got %v", tt.expected.AuthTier, result.AuthTier)
			}
		})
	}
}

func TestDetermineAuthenticationTier(t *testing.T) {
	tests := []struct {
		name                       string
		provisionerSecretName      string
		provisionerSecretNamespace string
		nodePublishSecretName      string
		nodePublishSecretNamespace string
		expected                   AuthenticationTier
	}{
		{
			name:                       "no secret parameters - driver credentials",
			provisionerSecretName:      "",
			provisionerSecretNamespace: "",
			nodePublishSecretName:      "",
			nodePublishSecretNamespace: "",
			expected:                   DriverCredentials,
		},
		{
			name:                       "provisioner secret only - secret credentials",
			provisionerSecretName:      "tenant-creds",
			provisionerSecretNamespace: "tenant-ns",
			nodePublishSecretName:      "",
			nodePublishSecretNamespace: "",
			expected:                   SecretCredentials,
		},
		{
			name:                       "node publish secret only - secret credentials",
			provisionerSecretName:      "",
			provisionerSecretNamespace: "",
			nodePublishSecretName:      "node-creds",
			nodePublishSecretNamespace: "default",
			expected:                   SecretCredentials,
		},
		{
			name:                       "both secrets - secret credentials",
			provisionerSecretName:      "controller-creds",
			provisionerSecretNamespace: "system",
			nodePublishSecretName:      "node-creds",
			nodePublishSecretNamespace: "default",
			expected:                   SecretCredentials,
		},
		{
			name:                       "secret values - secret credentials",
			provisionerSecretName:      "web-app-storage-creds",
			provisionerSecretNamespace: "production",
			nodePublishSecretName:      "web-app-storage-creds",
			nodePublishSecretNamespace: "production",
			expected:                   SecretCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineAuthenticationTier(tt.provisionerSecretName, tt.provisionerSecretNamespace, tt.nodePublishSecretName, tt.nodePublishSecretNamespace)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestValidateSecretParameterConsistency(t *testing.T) {
	tests := []struct {
		name            string
		secretName      string
		secretNamespace string
		shouldErr       bool
		expectedError   string
	}{
		{
			name:            "both empty - valid",
			secretName:      "",
			secretNamespace: "",
			shouldErr:       false,
		},
		{
			name:            "both provided - valid",
			secretName:      "test-creds",
			secretNamespace: "default",
			shouldErr:       false,
		},
		{
			name:            "name without namespace - invalid",
			secretName:      "test-creds",
			secretNamespace: "",
			shouldErr:       true,
			expectedError:   "provisioner secret name provided but namespace is missing",
		},
		{
			name:            "namespace without name - invalid",
			secretName:      "",
			secretNamespace: "default",
			shouldErr:       true,
			expectedError:   "provisioner secret namespace provided but name is missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecretParameterConsistency(tt.secretName, tt.secretNamespace, "provisioner")

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAuthenticationTierString(t *testing.T) {
	tests := []struct {
		tier     AuthenticationTier
		expected string
	}{
		{DriverCredentials, "driver-credentials"},
		{SecretCredentials, "secret-credentials"},
		{AuthenticationTier(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.tier.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
