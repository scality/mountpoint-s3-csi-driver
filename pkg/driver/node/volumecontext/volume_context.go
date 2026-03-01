// Package volumecontext provides utilities for accessing volume context passed via CSI RPC.
package volumecontext

const (
	BucketName           = "bucketName"
	AuthenticationSource = "authenticationSource"

	MountpointPodServiceAccountName = "mountpointPodServiceAccountName"

	// Resource configuration for Mountpoint containers
	MountpointContainerResourcesRequestsCpu    = "mountpointContainerResourcesRequestsCpu"
	MountpointContainerResourcesRequestsMemory = "mountpointContainerResourcesRequestsMemory"
	MountpointContainerResourcesLimitsCpu      = "mountpointContainerResourcesLimitsCpu"
	MountpointContainerResourcesLimitsMemory   = "mountpointContainerResourcesLimitsMemory"

	CSIServiceAccountName   = "csi.storage.k8s.io/serviceAccount.name"
	CSIServiceAccountTokens = "csi.storage.k8s.io/serviceAccount.tokens"
	CSIPodName              = "csi.storage.k8s.io/pod.name"
	CSIPodNamespace         = "csi.storage.k8s.io/pod.namespace"
	CSIPodUID               = "csi.storage.k8s.io/pod.uid"
)
