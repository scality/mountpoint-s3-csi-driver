package csicontroller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	crdv2 "github.com/scality/mountpoint-s3-csi-driver/pkg/api/v2"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
)

func TestStaleAttachmentCleaner_cleanupStaleWorkloads(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = crdv2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name              string
		s3pa              *crdv2.MountpointS3PodAttachment
		existingPods      map[string]*corev1.Pod
		expectedModified  bool
		expectedMPPods    int
		expectedWorkloads map[string]int // mpPodName -> workload count
	}{
		{
			name: "No stale workloads - pod exists",
			s3pa: &crdv2.MountpointS3PodAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-s3pa",
				},
				Spec: crdv2.MountpointS3PodAttachmentSpec{
					MountpointS3PodAttachments: map[string][]crdv2.WorkloadAttachment{
						"mp-pod-1": {
							{
								WorkloadPodUID: "pod-uid-1",
								AttachmentTime: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
							},
						},
					},
				},
			},
			existingPods: map[string]*corev1.Pod{
				"pod-uid-1": {
					ObjectMeta: metav1.ObjectMeta{
						UID: "pod-uid-1",
					},
				},
			},
			expectedModified:  false,
			expectedMPPods:    1,
			expectedWorkloads: map[string]int{"mp-pod-1": 1},
		},
		{
			name: "Stale workload - pod doesn't exist and old enough",
			s3pa: &crdv2.MountpointS3PodAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-s3pa",
				},
				Spec: crdv2.MountpointS3PodAttachmentSpec{
					MountpointS3PodAttachments: map[string][]crdv2.WorkloadAttachment{
						"mp-pod-1": {
							{
								WorkloadPodUID: "pod-uid-1",
								AttachmentTime: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
							},
						},
					},
				},
			},
			existingPods:      map[string]*corev1.Pod{},
			expectedModified:  true,
			expectedMPPods:    0,
			expectedWorkloads: map[string]int{},
		},
		{
			name: "Race condition protection - pod doesn't exist but too new",
			s3pa: &crdv2.MountpointS3PodAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-s3pa",
				},
				Spec: crdv2.MountpointS3PodAttachmentSpec{
					MountpointS3PodAttachments: map[string][]crdv2.WorkloadAttachment{
						"mp-pod-1": {
							{
								WorkloadPodUID: "pod-uid-1",
								AttachmentTime: metav1.NewTime(time.Now().Add(-30 * time.Second)),
							},
						},
					},
				},
			},
			existingPods:      map[string]*corev1.Pod{},
			expectedModified:  false,
			expectedMPPods:    1,
			expectedWorkloads: map[string]int{"mp-pod-1": 1},
		},
		{
			name: "Mixed workloads - some stale, some valid",
			s3pa: &crdv2.MountpointS3PodAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-s3pa",
				},
				Spec: crdv2.MountpointS3PodAttachmentSpec{
					MountpointS3PodAttachments: map[string][]crdv2.WorkloadAttachment{
						"mp-pod-1": {
							{
								WorkloadPodUID: "pod-uid-1",
								AttachmentTime: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
							},
							{
								WorkloadPodUID: "pod-uid-2",
								AttachmentTime: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
							},
						},
					},
				},
			},
			existingPods: map[string]*corev1.Pod{
				"pod-uid-2": {
					ObjectMeta: metav1.ObjectMeta{
						UID: "pod-uid-2",
					},
				},
			},
			expectedModified:  true,
			expectedMPPods:    1,
			expectedWorkloads: map[string]int{"mp-pod-1": 1},
		},
		{
			name: "Multiple mountpoint pods with mixed workloads",
			s3pa: &crdv2.MountpointS3PodAttachment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-s3pa",
				},
				Spec: crdv2.MountpointS3PodAttachmentSpec{
					MountpointS3PodAttachments: map[string][]crdv2.WorkloadAttachment{
						"mp-pod-1": {
							{
								WorkloadPodUID: "pod-uid-1",
								AttachmentTime: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
							},
						},
						"mp-pod-2": {
							{
								WorkloadPodUID: "pod-uid-2",
								AttachmentTime: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
							},
						},
					},
				},
			},
			existingPods: map[string]*corev1.Pod{
				"pod-uid-2": {
					ObjectMeta: metav1.ObjectMeta{
						UID: "pod-uid-2",
					},
				},
			},
			expectedModified:  true,
			expectedMPPods:    1,
			expectedWorkloads: map[string]int{"mp-pod-2": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake client with the S3PodAttachment
			objs := []client.Object{tt.s3pa}

			// Add any mountpoint pods that should exist
			for mpPodName := range tt.s3pa.Spec.MountpointS3PodAttachments {
				mpPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      mpPodName,
						Namespace: "mount-s3",
					},
				}
				objs = append(objs, mpPod)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			reconciler := &Reconciler{
				Client: fakeClient,
				mountpointPodConfig: mppod.Config{
					Namespace: "mount-s3",
				},
			}

			cleaner := NewStaleAttachmentCleaner(reconciler)

			// Run cleanup
			err := cleaner.cleanupStaleWorkloads(ctx, tt.s3pa, tt.existingPods)
			if err != nil {
				t.Fatalf("cleanupStaleWorkloads failed: %v", err)
			}

			// Check if S3PodAttachment was modified as expected
			if tt.expectedModified {
				// Get the updated S3PodAttachment
				updatedS3pa := &crdv2.MountpointS3PodAttachment{}
				err = fakeClient.Get(ctx, types.NamespacedName{Name: tt.s3pa.Name}, updatedS3pa)

				if tt.expectedMPPods == 0 {
					// Should have been deleted
					if err == nil {
						t.Errorf("Expected S3PodAttachment to be deleted, but it still exists")
					}
				} else {
					if err != nil {
						t.Fatalf("Failed to get updated S3PodAttachment: %v", err)
					}

					// Check the number of mountpoint pods
					if len(updatedS3pa.Spec.MountpointS3PodAttachments) != tt.expectedMPPods {
						t.Errorf("Expected %d mountpoint pods, got %d",
							tt.expectedMPPods, len(updatedS3pa.Spec.MountpointS3PodAttachments))
					}

					// Check workload counts for each mountpoint pod
					for mpPodName, expectedCount := range tt.expectedWorkloads {
						workloads := updatedS3pa.Spec.MountpointS3PodAttachments[mpPodName]
						if len(workloads) != expectedCount {
							t.Errorf("Expected %d workloads for %s, got %d",
								expectedCount, mpPodName, len(workloads))
						}
					}
				}
			}
		})
	}
}

func TestStaleAttachmentCleaner_RunCleanup(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = crdv2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Create test data
	existingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-pod",
			Namespace: "default",
			UID:       "existing-pod-uid",
		},
	}

	s3pa1 := &crdv2.MountpointS3PodAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "s3pa-1",
		},
		Spec: crdv2.MountpointS3PodAttachmentSpec{
			MountpointS3PodAttachments: map[string][]crdv2.WorkloadAttachment{
				"mp-pod-1": {
					{
						WorkloadPodUID: "existing-pod-uid",
						AttachmentTime: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
					},
					{
						WorkloadPodUID: "stale-pod-uid",
						AttachmentTime: metav1.NewTime(time.Now().Add(-3 * time.Minute)),
					},
				},
			},
		},
	}

	s3pa2 := &crdv2.MountpointS3PodAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "s3pa-2",
		},
		Spec: crdv2.MountpointS3PodAttachmentSpec{
			MountpointS3PodAttachments: map[string][]crdv2.WorkloadAttachment{
				"mp-pod-2": {
					{
						WorkloadPodUID: "stale-pod-uid-2",
						AttachmentTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
					},
				},
			},
		},
	}

	mpPod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mp-pod-1",
			Namespace: "mount-s3",
		},
	}

	mpPod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mp-pod-2",
			Namespace: "mount-s3",
		},
	}

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingPod, s3pa1, s3pa2, mpPod1, mpPod2).
		Build()

	reconciler := &Reconciler{
		Client: fakeClient,
		mountpointPodConfig: mppod.Config{
			Namespace: "mount-s3",
		},
	}

	cleaner := NewStaleAttachmentCleaner(reconciler)

	// Run cleanup
	ctx := context.Background()
	err := cleaner.RunCleanup(ctx)
	if err != nil {
		t.Fatalf("RunCleanup failed: %v", err)
	}

	// Check s3pa1 - should still exist with one workload
	updatedS3pa1 := &crdv2.MountpointS3PodAttachment{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "s3pa-1"}, updatedS3pa1)
	if err != nil {
		t.Fatalf("Failed to get s3pa-1: %v", err)
	}

	if len(updatedS3pa1.Spec.MountpointS3PodAttachments["mp-pod-1"]) != 1 {
		t.Errorf("Expected 1 workload for mp-pod-1, got %d",
			len(updatedS3pa1.Spec.MountpointS3PodAttachments["mp-pod-1"]))
	}

	if updatedS3pa1.Spec.MountpointS3PodAttachments["mp-pod-1"][0].WorkloadPodUID != "existing-pod-uid" {
		t.Errorf("Expected workload to be 'existing-pod-uid', got %s",
			updatedS3pa1.Spec.MountpointS3PodAttachments["mp-pod-1"][0].WorkloadPodUID)
	}

	// Check s3pa2 - should be deleted
	updatedS3pa2 := &crdv2.MountpointS3PodAttachment{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "s3pa-2"}, updatedS3pa2)
	if err == nil {
		t.Errorf("Expected s3pa-2 to be deleted, but it still exists")
	}

	// Check that mp-pod-2 was annotated for unmount
	updatedMpPod2 := &corev1.Pod{}
	err = fakeClient.Get(ctx, types.NamespacedName{Name: "mp-pod-2", Namespace: "mount-s3"}, updatedMpPod2)
	if err != nil {
		t.Fatalf("Failed to get mp-pod-2: %v", err)
	}

	if updatedMpPod2.Annotations["s3.csi.scality.com/needs-unmount"] != "true" {
		t.Errorf("Expected mp-pod-2 to have unmount annotation, but it doesn't")
	}
}

func TestStaleAttachmentCleaner_Start(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = crdv2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Create fake client
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := &Reconciler{
		Client: fakeClient,
		mountpointPodConfig: mppod.Config{
			Namespace: "mount-s3",
		},
	}

	cleaner := NewStaleAttachmentCleaner(reconciler)

	// Test that Start returns when context is cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := cleaner.Start(ctx)
	if err != nil {
		t.Fatalf("Start returned unexpected error: %v", err)
	}
}
