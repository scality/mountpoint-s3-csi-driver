package mppod_test

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/cluster"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/volumecontext"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
)

func TestHeadroomPod(t *testing.T) {
	testCases := []struct {
		name             string
		volumeAttributes map[string]string
		expectError      bool
		expectedErrorMsg string
		verifyPod        func(t *testing.T, pod *corev1.Pod)
	}{
		{
			name: "basic headroom pod creation",
			verifyPod: func(t *testing.T, pod *corev1.Pod) {
				if pod == nil {
					t.Fatal("Expected pod to not be nil")
					return
				}
				assert.Equals(t, "mount-s3", pod.Namespace)
				assert.Equals(t, "headroom-priority", pod.Spec.PriorityClassName)
				assert.Equals(t, 1, len(pod.Spec.Containers))
				assert.Equals(t, "pause", pod.Spec.Containers[0].Name)
				assert.Equals(t, "pause-image:latest", pod.Spec.Containers[0].Image)
			},
		},
		{
			name: "with resource requests",
			volumeAttributes: map[string]string{
				volumecontext.MountpointContainerResourcesRequestsCpu:    "100m",
				volumecontext.MountpointContainerResourcesRequestsMemory: "256Mi",
			},
			verifyPod: func(t *testing.T, pod *corev1.Pod) {
				if pod == nil {
					t.Fatal("Expected pod to not be nil")
					return
				}
				container := &pod.Spec.Containers[0]
				if container.Resources.Requests == nil {
					t.Fatal("Expected container.Resources.Requests to not be nil")
				}
				assert.Equals(t, resource.MustParse("100m"), container.Resources.Requests[corev1.ResourceCPU])
				assert.Equals(t, resource.MustParse("256Mi"), container.Resources.Requests[corev1.ResourceMemory])
			},
		},
		{
			name: "with resource limits",
			volumeAttributes: map[string]string{
				volumecontext.MountpointContainerResourcesLimitsCpu:    "500m",
				volumecontext.MountpointContainerResourcesLimitsMemory: "1Gi",
			},
			verifyPod: func(t *testing.T, pod *corev1.Pod) {
				if pod == nil {
					t.Fatal("Expected pod to not be nil")
					return
				}
				container := &pod.Spec.Containers[0]
				if container.Resources.Limits == nil {
					t.Fatal("Expected container.Resources.Limits to not be nil")
				}
				assert.Equals(t, resource.MustParse("500m"), container.Resources.Limits[corev1.ResourceCPU])
				assert.Equals(t, resource.MustParse("1Gi"), container.Resources.Limits[corev1.ResourceMemory])
			},
		},
		{
			name: "with invalid cpu request",
			volumeAttributes: map[string]string{
				volumecontext.MountpointContainerResourcesRequestsCpu: "invalid-cpu",
			},
			expectError:      true,
			expectedErrorMsg: "failed to parse quantity \"invalid-cpu\"",
		},
		{
			name: "with invalid memory limit",
			volumeAttributes: map[string]string{
				volumecontext.MountpointContainerResourcesLimitsMemory: "invalid-mem",
			},
			expectError:      true,
			expectedErrorMsg: "failed to parse quantity \"invalid-mem\"",
		},
		{
			name: "verify affinity rules",
			verifyPod: func(t *testing.T, pod *corev1.Pod) {
				if pod.Spec.Affinity == nil {
					t.Fatal("Expected pod.Spec.Affinity to not be nil")
				}
				if pod.Spec.Affinity.PodAffinity == nil {
					t.Fatal("Expected pod.Spec.Affinity.PodAffinity to not be nil")
				}
				assert.Equals(t, 1, len(pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution))

				term := pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]
				assert.Equals(t, "kubernetes.io/hostname", term.TopologyKey)
				assert.Equals(t, []string{"test-namespace"}, term.Namespaces)

				if term.LabelSelector == nil {
					t.Fatal("Expected term.LabelSelector to not be nil")
				}
				assert.Equals(t, 1, len(term.LabelSelector.MatchExpressions))
				expr := term.LabelSelector.MatchExpressions[0]
				assert.Equals(t, mppod.LabelHeadroomForWorkload, expr.Key)
				assert.Equals(t, metav1.LabelSelectorOpIn, expr.Operator)
				assert.Equals(t, []string{"workload-pod-uid"}, expr.Values)
			},
		},
		{
			name: "verify security context",
			verifyPod: func(t *testing.T, pod *corev1.Pod) {
				container := &pod.Spec.Containers[0]
				if container.SecurityContext == nil {
					t.Fatal("Expected container.SecurityContext to not be nil")
				}
				assert.Equals(t, ptr.To(false), container.SecurityContext.AllowPrivilegeEscalation)
				assert.Equals(t, ptr.To(true), container.SecurityContext.RunAsNonRoot)
				if container.SecurityContext.Capabilities == nil {
					t.Fatal("Expected container.SecurityContext.Capabilities to not be nil")
				}
				assert.Equals(t, []corev1.Capability{"ALL"}, container.SecurityContext.Capabilities.Drop)
				if container.SecurityContext.SeccompProfile == nil {
					t.Fatal("Expected container.SecurityContext.SeccompProfile to not be nil")
				}
				assert.Equals(t, corev1.SeccompProfileTypeRuntimeDefault, container.SecurityContext.SeccompProfile.Type)
			},
		},
		{
			name: "verify tolerations",
			verifyPod: func(t *testing.T, pod *corev1.Pod) {
				assert.Equals(t, 1, len(pod.Spec.Tolerations))
				assert.Equals(t, corev1.TolerationOpExists, pod.Spec.Tolerations[0].Operator)
			},
		},
		{
			name: "verify labels",
			verifyPod: func(t *testing.T, pod *corev1.Pod) {
				if pod.Labels == nil {
					t.Fatal("Expected pod.Labels to not be nil")
				}
				assert.Equals(t, "workload-pod-uid", pod.Labels[mppod.LabelHeadroomForPod])
				assert.Equals(t, "test-pv", pod.Labels[mppod.LabelHeadroomForVolume])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := mppod.Config{
				Namespace:                 "mount-s3",
				HeadroomPriorityClassName: "headroom-priority",
				Container: mppod.ContainerConfig{
					HeadroomImage: "pause-image:latest",
				},
				ClusterVariant: cluster.DefaultKubernetes,
			}

			creator := mppod.NewCreator(config)

			workloadPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "workload-pod",
					Namespace: "test-namespace",
					UID:       types.UID("workload-pod-uid"),
				},
			}

			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pv",
				},
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							VolumeAttributes: tc.volumeAttributes,
						},
					},
				},
			}

			pod, err := creator.HeadroomPod(workloadPod, pv)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}
				if tc.expectedErrorMsg != "" && !strings.Contains(err.Error(), tc.expectedErrorMsg) {
					t.Errorf("Expected error to contain %q, got %q", tc.expectedErrorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got %v", err)
				}
				if tc.verifyPod != nil {
					tc.verifyPod(t, pod)
				}
			}
		})
	}
}

func TestHeadroomPodNameFor(t *testing.T) {
	testCases := []struct {
		name         string
		workloadUID  string
		pvName       string
		expectedName string
	}{
		{
			name:         "standard pod and pv",
			workloadUID:  "pod-123",
			pvName:       "pv-456",
			expectedName: "hr-4a9703f375169c248f447afc10bed9459a12ded40eccd3104835ed3e",
		},
		{
			name:         "empty uid",
			workloadUID:  "",
			pvName:       "pv-789",
			expectedName: "hr-21739e0f8b2f3fda07bec18d48da2e8255b957504f32d311bd39cbee",
		},
		{
			name:         "empty pv name",
			workloadUID:  "pod-abc",
			pvName:       "",
			expectedName: "hr-e1d6cb42b532365443b43e5708daf02cc9799fad4c8d37e53e32192c",
		},
		{
			name:         "both empty",
			workloadUID:  "",
			pvName:       "",
			expectedName: "hr-d14a028c2a3a2bc9476102bb288234c415a2b01f828ea62ac5b3e42f",
		},
		{
			name:         "consistent hashing",
			workloadUID:  "test-uid",
			pvName:       "test-pv",
			expectedName: "hr-aae734ade8800262147c73f5dffabd7f293cbd616b5ffd7066914a58",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workloadPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID(tc.workloadUID),
				},
			}

			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.pvName,
				},
			}

			name := mppod.HeadroomPodNameFor(workloadPod, pv)
			assert.Equals(t, tc.expectedName, name)

			// Verify consistency - calling again should return same name
			name2 := mppod.HeadroomPodNameFor(workloadPod, pv)
			assert.Equals(t, name, name2)
		})
	}
}

func TestIsHeadroomPod(t *testing.T) {
	testCases := []struct {
		name       string
		podName    string
		isHeadroom bool
	}{
		{
			name:       "valid headroom pod",
			podName:    "hr-abc123",
			isHeadroom: true,
		},
		{
			name:       "valid headroom pod with long hash",
			podName:    "hr-d0f23a1ff0ad96e4cf087a2aa04b54f09a456b926b87f49b9b93c72c",
			isHeadroom: true,
		},
		{
			name:       "not a headroom pod - different prefix",
			podName:    "mp-abc123",
			isHeadroom: false,
		},
		{
			name:       "not a headroom pod - no prefix",
			podName:    "abc123",
			isHeadroom: false,
		},
		{
			name:       "edge case - hr without dash",
			podName:    "hrabc123",
			isHeadroom: false,
		},
		{
			name:       "edge case - just hr-",
			podName:    "hr-",
			isHeadroom: true,
		},
		{
			name:       "empty name",
			podName:    "",
			isHeadroom: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.podName,
				},
			}

			result := mppod.IsHeadroomPod(pod)
			assert.Equals(t, tc.isHeadroom, result)
		})
	}
}

func TestLabelWorkloadPodForHeadroomPod(t *testing.T) {
	testCases := []struct {
		name          string
		initialLabels map[string]string
		expectAdded   bool
	}{
		{
			name:          "add label to pod without labels",
			initialLabels: nil,
			expectAdded:   true,
		},
		{
			name:          "add label to pod with existing labels",
			initialLabels: map[string]string{"existing": "label"},
			expectAdded:   true,
		},
		{
			name:          "label already exists",
			initialLabels: map[string]string{mppod.LabelHeadroomForWorkload: "existing-uid"},
			expectAdded:   false,
		},
		{
			name:          "empty map",
			initialLabels: map[string]string{},
			expectAdded:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					UID:    types.UID("test-uid-123"),
					Labels: tc.initialLabels,
				},
			}

			added := mppod.LabelWorkloadPodForHeadroomPod(pod)
			assert.Equals(t, tc.expectAdded, added)

			// Verify label is set correctly
			if pod.Labels == nil {
				t.Fatal("Expected pod.Labels to not be nil")
			}
			if tc.expectAdded {
				assert.Equals(t, "test-uid-123", pod.Labels[mppod.LabelHeadroomForWorkload])
			}

			// Verify existing labels are preserved
			if tc.initialLabels != nil {
				for k, v := range tc.initialLabels {
					if k != mppod.LabelHeadroomForWorkload || !tc.expectAdded {
						assert.Equals(t, v, pod.Labels[k])
					}
				}
			}
		})
	}
}

func TestWorkloadHasLabelPodForHeadroomPod(t *testing.T) {
	testCases := []struct {
		name     string
		labels   map[string]string
		hasLabel bool
	}{
		{
			name:     "has label",
			labels:   map[string]string{mppod.LabelHeadroomForWorkload: "some-uid"},
			hasLabel: true,
		},
		{
			name:     "no label",
			labels:   map[string]string{"other": "label"},
			hasLabel: false,
		},
		{
			name:     "empty label value",
			labels:   map[string]string{mppod.LabelHeadroomForWorkload: ""},
			hasLabel: false,
		},
		{
			name:     "nil labels",
			labels:   nil,
			hasLabel: false,
		},
		{
			name:     "empty map",
			labels:   map[string]string{},
			hasLabel: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tc.labels,
				},
			}

			result := mppod.WorkloadHasLabelPodForHeadroomPod(pod)
			assert.Equals(t, tc.hasLabel, result)
		})
	}
}

func TestUnlabelWorkloadPodForHeadroomPod(t *testing.T) {
	testCases := []struct {
		name           string
		initialLabels  map[string]string
		expectRemoved  bool
		expectedLabels map[string]string
	}{
		{
			name:           "remove existing label",
			initialLabels:  map[string]string{mppod.LabelHeadroomForWorkload: "uid-123", "other": "label"},
			expectRemoved:  true,
			expectedLabels: map[string]string{"other": "label"},
		},
		{
			name:           "label doesn't exist",
			initialLabels:  map[string]string{"other": "label"},
			expectRemoved:  false,
			expectedLabels: map[string]string{"other": "label"},
		},
		{
			name:           "empty label value",
			initialLabels:  map[string]string{mppod.LabelHeadroomForWorkload: ""},
			expectRemoved:  false,
			expectedLabels: map[string]string{mppod.LabelHeadroomForWorkload: ""},
		},
		{
			name:          "nil labels",
			initialLabels: nil,
			expectRemoved: false,
		},
		{
			name:           "only headroom label",
			initialLabels:  map[string]string{mppod.LabelHeadroomForWorkload: "uid"},
			expectRemoved:  true,
			expectedLabels: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: tc.initialLabels,
				},
			}

			removed := mppod.UnlabelWorkloadPodForHeadroomPod(pod)
			assert.Equals(t, tc.expectRemoved, removed)

			// Verify expected labels
			if tc.expectedLabels != nil {
				if pod.Labels == nil {
					t.Fatal("Expected pod.Labels to not be nil")
				}
				assert.Equals(t, len(tc.expectedLabels), len(pod.Labels))
				for k, v := range tc.expectedLabels {
					assert.Equals(t, v, pod.Labels[k])
				}
			}
		})
	}
}

func TestShouldReserveHeadroomForMountpointPod(t *testing.T) {
	testCases := []struct {
		name            string
		schedulingGates []corev1.PodSchedulingGate
		shouldReserve   bool
	}{
		{
			name: "has headroom scheduling gate",
			schedulingGates: []corev1.PodSchedulingGate{
				{Name: mppod.SchedulingGateReserveHeadroomForMountpointPod},
			},
			shouldReserve: true,
		},
		{
			name: "has headroom scheduling gate among others",
			schedulingGates: []corev1.PodSchedulingGate{
				{Name: "other-gate"},
				{Name: mppod.SchedulingGateReserveHeadroomForMountpointPod},
				{Name: "another-gate"},
			},
			shouldReserve: true,
		},
		{
			name: "no headroom scheduling gate",
			schedulingGates: []corev1.PodSchedulingGate{
				{Name: "other-gate"},
			},
			shouldReserve: false,
		},
		{
			name:            "no scheduling gates",
			schedulingGates: nil,
			shouldReserve:   false,
		},
		{
			name:            "empty scheduling gates",
			schedulingGates: []corev1.PodSchedulingGate{},
			shouldReserve:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					SchedulingGates: tc.schedulingGates,
				},
			}

			result := mppod.ShouldReserveHeadroomForMountpointPod(pod)
			assert.Equals(t, tc.shouldReserve, result)
		})
	}
}

func TestUngateHeadroomSchedulingGateForWorkloadPod(t *testing.T) {
	testCases := []struct {
		name                    string
		initialSchedulingGates  []corev1.PodSchedulingGate
		expectedSchedulingGates []corev1.PodSchedulingGate
	}{
		{
			name: "remove headroom gate only",
			initialSchedulingGates: []corev1.PodSchedulingGate{
				{Name: mppod.SchedulingGateReserveHeadroomForMountpointPod},
			},
			expectedSchedulingGates: []corev1.PodSchedulingGate{},
		},
		{
			name: "remove headroom gate among others",
			initialSchedulingGates: []corev1.PodSchedulingGate{
				{Name: "gate1"},
				{Name: mppod.SchedulingGateReserveHeadroomForMountpointPod},
				{Name: "gate2"},
			},
			expectedSchedulingGates: []corev1.PodSchedulingGate{
				{Name: "gate1"},
				{Name: "gate2"},
			},
		},
		{
			name: "no headroom gate to remove",
			initialSchedulingGates: []corev1.PodSchedulingGate{
				{Name: "gate1"},
				{Name: "gate2"},
			},
			expectedSchedulingGates: []corev1.PodSchedulingGate{
				{Name: "gate1"},
				{Name: "gate2"},
			},
		},
		{
			name:                    "no scheduling gates",
			initialSchedulingGates:  nil,
			expectedSchedulingGates: nil,
		},
		{
			name: "multiple headroom gates",
			initialSchedulingGates: []corev1.PodSchedulingGate{
				{Name: mppod.SchedulingGateReserveHeadroomForMountpointPod},
				{Name: "middle"},
				{Name: mppod.SchedulingGateReserveHeadroomForMountpointPod},
			},
			expectedSchedulingGates: []corev1.PodSchedulingGate{
				{Name: "middle"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					SchedulingGates: tc.initialSchedulingGates,
				},
			}

			mppod.UngateHeadroomSchedulingGateForWorkloadPod(pod)

			if tc.expectedSchedulingGates == nil {
				if pod.Spec.SchedulingGates != nil {
					t.Errorf("Expected pod.Spec.SchedulingGates to be nil, got %v", pod.Spec.SchedulingGates)
				}
			} else {
				if pod.Spec.SchedulingGates == nil {
					t.Fatal("Expected pod.Spec.SchedulingGates to not be nil")
				}
				assert.Equals(t, len(tc.expectedSchedulingGates), len(pod.Spec.SchedulingGates))
				for i, gate := range tc.expectedSchedulingGates {
					assert.Equals(t, gate.Name, pod.Spec.SchedulingGates[i].Name)
				}
			}
		})
	}
}

func TestExtractVolumeAttributes(t *testing.T) {
	testCases := []struct {
		name               string
		pv                 *corev1.PersistentVolume
		expectedAttributes map[string]string
	}{
		{
			name: "pv with volume attributes",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							VolumeAttributes: map[string]string{
								"key1": "value1",
								"key2": "value2",
							},
						},
					},
				},
			},
			expectedAttributes: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "pv without CSI spec",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{},
			},
			expectedAttributes: map[string]string{},
		},
		{
			name: "pv with nil volume attributes",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							VolumeAttributes: nil,
						},
					},
				},
			},
			expectedAttributes: map[string]string{},
		},
		{
			name: "pv with empty volume attributes",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							VolumeAttributes: map[string]string{},
						},
					},
				},
			},
			expectedAttributes: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mppod.ExtractVolumeAttributes(tc.pv)
			if result == nil {
				t.Fatal("ExtractVolumeAttributes should never return nil")
			}
			assert.Equals(t, len(tc.expectedAttributes), len(result))
			for k, v := range tc.expectedAttributes {
				assert.Equals(t, v, result[k])
			}
		})
	}
}
