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

// A ContainerConfig represents configuration for containers in the spawned Mountpoint Pods.
type ContainerConfig struct {
	Command         string
	Image           string
	HeadroomImage   string // Image to use for headroom pods (typically a pause container)
	ImagePullPolicy corev1.PullPolicy
}

// TLSConfig holds TLS configuration for custom CA certificates in mounter pods.
type TLSConfig struct {
	// CACertSecretName is the name of the Kubernetes Secret containing custom CA certificate(s)
	CACertSecretName string
	// InitImage is the container image for the CA certificate installation initContainer
	InitImage string
	// InitImagePullPolicy is the pull policy for the init container image
	InitImagePullPolicy corev1.PullPolicy
	// InitResourcesReqCPU is the CPU request for the init container
	InitResourcesReqCPU resource.Quantity
	// InitResourcesReqMemory is the memory request for the init container
	InitResourcesReqMemory resource.Quantity
	// InitResourcesLimMemory is the memory limit for the init container
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
	TLS                         *TLSConfig // TLS configuration for custom CA certificates
}

// A Creator allows creating specification for Mountpoint Pods to schedule.
type Creator struct {
	config Config
}

// NewCreator creates a new creator with the given `config`.
func NewCreator(config Config) *Creator {
	return &Creator{config: config}
}

// Volume and container names for TLS certificate installation
const (
	tlsCustomCACertVolumeName = "custom-ca-cert"
	tlsEtcSSLCertsVolumeName  = "etc-ssl-certs"
	tlsInitContainerName      = "install-ca-cert"
)

// Create returns a new Mountpoint Pod spec to schedule for given `pod` and `pv`.
//
// It automatically assigns Mountpoint Pod to `pod`'s node.
// The name of the Mountpoint Pod is consistently generated from `pod` and `pv` using `MountpointPodNameFor` function.
func (c *Creator) Create(pod *corev1.Pod, pv *corev1.PersistentVolume) *corev1.Pod {
	node := pod.Spec.NodeName
	name := MountpointPodNameFor(string(pod.UID), pv.Name)

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      CommunicationDirName,
			MountPath: filepath.Join("/", CommunicationDirName),
		},
	}

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

	var initContainers []corev1.Container

	// Add TLS volumes and init container if custom CA certificate is configured
	if c.config.TLS != nil && c.config.TLS.CACertSecretName != "" {
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
			RestartPolicy:  corev1.RestartPolicyOnFailure,
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

// configureTLS adds TLS-related volumes, volume mounts, and init container for custom CA certificate installation.
// It uses an Alpine-based init container to install the custom CA certificate into the system trust store,
// then shares the certificate store with the main container via an emptyDir volume.
//
// This approach is necessary because mount-s3's s2n-tls library only reads from /etc/ssl/certs/ and does not
// support AWS_CA_BUNDLE or other environment variables for custom CA configuration.
func (c *Creator) configureTLS(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount, []corev1.Container) {
	tls := c.config.TLS

	// Add volume for the custom CA certificate Secret
	volumes = append(volumes, corev1.Volume{
		Name: tlsCustomCACertVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: tls.CACertSecretName,
				Items: []corev1.KeyToPath{
					{
						Key:  "ca-bundle.crt",
						Path: "ca-bundle.crt",
					},
				},
			},
		},
	})

	// Add emptyDir volume to share certificate store between init container and main container
	volumes = append(volumes, corev1.Volume{
		Name: tlsEtcSSLCertsVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	// Add volume mount for the certificate store in the main container
	// This replaces /etc/ssl/certs with the certificate store prepared by the init container
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      tlsEtcSSLCertsVolumeName,
		MountPath: "/etc/ssl/certs",
		ReadOnly:  true,
	})

	// Create the init container that installs the custom CA certificate
	// Uses Alpine because it has update-ca-certificates and ca-certificates package
	initContainer := corev1.Container{
		Name:            tlsInitContainerName,
		Image:           tls.InitImage,
		ImagePullPolicy: tls.InitImagePullPolicy,
		Command:         []string{"/bin/sh", "-c"},
		Args: []string{
			// Script to install custom CA certificate into Alpine's CA store
			// 1. Install ca-certificates package (provides update-ca-certificates and creates required directories)
			// 2. Copy custom CA to Alpine's certificate directory
			// 3. Run update-ca-certificates to install and create hash symlinks
			// 4. Copy the entire certificate store to the shared volume
			`set -e
echo "Installing ca-certificates package..."
apk add --no-cache ca-certificates
echo "Installing custom CA certificate..."
cp /custom-ca/ca-bundle.crt /usr/local/share/ca-certificates/custom-ca.crt
update-ca-certificates
cp -r /etc/ssl/certs/* /shared-certs/
echo "Custom CA certificate installed successfully"`,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      tlsCustomCACertVolumeName,
				MountPath: "/custom-ca",
				ReadOnly:  true,
			},
			{
				Name:      tlsEtcSSLCertsVolumeName,
				MountPath: "/shared-certs",
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    tls.InitResourcesReqCPU,
				corev1.ResourceMemory: tls.InitResourcesReqMemory,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: tls.InitResourcesLimMemory,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			// Need to write to /etc/ssl/certs and /usr/local/share/ca-certificates
			ReadOnlyRootFilesystem: ptr.To(false),
			// update-ca-certificates needs root to write to system directories
			RunAsNonRoot: ptr.To(false),
			RunAsUser:    ptr.To(int64(0)),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		},
	}

	return volumes, volumeMounts, []corev1.Container{initContainer}
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
