// Package constants provides common constants for the Scality S3 CSI driver.
// This includes AWS credential fields, CSI specification parameters, and volume context keys.
package constants

const (
	// Secret field names for AWS credentials as expected in Kubernetes secrets
	// These match the format used by the existing node credential provider
	AccessKeyIDField     = "access_key_id"
	SecretAccessKeyField = "secret_access_key"
	SessionTokenField    = "session_token"
	RegionField          = "region"

	// CSI standard provisioner secret parameters (for controller operations)
	// These are defined by the CSI specification and used in StorageClass parameters
	ProvisionerSecretNameKey      = "csi.storage.k8s.io/provisioner-secret-name"
	ProvisionerSecretNamespaceKey = "csi.storage.k8s.io/provisioner-secret-namespace"

	// CSI standard node publish secret parameters (for node operations)
	// These are defined by the CSI specification and used in StorageClass parameters
	NodePublishSecretNameKey      = "csi.storage.k8s.io/node-publish-secret-name"
	NodePublishSecretNamespaceKey = "csi.storage.k8s.io/node-publish-secret-namespace"

	// Volume context keys for storing credential metadata
	// Used to pass credential information from controller to node
	VolumeContextProvisionerSecretNameKey      = "provisioner-secret-name"
	VolumeContextProvisionerSecretNamespaceKey = "provisioner-secret-namespace"
	VolumeContextNodePublishSecretNameKey      = "node-publish-secret-name"
	VolumeContextNodePublishSecretNamespaceKey = "node-publish-secret-namespace"
)
