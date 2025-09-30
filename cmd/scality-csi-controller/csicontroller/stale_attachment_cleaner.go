package csicontroller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	crdv2 "github.com/scality/mountpoint-s3-csi-driver/pkg/api/v2"
)

const (
	// Use AWS's proven intervals
	cleanupInterval          = 2 * time.Minute
	staleAttachmentThreshold = 2 * time.Minute
)

// StaleAttachmentCleaner handles periodic cleanup of stale workload attachments
// in case reconciler missed pod deletion event.
type StaleAttachmentCleaner struct {
	reconciler *Reconciler
}

// NewStaleAttachmentCleaner creates a new StaleAttachmentCleaner
func NewStaleAttachmentCleaner(reconciler *Reconciler) *StaleAttachmentCleaner {
	return &StaleAttachmentCleaner{
		reconciler: reconciler,
	}
}

// Start begins the periodic cleanup process
func (cm *StaleAttachmentCleaner) Start(ctx context.Context) error {
	log := logf.FromContext(ctx)
	log.Info("Starting stale attachment cleaner")

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Completed stale attachment cleaner")
			return nil
		case <-ticker.C:
			if err := cm.RunCleanup(ctx); err != nil {
				log.Error(err, "Failed to run cleanup")
				// Continue running even if cleanup fails
			}
		}
	}
}

// RunCleanup cleans up stale attachments and Mountpoint Pods.
func (cm *StaleAttachmentCleaner) RunCleanup(ctx context.Context) error {
	log := logf.FromContext(ctx)

	// Get all pods in the cluster
	podList := &corev1.PodList{}
	if err := cm.reconciler.List(ctx, podList); err != nil {
		return err
	}

	// Create a map of existing pod UIDs for quick lookup
	existingPods := make(map[string]*corev1.Pod)
	for i := range podList.Items {
		pod := &podList.Items[i]
		existingPods[string(pod.UID)] = pod
	}

	// Get all MountpointS3PodAttachments
	s3paList := &crdv2.MountpointS3PodAttachmentList{}
	if err := cm.reconciler.List(ctx, s3paList); err != nil {
		return err
	}

	// Check each S3PodAttachment for stale workload references
	for i := range s3paList.Items {
		s3pa := &s3paList.Items[i]
		if err := cm.cleanupStaleWorkloads(ctx, s3pa, existingPods); err != nil {
			log.Error(err, "Error cleaning up S3PodAttachment", "s3pa", s3pa.Name)
			continue
		}
	}

	return nil
}

// cleanupStaleWorkloads removes stale workload references from a single S3PodAttachment.
// A workload reference is considered stale if:
// 1. The referenced Pod no longer exists in the cluster
// 2. The attachment is older than staleAttachmentThreshold (to avoid race conditions)
//
// If a Mountpoint Pod has zero attachments after cleanup, it's marked for unmounting.
// If S3PodAttachment has no remaining Mountpoint Pods, the entire S3PodAttachment is deleted.
func (cm *StaleAttachmentCleaner) cleanupStaleWorkloads(
	ctx context.Context,
	s3pa *crdv2.MountpointS3PodAttachment,
	existingPods map[string]*corev1.Pod,
) error {
	log := logf.FromContext(ctx).WithValues("s3pa", s3pa.Name)
	modified := false
	now := time.Now().UTC()

	// Check each Mountpoint Pod's workload attachments
	mpPodsToDelete := []string{}
	for mpPodName, workloads := range s3pa.Spec.MountpointS3PodAttachments {
		validWorkloads := []crdv2.WorkloadAttachment{}

		for _, workload := range workloads {
			// Check if workload pod still exists
			if _, exists := existingPods[workload.WorkloadPodUID]; exists {
				validWorkloads = append(validWorkloads, workload)
			} else if now.Sub(workload.AttachmentTime.Time) > staleAttachmentThreshold {
				// Pod doesn't exist and attachment is old enough
				log.Info("Removing stale workload attachment",
					"mpPod", mpPodName,
					"workloadUID", workload.WorkloadPodUID,
					"age", now.Sub(workload.AttachmentTime.Time))
				modified = true
			} else {
				// Keep it for now (might be a race condition)
				validWorkloads = append(validWorkloads, workload)
			}
		}

		if len(validWorkloads) == 0 {
			// No valid workloads, mark Mountpoint Pod for deletion
			mpPodsToDelete = append(mpPodsToDelete, mpPodName)

			// Add unmount annotation to Mountpoint Pod
			mpPod := &corev1.Pod{}
			mpPodKey := types.NamespacedName{
				Namespace: cm.reconciler.mountpointPodConfig.Namespace,
				Name:      mpPodName,
			}
			if err := cm.reconciler.Get(ctx, mpPodKey, mpPod); err == nil {
				if mpPod.Annotations == nil {
					mpPod.Annotations = make(map[string]string)
				}
				mpPod.Annotations["s3.csi.scality.com/needs-unmount"] = "true"
				if err := cm.reconciler.Update(ctx, mpPod); err != nil {
					log.Error(err, "Failed to add unmount annotation", "mpPod", mpPodName)
				}
			}
		} else {
			s3pa.Spec.MountpointS3PodAttachments[mpPodName] = validWorkloads
		}
	}

	// Remove entries for Mountpoint Pods with no workloads
	for _, mpPodName := range mpPodsToDelete {
		delete(s3pa.Spec.MountpointS3PodAttachments, mpPodName)
		modified = true
		log.Info("Removed Mountpoint Pod entry with no workloads", "mpPod", mpPodName)
	}

	// Update S3PodAttachment if modified
	if modified {
		if len(s3pa.Spec.MountpointS3PodAttachments) == 0 {
			// Delete entire S3PodAttachment if no Mountpoint Pods remain
			log.Info("Deleting S3PodAttachment with no remaining Mountpoint Pods")
			if err := cm.reconciler.Delete(ctx, s3pa); err != nil {
				return err
			}
		} else {
			// Update S3PodAttachment with remaining entries
			if err := cm.reconciler.Update(ctx, s3pa); err != nil {
				return err
			}
		}
	}

	return nil
}
