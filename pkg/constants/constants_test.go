package constants

import (
	"testing"
)

func TestCredentialConstants(t *testing.T) {
	// Test AWS credential field names match expected format
	expectedFields := map[string]string{
		AccessKeyIDField:     "access_key_id",
		SecretAccessKeyField: "secret_access_key",
		SessionTokenField:    "session_token",
		RegionField:          "region",
	}

	for constant, expected := range expectedFields {
		if constant != expected {
			t.Errorf("Expected credential field constant %q, got %q", expected, constant)
		}
	}
}

func TestCSISecretParameterConstants(t *testing.T) {
	// Test CSI secret parameter names match CSI specification format
	expectedParams := map[string]string{
		ProvisionerSecretNameKey:      "csi.storage.k8s.io/provisioner-secret-name",
		ProvisionerSecretNamespaceKey: "csi.storage.k8s.io/provisioner-secret-namespace",
		NodePublishSecretNameKey:      "csi.storage.k8s.io/node-publish-secret-name",
		NodePublishSecretNamespaceKey: "csi.storage.k8s.io/node-publish-secret-namespace",
	}

	for constant, expected := range expectedParams {
		if constant != expected {
			t.Errorf("Expected CSI parameter constant %q, got %q", expected, constant)
		}
	}
}

func TestVolumeContextConstants(t *testing.T) {
	// Test volume context key names match expected format
	expectedKeys := map[string]string{
		VolumeContextProvisionerSecretNameKey:      "provisioner-secret-name",
		VolumeContextProvisionerSecretNamespaceKey: "provisioner-secret-namespace",
	}

	for constant, expected := range expectedKeys {
		if constant != expected {
			t.Errorf("Expected volume context constant %q, got %q", expected, constant)
		}
	}
}

func TestConstantUniqueness(t *testing.T) {
	// Ensure all constants are unique
	constants := []string{
		AccessKeyIDField,
		SecretAccessKeyField,
		SessionTokenField,
		RegionField,
		ProvisionerSecretNameKey,
		ProvisionerSecretNamespaceKey,
		NodePublishSecretNameKey,
		NodePublishSecretNamespaceKey,
		VolumeContextProvisionerSecretNameKey,
		VolumeContextProvisionerSecretNamespaceKey,
	}

	seen := make(map[string]bool)
	for _, constant := range constants {
		if seen[constant] {
			t.Errorf("Duplicate constant value found: %q", constant)
		}
		seen[constant] = true
	}
}
