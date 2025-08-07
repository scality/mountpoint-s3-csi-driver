// Package credentialprovider provides credential resolution for controller operations with granular credential handling.
// This package adapts the existing credential provider patterns for CSI controller use cases,
// supporting driver-level credentials, provisioner secrets, and node-publish secrets with independent fallback logic.
package credentialprovider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/storageclass"
)

// CredentialCacheEntry represents a cached credential with expiration
type CredentialCacheEntry struct {
	Config    aws.Config
	ExpiresAt time.Time
}

// Provider provides credential resolution for controller operations with granular credential handling
type Provider struct {
	client kubernetes.Interface

	// Credential cache for performance optimization and Kubernetes API rate limiting
	credentialCache map[string]*CredentialCacheEntry
	cacheMutex      sync.RWMutex

	// Cache TTL for credential validation results
	cacheTTL time.Duration
}

// New creates a new controller credential provider
func New(client kubernetes.Interface) *Provider {
	return &Provider{
		client:          client,
		credentialCache: make(map[string]*CredentialCacheEntry),
		cacheTTL:        5 * time.Minute, // Default cache TTL
	}
}

// NewWithCacheTTL creates a new controller credential provider with custom cache TTL
func NewWithCacheTTL(client kubernetes.Interface, cacheTTL time.Duration) *Provider {
	return &Provider{
		client:          client,
		credentialCache: make(map[string]*CredentialCacheEntry),
		cacheTTL:        cacheTTL,
	}
}

// ProvideForCreateVolume resolves controller credentials for CreateVolume operations
// Uses provisioner secret if available, otherwise falls back to driver credentials
func (p *Provider) ProvideForCreateVolume(ctx context.Context, parameters *storageclass.Parameters) (aws.Config, error) {
	if parameters.HasProvisionerSecret() {
		name, namespace := parameters.GetProvisionerSecretRef()
		klog.V(4).InfoS("Using provisioner secret for CreateVolume", "secretName", name, "secretNamespace", namespace)
		return p.getSecretCredentials(ctx, name, namespace)
	}

	klog.V(4).InfoS("Using driver credentials for CreateVolume")
	return p.getDriverCredentials(ctx)
}

// ProvideForDeleteVolume resolves controller credentials for DeleteVolume operations
// Retrieves credentials based on volume context metadata
func (p *Provider) ProvideForDeleteVolume(ctx context.Context, volumeContext map[string]string) (aws.Config, error) {
	// Check if volume was created with provisioner secret
	if secretName := volumeContext[constants.VolumeContextProvisionerSecretNameKey]; secretName != "" {
		secretNamespace := volumeContext[constants.VolumeContextProvisionerSecretNamespaceKey]
		if secretNamespace == "" {
			return aws.Config{}, fmt.Errorf("volume context has provisioner secret name but missing namespace")
		}
		klog.V(4).InfoS("Using provisioner secret for DeleteVolume", "secretName", secretName, "secretNamespace", secretNamespace)
		return p.getSecretCredentials(ctx, secretName, secretNamespace)
	}

	klog.V(4).InfoS("Using driver credentials for DeleteVolume")
	return p.getDriverCredentials(ctx)
}

// ProvideForNodePublish resolves node credentials for NodePublishVolume operations
// Uses node-publish secret if available, otherwise falls back to driver credentials
func (p *Provider) ProvideForNodePublish(ctx context.Context, parameters *storageclass.Parameters) (aws.Config, error) {
	if parameters.HasNodePublishSecret() {
		name, namespace := parameters.GetNodePublishSecretRef()
		klog.V(4).InfoS("Using node-publish secret for NodePublish", "secretName", name, "secretNamespace", namespace)
		return p.getSecretCredentials(ctx, name, namespace)
	}

	klog.V(4).InfoS("Using driver credentials for NodePublish")
	return p.getDriverCredentials(ctx)
}

// GetCredentialsFor provides generic credential resolution for any operation
func (p *Provider) GetCredentialsFor(ctx context.Context, operation string, parameters *storageclass.Parameters) (aws.Config, error) {
	switch operation {
	case "CreateVolume", "controller":
		return p.ProvideForCreateVolume(ctx, parameters)
	case "NodePublishVolume", "node":
		return p.ProvideForNodePublish(ctx, parameters)
	default:
		return aws.Config{}, fmt.Errorf("unsupported operation %q for credential resolution", operation)
	}
}

// getSecretCredentials retrieves AWS credentials from a Kubernetes secret with caching
func (p *Provider) getSecretCredentials(ctx context.Context, secretName, secretNamespace string) (aws.Config, error) {
	cacheKey := fmt.Sprintf("%s/%s", secretNamespace, secretName)

	// Check cache first
	p.cacheMutex.RLock()
	if entry, exists := p.credentialCache[cacheKey]; exists && time.Now().Before(entry.ExpiresAt) {
		p.cacheMutex.RUnlock()
		klog.V(5).InfoS("Using cached credentials", "secretName", secretName, "secretNamespace", secretNamespace)
		return entry.Config, nil
	}
	p.cacheMutex.RUnlock()

	// Retrieve secret from Kubernetes API
	secret, err := p.client.CoreV1().Secrets(secretNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to retrieve secret %s/%s: %w", secretNamespace, secretName, err)
	}

	// Validate secret contains required AWS credential fields
	if err := p.ValidateSecretCredentials(secret); err != nil {
		return aws.Config{}, fmt.Errorf("invalid credentials in secret %s/%s: %w", secretNamespace, secretName, err)
	}

	// Create AWS config from secret data
	awsConfig, err := p.CreateAWSConfigFromSecret(secret)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create AWS config from secret %s/%s: %w", secretNamespace, secretName, err)
	}

	// Cache the credentials
	p.cacheMutex.Lock()
	p.credentialCache[cacheKey] = &CredentialCacheEntry{
		Config:    awsConfig,
		ExpiresAt: time.Now().Add(p.cacheTTL),
	}
	p.cacheMutex.Unlock()

	klog.V(4).InfoS("Retrieved and cached secret credentials", "secretName", secretName, "secretNamespace", secretNamespace)
	return awsConfig, nil
}

// getDriverCredentials returns the driver-level AWS configuration with caching
func (p *Provider) getDriverCredentials(ctx context.Context) (aws.Config, error) {
	cacheKey := "driver-credentials"

	// Check cache first
	p.cacheMutex.RLock()
	if entry, exists := p.credentialCache[cacheKey]; exists && time.Now().Before(entry.ExpiresAt) {
		p.cacheMutex.RUnlock()
		klog.V(5).InfoS("Using cached driver credentials")
		return entry.Config, nil
	}
	p.cacheMutex.RUnlock()

	// Load driver credentials using default credential chain
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}

	// Cache the driver credentials
	p.cacheMutex.Lock()
	p.credentialCache[cacheKey] = &CredentialCacheEntry{
		Config:    cfg,
		ExpiresAt: time.Now().Add(p.cacheTTL),
	}
	p.cacheMutex.Unlock()

	klog.V(4).InfoS("Loaded and cached driver credentials")
	return cfg, nil
}

// ClearCache clears the credential cache (useful for testing or forced refresh)
func (p *Provider) ClearCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	p.credentialCache = make(map[string]*CredentialCacheEntry)
	klog.V(4).InfoS("Cleared credential cache")
}

// GetCacheStats returns cache statistics for monitoring
func (p *Provider) GetCacheStats() (total int, expired int) {
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()

	now := time.Now()
	total = len(p.credentialCache)

	for _, entry := range p.credentialCache {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}

	return total, expired
}

// SetCacheTTL updates the cache TTL (useful for testing or runtime configuration)
func (p *Provider) SetCacheTTL(ttl time.Duration) {
	p.cacheTTL = ttl
	klog.V(4).InfoS("Updated credential cache TTL", "ttl", ttl)
}

// ValidateSecretCredentials performs basic validation of AWS credentials in a Kubernetes secret.
// This validates only the presence of required fields.
func (p *Provider) ValidateSecretCredentials(secret *corev1.Secret) error {
	if secret == nil {
		return fmt.Errorf("secret is nil")
	}

	if secret.Data == nil {
		return fmt.Errorf("secret has no data")
	}

	// Check for required AWS credential fields
	accessKeyID := secret.Data[constants.AccessKeyIDField]
	secretAccessKey := secret.Data[constants.SecretAccessKeyField]

	if len(accessKeyID) == 0 {
		return fmt.Errorf("secret missing required field: %s", constants.AccessKeyIDField)
	}

	if len(secretAccessKey) == 0 {
		return fmt.Errorf("secret missing required field: %s", constants.SecretAccessKeyField)
	}

	return nil
}

// CreateAWSConfigFromSecret creates an AWS configuration using credentials from a Kubernetes secret.
// The secret must contain at least access_key_id and secret_access_key fields.
// Optional fields include session_token and region.
func (p *Provider) CreateAWSConfigFromSecret(secret *corev1.Secret) (aws.Config, error) {
	// Validate the secret first
	if err := p.ValidateSecretCredentials(secret); err != nil {
		return aws.Config{}, err
	}

	// Extract credentials
	accessKeyID := strings.TrimSpace(string(secret.Data[constants.AccessKeyIDField]))
	secretAccessKey := strings.TrimSpace(string(secret.Data[constants.SecretAccessKeyField]))

	// Optional session token for temporary credentials
	var sessionToken string
	if token := secret.Data[constants.SessionTokenField]; len(token) > 0 {
		sessionToken = strings.TrimSpace(string(token))
	}

	// Optional region override
	var region string
	if regionData := secret.Data[constants.RegionField]; len(regionData) > 0 {
		region = strings.TrimSpace(string(regionData))
	}

	// Create static credential provider
	credsProvider := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)

	// Load base config with static credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credsProvider),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Override region if provided in secret
	if region != "" {
		cfg.Region = region
	}

	return cfg, nil
}
