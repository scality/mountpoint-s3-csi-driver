package storageclass

import (
	"fmt"
	"maps"
	"strings"

	"k8s.io/klog/v2"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
)

// Parameters represents parsed and validated StorageClass parameters for dynamic provisioning
type Parameters struct {
	// Provisioner secret configuration (used by CSI Controller for bucket operations)
	ProvisionerSecretName      string
	ProvisionerSecretNamespace string

	// Node publish secret configuration (used by CSI Node for mount operations)
	NodePublishSecretName      string
	NodePublishSecretNamespace string

	// Authentication tier automatically determined from parameter content
	AuthTier AuthenticationTier
}

// AuthenticationTier represents the credential resolution strategy
type AuthenticationTier int

const (
	// DriverCredentials - No secret parameters provided, use driver-level credentials
	DriverCredentials AuthenticationTier = iota
	// SecretCredentials - Secret parameters provided, use secret-based credentials
	SecretCredentials
)

// ParseAndValidate parses StorageClass parameters and validates them according to CSI driver policy
func ParseAndValidate(parameters map[string]string) (*Parameters, error) {
	if parameters == nil {
		return &Parameters{AuthTier: DriverCredentials}, nil
	}

	// Make a copy to avoid modifying the original
	params := make(map[string]string)
	maps.Copy(params, parameters)

	// Apply CSI driver parameter policy - strip unsupported parameters
	enforceCSIDriverParameterPolicy(params)

	// Parse and validate CSI secret parameters
	provisionerSecretName := strings.TrimSpace(params[constants.ProvisionerSecretNameKey])
	provisionerSecretNamespace := strings.TrimSpace(params[constants.ProvisionerSecretNamespaceKey])
	nodePublishSecretName := strings.TrimSpace(params[constants.NodePublishSecretNameKey])
	nodePublishSecretNamespace := strings.TrimSpace(params[constants.NodePublishSecretNamespaceKey])

	// Validate secret parameter consistency
	if err := validateSecretParameterConsistency(provisionerSecretName, provisionerSecretNamespace, "provisioner"); err != nil {
		return nil, err
	}
	if err := validateSecretParameterConsistency(nodePublishSecretName, nodePublishSecretNamespace, "node-publish"); err != nil {
		return nil, err
	}

	// Determine authentication tier based on parameter presence
	authTier := determineAuthenticationTier(provisionerSecretName, provisionerSecretNamespace, nodePublishSecretName, nodePublishSecretNamespace)

	result := &Parameters{
		ProvisionerSecretName:      provisionerSecretName,
		ProvisionerSecretNamespace: provisionerSecretNamespace,
		NodePublishSecretName:      nodePublishSecretName,
		NodePublishSecretNamespace: nodePublishSecretNamespace,
		AuthTier:                   authTier,
	}

	return result, nil
}

// enforceCSIDriverParameterPolicy strips parameters that are not supported by the CSI driver
// We only support CSI standard provisioner secret parameters, all others are silently ignored
func enforceCSIDriverParameterPolicy(parameters map[string]string) {
	supportedParams := map[string]bool{
		constants.ProvisionerSecretNameKey:      true,
		constants.ProvisionerSecretNamespaceKey: true,
		constants.NodePublishSecretNameKey:      true,
		constants.NodePublishSecretNamespaceKey: true,
	}

	// Remove any parameters that are not in our supported list
	for param := range parameters {
		if !supportedParams[param] {
			delete(parameters, param)
			klog.V(4).Infof("StorageClass parameter %q ignored: only CSI provisioner secret parameters are supported", param)
		}
	}
}

// validateSecretParameterConsistency ensures both secret name and namespace are provided if either is specified
func validateSecretParameterConsistency(secretName, secretNamespace, secretType string) error {
	hasName := secretName != ""
	hasNamespace := secretNamespace != ""

	if hasName && !hasNamespace {
		return fmt.Errorf("%s secret name provided but namespace is missing", secretType)
	}
	if !hasName && hasNamespace {
		return fmt.Errorf("%s secret namespace provided but name is missing", secretType)
	}

	return nil
}

// determineAuthenticationTier determines the credential resolution strategy based on parameter presence
func determineAuthenticationTier(provisionerName, provisionerNamespace, nodePublishName, nodePublishNamespace string) AuthenticationTier {
	// Check if any secret parameters are provided
	hasProvisionerSecret := provisionerName != "" || provisionerNamespace != ""
	hasNodePublishSecret := nodePublishName != "" || nodePublishNamespace != ""

	// No secret parameters = driver-level credentials
	if !hasProvisionerSecret && !hasNodePublishSecret {
		return DriverCredentials
	}

	// Any secret parameters = secret-based credentials
	return SecretCredentials
}

// String returns a human-readable representation of the authentication tier
func (t AuthenticationTier) String() string {
	switch t {
	case DriverCredentials:
		return "driver-credentials"
	case SecretCredentials:
		return "secret-credentials"
	default:
		return "unknown"
	}
}

// HasProvisionerSecret returns true if provisioner secret parameters are provided
// This indicates the controller should use the specified secret instead of driver credentials
func (p *Parameters) HasProvisionerSecret() bool {
	return p.ProvisionerSecretName != "" && p.ProvisionerSecretNamespace != ""
}

// HasNodePublishSecret returns true if node publish secret parameters are provided
// This indicates the node should use the specified secret instead of driver credentials
func (p *Parameters) HasNodePublishSecret() bool {
	return p.NodePublishSecretName != "" && p.NodePublishSecretNamespace != ""
}

// UsesDriverCredentials returns true if no secret parameters are provided
// and the driver should use its own configured credentials
func (p *Parameters) UsesDriverCredentials() bool {
	return p.AuthTier == DriverCredentials
}

// GetProvisionerSecretRef returns the provisioner secret name and namespace
// Returns empty strings if no provisioner secret is configured
func (p *Parameters) GetProvisionerSecretRef() (name, namespace string) {
	return p.ProvisionerSecretName, p.ProvisionerSecretNamespace
}

// GetNodePublishSecretRef returns the node publish secret name and namespace
// Returns empty strings if no node publish secret is configured
func (p *Parameters) GetNodePublishSecretRef() (name, namespace string) {
	return p.NodePublishSecretName, p.NodePublishSecretNamespace
}
