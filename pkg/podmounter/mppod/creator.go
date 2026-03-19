package mppod

import (
	"fmt"
	"path/filepath"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/cluster"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/volumecontext"
)

// Labels populated on spawned Mountpoint Pods.
const (
	LabelMountpointVersion = constants.DriverName + "/mountpoint-version"
	LabelPodUID            = constants.DriverName + "/pod-uid"
	LabelVolumeName        = constants.DriverName + "/volume-name"
	LabelCSIDriverVersion  = constants.DriverName + "/mounted-by-csi-driver-version"
)

const EmptyDirSizeLimit = 10 * 1024 * 1024 // 10MiB

const TLSEmptyDirSizeLimit = 2 * 1024 * 1024 // 2MiB — room for system CA bundle (~200KB) + custom CAs

// Volume and container name constants for TLS configuration.
const (
	TLSCACertVolumeName      = "custom-ca-cert"
	TLSEtcSSLCertsVolumeName = "etc-ssl-certs"
	TLSInitContainerName     = "install-ca-cert"
)

// A ContainerConfig represents configuration for containers in the spawned Mountpoint Pods.
type ContainerConfig struct {
	Command         string
	Image           string
	HeadroomImage   string // Image to use for headroom pods (typically a pause container)
	ImagePullPolicy corev1.PullPolicy
}

// TLSConfig holds TLS configuration for custom CA certificates in mounter pods.
type TLSConfig struct {
	CACertConfigMapName    string
	InitImage              string
	InitImagePullPolicy    corev1.PullPolicy
	InitResourcesReqCPU    resource.Quantity
	InitResourcesReqMemory resource.Quantity
	InitResourcesLimMemory resource.Quantity
}

// A Config represents configuration for spawned Mountpoint Pods.
type Config struct {
	Namespace                   string
	MountpointVersion           string
	PriorityClassName           string
	PreemptingPriorityClassName string // Priority class for pods that can preempt headroom pods
	HeadroomPriorityClassName   string // Priority class for headroom pods (typically low priority)
	Container                   ContainerConfig
	CSIDriverVersion            string
	ClusterVariant              cluster.Variant
	TLS                         *TLSConfig
}

// A Creator allows creating specification for Mountpoint Pods to schedule.
type Creator struct {
	config Config
}

// NewCreator creates a new creator with the given `config`.
func NewCreator(config Config) *Creator {
	return &Creator{config: config}
}

// Create returns a new Mountpoint Pod spec to schedule for given `pod` and `pv`.
//
// It automatically assigns Mountpoint Pod to `pod`'s node.
// The name of the Mountpoint Pod is consistently generated from `pod` and `pv` using `MountpointPodNameFor` function.
func (c *Creator) Create(pod *corev1.Pod, pv *corev1.PersistentVolume) *corev1.Pod {
	node := pod.Spec.NodeName
	name := MountpointPodNameFor(string(pod.UID), pv.Name)

	volumes := []corev1.Volume{
		// This emptyDir volume is used for communication between Mountpoint Pod and the CSI Driver Node Pod
		{
			Name: CommunicationDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium:    corev1.StorageMediumMemory,
					SizeLimit: resource.NewQuantity(EmptyDirSizeLimit, resource.BinarySI),
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      CommunicationDirName,
			MountPath: filepath.Join("/", CommunicationDirName),
		},
	}

	var initContainers []corev1.Container
	if c.config.TLS != nil && c.config.TLS.CACertConfigMapName != "" {
		volumes, volumeMounts, initContainers = c.configureTLS(volumes, volumeMounts)
	}

	mpPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.config.Namespace,
			Labels: map[string]string{
				LabelMountpointVersion: c.config.MountpointVersion,
				LabelPodUID:            string(pod.UID),
				LabelVolumeName:        pv.Name,
				LabelCSIDriverVersion:  c.config.CSIDriverVersion,
			},
		},
		Spec: corev1.PodSpec{
			// Mountpoint terminates with zero exit code on a successful termination,
			// and in turn `/bin/scality-s3-csi-mounter` also exits with Mountpoint process' exit code,
			// here `restartPolicy: OnFailure` allows Pod to only restart on non-zero exit codes (i.e. some failures)
			// and not successful exists (i.e. zero exit code).
			RestartPolicy: corev1.RestartPolicyOnFailure,
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup: c.config.ClusterVariant.MountpointPodUserID(),
			},
			InitContainers: initContainers,
			Containers: []corev1.Container{{
				Name:            "mountpoint",
				Image:           c.config.Container.Image,
				ImagePullPolicy: c.config.Container.ImagePullPolicy,
				Command:         []string{c.config.Container.Command},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					RunAsUser:    c.config.ClusterVariant.MountpointPodUserID(),
					RunAsNonRoot: ptr.To(true),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				VolumeMounts: volumeMounts,
			}},
			PriorityClassName: c.config.PriorityClassName,
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					// This is to making sure Mountpoint Pod gets scheduled into same node as the Workload Pod
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchFields: []corev1.NodeSelectorRequirement{{
									Key:      metav1.ObjectNameField,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{node},
								}},
							},
						},
					},
				},
			},
			Tolerations: []corev1.Toleration{
				// Tolerate all taints.
				// - "NoScheduled" – If the Workload Pod gets scheduled to a node, Mountpoint Pod should also get
				//   scheduled into the same node to provide the volume.
				// - "NoExecute" – If the Workload Pod tolerates a "NoExecute" taint, Mountpoint Pod should also
				//   tolerate it to keep running and provide volume for the Workload Pod.
				//   If the Workload Pod would get descheduled and then the corresponding Mountpoint Pod
				//   would also get descheduled naturally due to CSI volume lifecycle.
				{Operator: corev1.TolerationOpExists},
			},
			Volumes: volumes,
		},
	}

	volumeAttributes := extractVolumeAttributes(pv)

	if saName := volumeAttributes[volumecontext.MountpointPodServiceAccountName]; saName != "" {
		mpPod.Spec.ServiceAccountName = saName
	}

	return mpPod
}

// configureTLS adds TLS-related volumes, volume mounts, and init containers for custom CA certificate support.
// The init container installs the CA certificate into the system trust store so mount-s3's s2n-tls can use it.
func (c *Creator) configureTLS(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount, []corev1.Container) {
	// ConfigMap volume with ca-bundle.crt key.
	// CA certificates are public, non-sensitive data — ConfigMap is the standard K8s choice over Secret.
	// Items selects only the ca-bundle.crt key to avoid mounting unrelated keys if the ConfigMap is shared.
	volumes = append(volumes, corev1.Volume{
		Name: TLSCACertVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: c.config.TLS.CACertConfigMapName,
				},
				Items: []corev1.KeyToPath{{Key: "ca-bundle.crt", Path: "ca-bundle.crt"}},
			},
		},
	})

	// emptyDir for shared cert store between init container and main container
	volumes = append(volumes, corev1.Volume{
		Name: TLSEtcSSLCertsVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				SizeLimit: resource.NewQuantity(TLSEmptyDirSizeLimit, resource.BinarySI),
			},
		},
	})

	// Mount the shared cert store in the main container (read-only)
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      TLSEtcSSLCertsVolumeName,
		MountPath: "/etc/ssl/certs",
		ReadOnly:  true,
	})

	// Init container that builds a combined CA bundle for mount-s3's s2n-tls.
	//
	// Why an init container instead of mounting the ConfigMap directly at /etc/ssl/certs?
	// Mounting the ConfigMap there would shadow the system CA bundle, leaving mount-s3
	// unable to verify well-known CAs (e.g., AWS endpoints, OCSP responders).
	// Instead, we copy the system bundle and append the custom CA into a shared emptyDir,
	// so s2n-tls finds both default and custom CAs in /etc/ssl/certs/ca-certificates.crt.
	//
	// Runs as non-root to comply with PodSecurity "restricted" policy.
	initContainers := []corev1.Container{
		{
			Name:            TLSInitContainerName,
			Image:           c.config.TLS.InitImage,
			ImagePullPolicy: c.config.TLS.InitImagePullPolicy,
			Command: []string{
				"sh", "-c",
				"set -e; cp /etc/ssl/certs/ca-certificates.crt /shared-certs/ca-certificates.crt; echo >> /shared-certs/ca-certificates.crt; cat /custom-ca/ca-bundle.crt >> /shared-certs/ca-certificates.crt",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      TLSCACertVolumeName,
					MountPath: "/custom-ca",
					ReadOnly:  true,
				},
				{
					Name:      TLSEtcSSLCertsVolumeName,
					MountPath: "/shared-certs",
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    c.config.TLS.InitResourcesReqCPU,
					corev1.ResourceMemory: c.config.TLS.InitResourcesReqMemory,
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: c.config.TLS.InitResourcesLimMemory,
				},
			},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    c.config.ClusterVariant.MountpointPodUserID(),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
		},
	}

	return volumes, volumeMounts, initContainers
}

// extractVolumeAttributes extracts volume attributes from given `pv`.
// It always returns a non-nil map, and it's safe to use even though `pv` doesn't contain any volume attributes.
func extractVolumeAttributes(pv *corev1.PersistentVolume) map[string]string {
	csiSpec := pv.Spec.CSI
	if csiSpec == nil {
		return map[string]string{}
	}

	volumeAttributes := csiSpec.VolumeAttributes
	if volumeAttributes == nil {
		return map[string]string{}
	}

	return volumeAttributes
}

// ExtractVolumeAttributes is a public wrapper for extractVolumeAttributes for use in other packages
func ExtractVolumeAttributes(pv *corev1.PersistentVolume) map[string]string {
	return extractVolumeAttributes(pv)
}

// configureResourceRequests configures resource requests of the container if its specified in the volume attributes.
func (c *Creator) configureResourceRequests(mpContainer *corev1.Container, volumeAttributes map[string]string) error {
	resourceRequestsCpu := volumeAttributes[volumecontext.MountpointContainerResourcesRequestsCpu]
	resourceRequestsMemory := volumeAttributes[volumecontext.MountpointContainerResourcesRequestsMemory]

	if resourceRequestsCpu != "" || resourceRequestsMemory != "" {
		mpContainer.Resources.Requests = make(corev1.ResourceList)

		if resourceRequestsCpu != "" {
			quantity, err := resource.ParseQuantity(resourceRequestsCpu)
			if err != nil {
				return failedToParseQuantityError(err, volumecontext.MountpointContainerResourcesRequestsCpu, resourceRequestsCpu)
			}
			mpContainer.Resources.Requests[corev1.ResourceCPU] = quantity
		}

		if resourceRequestsMemory != "" {
			quantity, err := resource.ParseQuantity(resourceRequestsMemory)
			if err != nil {
				return failedToParseQuantityError(err, volumecontext.MountpointContainerResourcesRequestsMemory, resourceRequestsMemory)
			}
			mpContainer.Resources.Requests[corev1.ResourceMemory] = quantity
		}
	}

	return nil
}

// configureResourceLimits configures resource limits of the container if its specified in the volume attributes.
func (c *Creator) configureResourceLimits(mpContainer *corev1.Container, volumeAttributes map[string]string) error {
	resourceLimitsCpu := volumeAttributes[volumecontext.MountpointContainerResourcesLimitsCpu]
	resourceLimitsMemory := volumeAttributes[volumecontext.MountpointContainerResourcesLimitsMemory]

	if resourceLimitsCpu != "" || resourceLimitsMemory != "" {
		mpContainer.Resources.Limits = make(corev1.ResourceList)

		if resourceLimitsCpu != "" {
			quantity, err := resource.ParseQuantity(resourceLimitsCpu)
			if err != nil {
				return failedToParseQuantityError(err, volumecontext.MountpointContainerResourcesLimitsCpu, resourceLimitsCpu)
			}
			mpContainer.Resources.Limits[corev1.ResourceCPU] = quantity
		}

		if resourceLimitsMemory != "" {
			quantity, err := resource.ParseQuantity(resourceLimitsMemory)
			if err != nil {
				return failedToParseQuantityError(err, volumecontext.MountpointContainerResourcesLimitsMemory, resourceLimitsMemory)
			}
			mpContainer.Resources.Limits[corev1.ResourceMemory] = quantity
		}
	}

	return nil
}

// failedToParseQuantityError creates an error if provided quantity is not parsable.
func failedToParseQuantityError(err error, field, value string) error {
	return fmt.Errorf("failed to parse quantity %q for %q: %w", value, field, err)
}
