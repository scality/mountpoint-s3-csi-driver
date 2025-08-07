package credentialprovider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/storageclass"
)

func TestNew(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	if provider.client != client {
		t.Error("Expected client to be set correctly")
	}
	if provider.credentialCache == nil {
		t.Error("Expected credential cache to be initialized")
	}
	if provider.cacheTTL != 5*time.Minute {
		t.Error("Expected default cache TTL to be 5 minutes")
	}
}

func TestNewWithCacheTTL(t *testing.T) {
	client := fake.NewSimpleClientset()
	customTTL := 10 * time.Minute
	provider := NewWithCacheTTL(client, customTTL)

	if provider.cacheTTL != customTTL {
		t.Errorf("Expected cache TTL to be %v, got %v", customTTL, provider.cacheTTL)
	}
}

func TestProvideForCreateVolume_DriverCredentials(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	parameters := &storageclass.Parameters{
		AuthTier: storageclass.DriverCredentials,
	}

	config, err := provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error for driver credentials, got: %v", err)
	}

	// Verify we got a valid AWS config (should have default values from AWS SDK)
	if config.Region == "" && config.Credentials == nil {
		t.Error("Expected AWS config to have either region or credentials set")
	}
}

func TestProvideForCreateVolume_ProvisionerSecret(t *testing.T) {
	// Create a secret with valid AWS credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provisioner-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIATEST"),
			constants.SecretAccessKeyField: []byte("test-secret-key"),
			constants.RegionField:          []byte("us-east-1"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	provider := New(client)

	parameters := &storageclass.Parameters{
		ProvisionerSecretName:      "test-provisioner-secret",
		ProvisionerSecretNamespace: "test-namespace",
		AuthTier:                   storageclass.SecretCredentials,
	}

	config, err := provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error for provisioner secret, got: %v", err)
	}

	// Verify the config has the correct region from secret
	if config.Region != "us-east-1" {
		t.Errorf("Expected region us-east-1, got %s", config.Region)
	}
}

func TestProvideForCreateVolume_ProvisionerSecret_Missing(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	parameters := &storageclass.Parameters{
		ProvisionerSecretName:      "missing-secret",
		ProvisionerSecretNamespace: "test-namespace",
		AuthTier:                   storageclass.SecretCredentials,
	}

	_, err := provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err == nil {
		t.Fatal("Expected error for missing secret")
	}

	expectedError := "failed to retrieve secret test-namespace/missing-secret"
	if !containsString(err.Error(), expectedError) {
		t.Errorf("Expected error to contain %q, got: %v", expectedError, err)
	}
}

func TestProvideForDeleteVolume_DriverCredentials(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	// Volume context without secret references (created with driver credentials)
	volumeContext := map[string]string{
		"bucket": "test-bucket",
	}

	config, err := provider.ProvideForDeleteVolume(context.TODO(), volumeContext)
	if err != nil {
		t.Fatalf("Expected no error for driver credentials, got: %v", err)
	}

	// Verify we got a valid AWS config
	if config.Region == "" && config.Credentials == nil {
		t.Error("Expected AWS config to have either region or credentials set")
	}
}

func TestProvideForDeleteVolume_ProvisionerSecret(t *testing.T) {
	// Create a secret with valid AWS credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provisioner-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIATEST"),
			constants.SecretAccessKeyField: []byte("test-secret-key"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	provider := New(client)

	// Volume context with secret references (created with provisioner secret)
	volumeContext := map[string]string{
		constants.VolumeContextProvisionerSecretNameKey:      "test-provisioner-secret",
		constants.VolumeContextProvisionerSecretNamespaceKey: "test-namespace",
	}

	config, err := provider.ProvideForDeleteVolume(context.TODO(), volumeContext)
	if err != nil {
		t.Fatalf("Expected no error for provisioner secret, got: %v", err)
	}

	// Verify we got a valid AWS config
	if config.Credentials == nil {
		t.Error("Expected AWS config to have credentials set")
	}
}

func TestProvideForDeleteVolume_ProvisionerSecret_MissingNamespace(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	// Volume context with only secret name (missing namespace)
	volumeContext := map[string]string{
		constants.VolumeContextProvisionerSecretNameKey: "test-secret",
	}

	_, err := provider.ProvideForDeleteVolume(context.TODO(), volumeContext)
	if err == nil {
		t.Fatal("Expected error for missing secret namespace")
	}

	expectedError := "volume context has provisioner secret name but missing namespace"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got: %v", expectedError, err)
	}
}

func TestProvideForNodePublish_NodePublishSecret(t *testing.T) {
	// Create a secret with valid AWS credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIATEST"),
			constants.SecretAccessKeyField: []byte("test-secret-key"),
			constants.SessionTokenField:    []byte("test-session-token"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	provider := New(client)

	parameters := &storageclass.Parameters{
		NodePublishSecretName:      "test-node-secret",
		NodePublishSecretNamespace: "test-namespace",
		AuthTier:                   storageclass.SecretCredentials,
	}

	config, err := provider.ProvideForNodePublish(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error for node publish secret, got: %v", err)
	}

	// Verify we got a valid AWS config
	if config.Credentials == nil {
		t.Error("Expected AWS config to have credentials set")
	}
}

func TestGetCredentialsFor_CreateVolume(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	parameters := &storageclass.Parameters{
		AuthTier: storageclass.DriverCredentials,
	}

	config, err := provider.GetCredentialsFor(context.TODO(), "CreateVolume", parameters)
	if err != nil {
		t.Fatalf("Expected no error for CreateVolume operation, got: %v", err)
	}

	// Verify we got a valid AWS config
	if config.Region == "" && config.Credentials == nil {
		t.Error("Expected AWS config to have either region or credentials set")
	}
}

func TestGetCredentialsFor_NodePublishVolume(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	parameters := &storageclass.Parameters{
		AuthTier: storageclass.DriverCredentials,
	}

	config, err := provider.GetCredentialsFor(context.TODO(), "NodePublishVolume", parameters)
	if err != nil {
		t.Fatalf("Expected no error for NodePublishVolume operation, got: %v", err)
	}

	// Verify we got a valid AWS config
	if config.Region == "" && config.Credentials == nil {
		t.Error("Expected AWS config to have either region or credentials set")
	}
}

func TestGetCredentialsFor_UnsupportedOperation(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	parameters := &storageclass.Parameters{
		AuthTier: storageclass.DriverCredentials,
	}

	_, err := provider.GetCredentialsFor(context.TODO(), "UnsupportedOperation", parameters)
	if err == nil {
		t.Fatal("Expected error for unsupported operation")
	}

	expectedError := "unsupported operation \"UnsupportedOperation\" for credential resolution"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got: %v", expectedError, err)
	}
}

func TestCredentialCaching(t *testing.T) {
	// Create a secret with valid AWS credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIATEST"),
			constants.SecretAccessKeyField: []byte("test-secret-key"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	provider := NewWithCacheTTL(client, 1*time.Minute)

	parameters := &storageclass.Parameters{
		ProvisionerSecretName:      "test-secret",
		ProvisionerSecretNamespace: "test-namespace",
		AuthTier:                   storageclass.SecretCredentials,
	}

	// First call should hit the API
	_, err := provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify cache has been populated
	total, expired := provider.GetCacheStats()
	if total != 1 {
		t.Errorf("Expected 1 cached entry, got %d", total)
	}
	if expired != 0 {
		t.Errorf("Expected 0 expired entries, got %d", expired)
	}

	// Second call should use cache (no additional API calls)
	callsBefore := len(client.Actions())
	_, err = provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	callsAfter := len(client.Actions())

	if callsAfter != callsBefore {
		t.Error("Expected second call to use cache, but API was called again")
	}
}

func TestCredentialCacheExpiration(t *testing.T) {
	// Create a secret with valid AWS credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIATEST"),
			constants.SecretAccessKeyField: []byte("test-secret-key"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	// Very short TTL for testing expiration
	provider := NewWithCacheTTL(client, 1*time.Millisecond)

	parameters := &storageclass.Parameters{
		ProvisionerSecretName:      "test-secret",
		ProvisionerSecretNamespace: "test-namespace",
		AuthTier:                   storageclass.SecretCredentials,
	}

	// First call
	_, err := provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(10 * time.Millisecond)

	// Second call should hit API again due to expiration
	callsBefore := len(client.Actions())
	_, err = provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	callsAfter := len(client.Actions())

	if callsAfter <= callsBefore {
		t.Error("Expected second call to hit API again due to cache expiration")
	}
}

func TestValidateSecretCredentials_Valid(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIATEST"),
			constants.SecretAccessKeyField: []byte("test-secret-key"),
		},
	}

	client := fake.NewSimpleClientset()
	provider := New(client)

	err := provider.ValidateSecretCredentials(secret)
	if err != nil {
		t.Errorf("Expected no error for valid secret, got: %v", err)
	}
}

func TestValidateSecretCredentials_MissingAccessKey(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			constants.SecretAccessKeyField: []byte("test-secret-key"),
		},
	}

	client := fake.NewSimpleClientset()
	provider := New(client)

	err := provider.ValidateSecretCredentials(secret)
	if err == nil {
		t.Fatal("Expected error for missing access key")
	}

	expectedError := fmt.Sprintf("secret missing required field: %s", constants.AccessKeyIDField)
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got: %v", expectedError, err)
	}
}

func TestValidateSecretCredentials_MissingSecretKey(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			constants.AccessKeyIDField: []byte("AKIATEST"),
		},
	}

	client := fake.NewSimpleClientset()
	provider := New(client)

	err := provider.ValidateSecretCredentials(secret)
	if err == nil {
		t.Fatal("Expected error for missing secret key")
	}

	expectedError := fmt.Sprintf("secret missing required field: %s", constants.SecretAccessKeyField)
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got: %v", expectedError, err)
	}
}

func TestValidateSecretCredentials_NoData(t *testing.T) {
	secret := &corev1.Secret{}

	client := fake.NewSimpleClientset()
	provider := New(client)

	err := provider.ValidateSecretCredentials(secret)
	if err == nil {
		t.Fatal("Expected error for secret with no data")
	}

	expectedError := "secret has no data"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got: %v", expectedError, err)
	}
}

func TestClearCache(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	// Populate cache manually for testing
	provider.credentialCache["test-key"] = &CredentialCacheEntry{
		Config:    aws.Config{},
		ExpiresAt: time.Now().Add(1 * time.Minute),
	}

	// Verify cache has entry
	total, _ := provider.GetCacheStats()
	if total != 1 {
		t.Errorf("Expected 1 cached entry before clear, got %d", total)
	}

	// Clear cache
	provider.ClearCache()

	// Verify cache is empty
	total, _ = provider.GetCacheStats()
	if total != 0 {
		t.Errorf("Expected 0 cached entries after clear, got %d", total)
	}
}

func TestSetCacheTTL(t *testing.T) {
	client := fake.NewSimpleClientset()
	provider := New(client)

	newTTL := 10 * time.Minute
	provider.SetCacheTTL(newTTL)

	if provider.cacheTTL != newTTL {
		t.Errorf("Expected cache TTL to be %v, got %v", newTTL, provider.cacheTTL)
	}
}

func TestMixedCredentialScenarios(t *testing.T) {
	// Create secrets for different tiers
	provisionerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "provisioner-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIAPROVISIONER"),
			constants.SecretAccessKeyField: []byte("provisioner-secret-key"),
		},
	}

	nodeSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIANODE"),
			constants.SecretAccessKeyField: []byte("node-secret-key"),
		},
	}

	client := fake.NewSimpleClientset(provisionerSecret, nodeSecret)
	provider := New(client)

	// Test mixed credential scenario
	parameters := &storageclass.Parameters{
		ProvisionerSecretName:      "provisioner-secret",
		ProvisionerSecretNamespace: "test-namespace",
		NodePublishSecretName:      "node-secret",
		NodePublishSecretNamespace: "test-namespace",
		AuthTier:                   storageclass.SecretCredentials,
	}

	// Controller operations should use provisioner secret
	controllerConfig, err := provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error for controller credentials, got: %v", err)
	}

	// Node operations should use node-publish secret
	nodeConfig, err := provider.ProvideForNodePublish(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error for node credentials, got: %v", err)
	}

	// Verify different credentials are used (both should have credentials set)
	if controllerConfig.Credentials == nil {
		t.Error("Expected controller config to have credentials set")
	}
	if nodeConfig.Credentials == nil {
		t.Error("Expected node config to have credentials set")
	}

	// Verify cache has separate entries for different secrets
	total, _ := provider.GetCacheStats()
	if total != 2 {
		t.Errorf("Expected 2 cached entries (one for each secret), got %d", total)
	}
}

func TestAPIRateLimiting(t *testing.T) {
	// Create a secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			constants.AccessKeyIDField:     []byte("AKIATEST"),
			constants.SecretAccessKeyField: []byte("test-secret-key"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	provider := New(client)

	parameters := &storageclass.Parameters{
		ProvisionerSecretName:      "test-secret",
		ProvisionerSecretNamespace: "test-namespace",
		AuthTier:                   storageclass.SecretCredentials,
	}

	// Test caching by making sequential calls
	_, err := provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error from first call, got: %v", err)
	}

	// Count initial API calls
	firstCallCount := 0
	for _, action := range client.Actions() {
		if action.GetVerb() == "get" {
			firstCallCount++
		}
	}

	// Second call should use cache (no additional API calls)
	_, err = provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err != nil {
		t.Fatalf("Expected no error from second call, got: %v", err)
	}

	// Count API calls after second call
	secondCallCount := 0
	for _, action := range client.Actions() {
		if action.GetVerb() == "get" {
			secondCallCount++
		}
	}

	// Should have made one API call for the first request, but not for the second
	if firstCallCount != 1 {
		t.Errorf("Expected 1 API call after first request, got %d", firstCallCount)
	}

	if secondCallCount != firstCallCount {
		t.Errorf("Expected cached second call to not make additional API calls, but got %d calls vs %d", secondCallCount, firstCallCount)
	}

	// Verify cache statistics
	total, expired := provider.GetCacheStats()
	if total != 1 {
		t.Errorf("Expected 1 cached entry, got %d", total)
	}
	if expired != 0 {
		t.Errorf("Expected 0 expired entries, got %d", expired)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && containsString(s[1:], substr)) ||
		(len(s) >= len(substr) && s[:len(substr)] == substr))
}

// Mock API failures for integration testing
func TestAPIFailureHandling(t *testing.T) {
	client := fake.NewSimpleClientset()

	// Add a reactor to simulate API failures
	client.PrependReactor("get", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, kerrors.NewInternalError(fmt.Errorf("internal server error"))
	})

	provider := New(client)

	parameters := &storageclass.Parameters{
		ProvisionerSecretName:      "test-secret",
		ProvisionerSecretNamespace: "test-namespace",
		AuthTier:                   storageclass.SecretCredentials,
	}

	_, err := provider.ProvideForCreateVolume(context.TODO(), parameters)
	if err == nil {
		t.Fatal("Expected error when API fails")
	}

	expectedError := "failed to retrieve secret test-namespace/test-secret"
	if !containsString(err.Error(), expectedError) {
		t.Errorf("Expected error to contain %q, got: %v", expectedError, err)
	}
}
