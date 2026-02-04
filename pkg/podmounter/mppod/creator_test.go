package mppod_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/cluster"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
)

const (
	namespace         = "mount-s3"
	mountpointVersion = "1.10.0"
	image             = "mp-image:latest"
	imagePullPolicy   = corev1.PullAlways
	command           = "/bin/scality-s3-csi-mounter"
	priorityClassName = "mount-s3-critical"
	testNode          = "test-node"
	testPodUID        = "test-pod-uid"
	testVolName       = "test-vol"
	csiDriverVersion  = "1.12.0"
)

func createTestConfig(clusterVariant cluster.Variant) mppod.Config {
	return mppod.Config{
		Namespace:         namespace,
		MountpointVersion: mountpointVersion,
		PriorityClassName: priorityClassName,
		Container: mppod.ContainerConfig{
			Image:           image,
			ImagePullPolicy: imagePullPolicy,
			Command:         command,
		},
		CSIDriverVersion: csiDriverVersion,
		ClusterVariant:   clusterVariant,
	}
}

func createAndVerifyPod(t *testing.T, clusterVariant cluster.Variant, expectedRunAsUser *int64) {
	creator := mppod.NewCreator(createTestConfig(clusterVariant))

	verifyDefaultValues := func(mpPod *corev1.Pod) {
		// This is a hash of `testPodUID` + `testVolName`
		assert.Equals(t, "mp-8ef7856a0c7f1d5706bd6af93fdc4bc90b33cf2ceb6769b4afd62586", mpPod.Name)
		assert.Equals(t, namespace, mpPod.Namespace)
		assert.Equals(t, map[string]string{
			mppod.LabelMountpointVersion: mountpointVersion,
			mppod.LabelPodUID:            testPodUID,
			mppod.LabelVolumeName:        testVolName,
			mppod.LabelCSIDriverVersion:  csiDriverVersion,
		}, mpPod.Labels)

		assert.Equals(t, priorityClassName, mpPod.Spec.PriorityClassName)
		assert.Equals(t, corev1.RestartPolicyOnFailure, mpPod.Spec.RestartPolicy)
		assert.Equals(t, []corev1.Volume{
			{
				Name: mppod.CommunicationDirName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium:    corev1.StorageMediumMemory,
						SizeLimit: resource.NewQuantity(mppod.EmptyDirSizeLimit, resource.BinarySI),
					},
				},
			},
		}, mpPod.Spec.Volumes)
		assert.Equals(t, &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchFields: []corev1.NodeSelectorRequirement{{
								Key:      metav1.ObjectNameField,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{testNode},
							}},
						},
					},
				},
			},
		}, mpPod.Spec.Affinity)
		assert.Equals(t, []corev1.Toleration{
			{Operator: corev1.TolerationOpExists},
		}, mpPod.Spec.Tolerations)

		assert.Equals(t, image, mpPod.Spec.Containers[0].Image)
		assert.Equals(t, imagePullPolicy, mpPod.Spec.Containers[0].ImagePullPolicy)
		assert.Equals(t, []string{command}, mpPod.Spec.Containers[0].Command)
		assert.Equals(t, ptr.To(false), mpPod.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation)
		assert.Equals(t, &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}, mpPod.Spec.Containers[0].SecurityContext.Capabilities)
		assert.Equals(t, expectedRunAsUser, mpPod.Spec.Containers[0].SecurityContext.RunAsUser)
		assert.Equals(t, ptr.To(true), mpPod.Spec.Containers[0].SecurityContext.RunAsNonRoot)
		assert.Equals(t, &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}, mpPod.Spec.Containers[0].SecurityContext.SeccompProfile)
		assert.Equals(t, []corev1.VolumeMount{
			{
				Name:      mppod.CommunicationDirName,
				MountPath: "/" + mppod.CommunicationDirName,
			},
		}, mpPod.Spec.Containers[0].VolumeMounts)
	}

	t.Run("Empty PV", func(t *testing.T) {
		mpPod := creator.Create(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(testPodUID),
			},
			Spec: corev1.PodSpec{
				NodeName: testNode,
			},
		}, &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: testVolName,
			},
		})

		verifyDefaultValues(mpPod)
	})

	t.Run("With ServiceAccountName specified in PV", func(t *testing.T) {
		mpPod := creator.Create(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(testPodUID),
			},
			Spec: corev1.PodSpec{
				NodeName: testNode,
			},
		}, &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: testVolName,
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						VolumeAttributes: map[string]string{
							"mountpointPodServiceAccountName": "mount-s3-sa",
						},
					},
				},
			},
		})

		verifyDefaultValues(mpPod)
		assert.Equals(t, "mount-s3-sa", mpPod.Spec.ServiceAccountName)
	})
}

func TestCreatingMountpointPods(t *testing.T) {
	createAndVerifyPod(t, cluster.DefaultKubernetes, ptr.To(int64(1000)))
}

func TestCreatingMountpointPodsInOpenShift(t *testing.T) {
	createAndVerifyPod(t, cluster.OpenShift, (*int64)(nil))
}

func TestNewCreator(t *testing.T) {
	config := mppod.Config{
		Namespace:         "test-namespace",
		MountpointVersion: "1.2.3",
		PriorityClassName: "high-priority",
		Container: mppod.ContainerConfig{
			Command:         "/bin/cmd",
			Image:           "test-image:latest",
			ImagePullPolicy: corev1.PullIfNotPresent,
		},
		CSIDriverVersion: "2.1.0",
		ClusterVariant:   cluster.DefaultKubernetes,
	}

	creator := mppod.NewCreator(config)
	if creator == nil {
		t.Fatal("NewCreator should not return nil")
	}

	// Create a simple pod to verify the creator uses the config
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: "test-uid",
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
		},
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pv",
		},
	}

	mpPod := creator.Create(pod, pv)

	// Verify config values are used
	assert.Equals(t, config.Namespace, mpPod.Namespace)
	assert.Equals(t, config.MountpointVersion, mpPod.Labels[mppod.LabelMountpointVersion])
	assert.Equals(t, config.CSIDriverVersion, mpPod.Labels[mppod.LabelCSIDriverVersion])
	assert.Equals(t, config.PriorityClassName, mpPod.Spec.PriorityClassName)
	assert.Equals(t, config.Container.Image, mpPod.Spec.Containers[0].Image)
	assert.Equals(t, config.Container.ImagePullPolicy, mpPod.Spec.Containers[0].ImagePullPolicy)
	assert.Equals(t, []string{config.Container.Command}, mpPod.Spec.Containers[0].Command)
}

func TestCreatingMountpointPodsWithTLS(t *testing.T) {
	tlsConfig := &mppod.TLSConfig{
		CACertSecretName:       "custom-ca-cert",
		InitImage:              "alpine:3.21",
		InitImagePullPolicy:    corev1.PullIfNotPresent,
		InitResourcesReqCPU:    resource.MustParse("10m"),
		InitResourcesReqMemory: resource.MustParse("16Mi"),
		InitResourcesLimMemory: resource.MustParse("64Mi"),
	}

	config := mppod.Config{
		Namespace:         namespace,
		MountpointVersion: mountpointVersion,
		PriorityClassName: priorityClassName,
		Container: mppod.ContainerConfig{
			Image:           image,
			ImagePullPolicy: imagePullPolicy,
			Command:         command,
		},
		CSIDriverVersion: csiDriverVersion,
		ClusterVariant:   cluster.DefaultKubernetes,
		TLS:              tlsConfig,
	}

	creator := mppod.NewCreator(config)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID(testPodUID),
		},
		Spec: corev1.PodSpec{
			NodeName: testNode,
		},
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: testVolName,
		},
	}

	mpPod := creator.Create(pod, pv)

	t.Run("has init container for CA cert installation", func(t *testing.T) {
		assert.Equals(t, 1, len(mpPod.Spec.InitContainers))
		initContainer := mpPod.Spec.InitContainers[0]
		assert.Equals(t, "install-ca-cert", initContainer.Name)
		assert.Equals(t, "alpine:3.21", initContainer.Image)
		assert.Equals(t, corev1.PullIfNotPresent, initContainer.ImagePullPolicy)
	})

	t.Run("init container has correct volume mounts", func(t *testing.T) {
		initContainer := mpPod.Spec.InitContainers[0]
		assert.Equals(t, 2, len(initContainer.VolumeMounts))

		// Check custom-ca-cert mount
		caCertMount := initContainer.VolumeMounts[0]
		assert.Equals(t, "custom-ca-cert", caCertMount.Name)
		assert.Equals(t, "/custom-ca", caCertMount.MountPath)
		assert.Equals(t, true, caCertMount.ReadOnly)

		// Check etc-ssl-certs mount
		sslCertsMount := initContainer.VolumeMounts[1]
		assert.Equals(t, "etc-ssl-certs", sslCertsMount.Name)
		assert.Equals(t, "/shared-certs", sslCertsMount.MountPath)
	})

	t.Run("init container has correct resources", func(t *testing.T) {
		initContainer := mpPod.Spec.InitContainers[0]
		assert.Equals(t, resource.MustParse("10m"), initContainer.Resources.Requests[corev1.ResourceCPU])
		assert.Equals(t, resource.MustParse("16Mi"), initContainer.Resources.Requests[corev1.ResourceMemory])
		assert.Equals(t, resource.MustParse("64Mi"), initContainer.Resources.Limits[corev1.ResourceMemory])
	})

	t.Run("init container runs as root for update-ca-certificates", func(t *testing.T) {
		initContainer := mpPod.Spec.InitContainers[0]
		assert.Equals(t, ptr.To(int64(0)), initContainer.SecurityContext.RunAsUser)
		assert.Equals(t, ptr.To(false), initContainer.SecurityContext.RunAsNonRoot)
	})

	t.Run("has TLS volumes", func(t *testing.T) {
		// Should have communication dir + custom-ca-cert + etc-ssl-certs = 3 volumes
		assert.Equals(t, 3, len(mpPod.Spec.Volumes))

		// Check custom-ca-cert secret volume
		var caCertVolume *corev1.Volume
		var sslCertsVolume *corev1.Volume
		for i := range mpPod.Spec.Volumes {
			if mpPod.Spec.Volumes[i].Name == "custom-ca-cert" {
				caCertVolume = &mpPod.Spec.Volumes[i]
			}
			if mpPod.Spec.Volumes[i].Name == "etc-ssl-certs" {
				sslCertsVolume = &mpPod.Spec.Volumes[i]
			}
		}

		if caCertVolume == nil {
			t.Fatal("custom-ca-cert volume not found")
		}
		assert.Equals(t, "custom-ca-cert", caCertVolume.Secret.SecretName)

		if sslCertsVolume == nil {
			t.Fatal("etc-ssl-certs volume not found")
		}
		if sslCertsVolume.EmptyDir == nil {
			t.Fatal("etc-ssl-certs volume should be an emptyDir")
		}
	})

	t.Run("main container has /etc/ssl/certs mount", func(t *testing.T) {
		mainContainer := mpPod.Spec.Containers[0]
		var sslCertsMount *corev1.VolumeMount
		for i := range mainContainer.VolumeMounts {
			if mainContainer.VolumeMounts[i].Name == "etc-ssl-certs" {
				sslCertsMount = &mainContainer.VolumeMounts[i]
				break
			}
		}

		if sslCertsMount == nil {
			t.Fatal("etc-ssl-certs volume mount not found in main container")
		}
		assert.Equals(t, "/etc/ssl/certs", sslCertsMount.MountPath)
		assert.Equals(t, true, sslCertsMount.ReadOnly)
	})
}

func TestCreatingMountpointPodsWithoutTLS(t *testing.T) {
	config := mppod.Config{
		Namespace:         namespace,
		MountpointVersion: mountpointVersion,
		PriorityClassName: priorityClassName,
		Container: mppod.ContainerConfig{
			Image:           image,
			ImagePullPolicy: imagePullPolicy,
			Command:         command,
		},
		CSIDriverVersion: csiDriverVersion,
		ClusterVariant:   cluster.DefaultKubernetes,
		TLS:              nil, // No TLS config
	}

	creator := mppod.NewCreator(config)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID(testPodUID),
		},
		Spec: corev1.PodSpec{
			NodeName: testNode,
		},
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: testVolName,
		},
	}

	mpPod := creator.Create(pod, pv)

	t.Run("no init container without TLS", func(t *testing.T) {
		assert.Equals(t, 0, len(mpPod.Spec.InitContainers))
	})

	t.Run("only communication dir volume without TLS", func(t *testing.T) {
		assert.Equals(t, 1, len(mpPod.Spec.Volumes))
		assert.Equals(t, mppod.CommunicationDirName, mpPod.Spec.Volumes[0].Name)
	})

	t.Run("main container only has communication dir mount without TLS", func(t *testing.T) {
		mainContainer := mpPod.Spec.Containers[0]
		assert.Equals(t, 1, len(mainContainer.VolumeMounts))
		assert.Equals(t, mppod.CommunicationDirName, mainContainer.VolumeMounts[0].Name)
	})
}
