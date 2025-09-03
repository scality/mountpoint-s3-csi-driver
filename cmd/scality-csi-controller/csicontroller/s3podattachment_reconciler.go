package csicontroller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	crdv2 "github.com/scality/mountpoint-s3-csi-driver/pkg/api/v2"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
)

const (
	// Labels for tracking
	LabelCSIDriverVersion = constants.DriverName + "/mounted-by-csi-driver-version"
	LabelNodeName         = constants.DriverName + "/node-name"
	LabelPVName           = constants.DriverName + "/pv-name"
)

// S3PodAttachmentReconciler reconciles MountpointS3PodAttachment objects
type S3PodAttachmentReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	mountpointPodConfig  mppod.Config
	mountpointPodCreator *mppod.Creator
}

// NewS3PodAttachmentReconciler creates a new S3PodAttachmentReconciler
func NewS3PodAttachmentReconciler(client client.Client, scheme *runtime.Scheme, podConfig mppod.Config) *S3PodAttachmentReconciler {
	creator := mppod.NewCreator(podConfig)
	return &S3PodAttachmentReconciler{
		Client:               client,
		Scheme:               scheme,
		mountpointPodConfig:  podConfig,
		mountpointPodCreator: creator,
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *S3PodAttachmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("s3podattachment").
		For(&crdv2.MountpointS3PodAttachment{}).
		Owns(&corev1.Pod{}). // Watch pods we create
		Complete(r)
}

// Reconcile handles MountpointS3PodAttachment objects
func (r *S3PodAttachmentReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	klog.V(4).Infof("Reconciling MountpointS3PodAttachment %s", req.NamespacedName)

	// Fetch the MountpointS3PodAttachment
	s3pa := &crdv2.MountpointS3PodAttachment{}
	err := r.Get(ctx, req.NamespacedName, s3pa)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("MountpointS3PodAttachment %s not found, ignoring", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		klog.Errorf("Failed to get MountpointS3PodAttachment: %v", err)
		return reconcile.Result{}, err
	}

	// Process each Mountpoint Pod in the attachment
	for mpPodName, attachments := range s3pa.Spec.MountpointS3PodAttachments {
		if len(attachments) == 0 {
			klog.V(4).Infof("No attachments for Mountpoint Pod %s, skipping", mpPodName)
			continue
		}

		// Check if the Mountpoint Pod exists
		mpPod := &corev1.Pod{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      mpPodName,
			Namespace: r.mountpointPodConfig.Namespace,
		}, mpPod)

		if err != nil {
			if apierrors.IsNotFound(err) {
				// Pod doesn't exist, we need to create it
				klog.Infof("Mountpoint Pod %s not found, creating it", mpPodName)
				err = r.createMountpointPod(ctx, s3pa, mpPodName, attachments[0])
				if err != nil {
					klog.Errorf("Failed to create Mountpoint Pod %s: %v", mpPodName, err)
					// Requeue to retry
					return reconcile.Result{RequeueAfter: 5 * time.Second}, err
				}
			} else {
				klog.Errorf("Failed to get Mountpoint Pod %s: %v", mpPodName, err)
				return reconcile.Result{}, err
			}
		} else {
			// Pod exists, check its status
			klog.V(4).Infof("Mountpoint Pod %s already exists in phase %s", mpPodName, mpPod.Status.Phase)
			
			// Handle pod lifecycle
			switch mpPod.Status.Phase {
			case corev1.PodSucceeded:
				// Pod completed successfully, delete it
				klog.Infof("Mountpoint Pod %s succeeded, deleting it", mpPodName)
				err = r.Delete(ctx, mpPod)
				if err != nil {
					klog.Errorf("Failed to delete succeeded pod %s: %v", mpPodName, err)
				}
				// Also remove from attachment
				delete(s3pa.Spec.MountpointS3PodAttachments, mpPodName)
				err = r.Update(ctx, s3pa)
				if err != nil {
					klog.Errorf("Failed to update S3PodAttachment after pod deletion: %v", err)
				}
			case corev1.PodFailed:
				// Pod failed, log and potentially retry
				klog.Errorf("Mountpoint Pod %s failed: %s", mpPodName, mpPod.Status.Reason)
				// TODO: Implement retry logic with backoff
			}
		}
	}

	// Clean up S3PodAttachment if no more attachments
	if len(s3pa.Spec.MountpointS3PodAttachments) == 0 {
		klog.Infof("No more attachments in S3PodAttachment %s, deleting it", req.NamespacedName)
		err = r.Delete(ctx, s3pa)
		if err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to delete empty S3PodAttachment: %v", err)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// createMountpointPod creates a new Mountpoint Pod for the given attachment
func (r *S3PodAttachmentReconciler) createMountpointPod(ctx context.Context, s3pa *crdv2.MountpointS3PodAttachment, mpPodName string, attachment crdv2.WorkloadAttachment) error {
	// Get the PersistentVolume
	pv := &corev1.PersistentVolume{}
	err := r.Get(ctx, types.NamespacedName{Name: s3pa.Spec.PersistentVolumeName}, pv)
	if err != nil {
		return fmt.Errorf("failed to get PersistentVolume %s: %w", s3pa.Spec.PersistentVolumeName, err)
	}

	// Try to find the workload pod by UID
	workloadPod := &corev1.Pod{}
	podList := &corev1.PodList{}
	err = r.List(ctx, podList)
	if err != nil {
		klog.Warningf("Failed to list pods to find workload pod %s: %v", attachment.WorkloadPodUID, err)
		// Continue with minimal spec if we can't find the workload pod
		workloadPod = nil
	} else {
		found := false
		for _, pod := range podList.Items {
			if string(pod.UID) == attachment.WorkloadPodUID {
				workloadPod = &pod
				found = true
				break
			}
		}
		if !found {
			klog.Warningf("Could not find workload pod with UID %s, using minimal spec", attachment.WorkloadPodUID)
			workloadPod = nil
		}
	}

	// Create the Mountpoint Pod spec using the Creator
	var mpPod *corev1.Pod
	if workloadPod != nil {
		// Use the Creator to build a proper pod spec with workload pod info
		mpPod = r.mountpointPodCreator.Create(workloadPod, pv)
		// Override the name to match what we expect
		mpPod.Name = mpPodName
	} else {
		// Fallback to minimal spec if we don't have workload pod
		mpPod = r.createMountpointPodSpec(mpPodName, s3pa, pv)
	}

	// Add owner reference to the S3PodAttachment
	mpPod.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: crdv2.GroupVersion.String(),
			Kind:       "MountpointS3PodAttachment",
			Name:       s3pa.Name,
			UID:        s3pa.UID,
			Controller: func(b bool) *bool { return &b }(true),
		},
	}

	// Create the pod
	err = r.Create(ctx, mpPod)
	if err != nil {
		return fmt.Errorf("failed to create Mountpoint Pod: %w", err)
	}

	klog.Infof("Successfully created Mountpoint Pod %s", mpPodName)
	return nil
}

// createMountpointPodSpec creates the Pod specification for a Mountpoint Pod
func (r *S3PodAttachmentReconciler) createMountpointPodSpec(name string, s3pa *crdv2.MountpointS3PodAttachment, pv *corev1.PersistentVolume) *corev1.Pod {
	// Extract mount options
	mountOptions := []string{}
	if s3pa.Spec.MountOptions != "" {
		mountOptions = strings.Split(s3pa.Spec.MountOptions, ",")
	}

	// Create minimal pod spec as a fallback
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.mountpointPodConfig.Namespace,
			Labels: map[string]string{
				mppod.LabelMountpointVersion: r.mountpointPodConfig.MountpointVersion,
				mppod.LabelVolumeName:        pv.Name,
				mppod.LabelCSIDriverVersion:  r.mountpointPodConfig.CSIDriverVersion,
				LabelNodeName:                s3pa.Spec.NodeName,
				LabelPVName:                  s3pa.Spec.PersistentVolumeName,
			},
		},
		Spec: corev1.PodSpec{
			NodeName:          s3pa.Spec.NodeName,
			RestartPolicy:     corev1.RestartPolicyOnFailure,
			PriorityClassName: r.mountpointPodConfig.PriorityClassName,
			Tolerations: []corev1.Toleration{
				{Operator: corev1.TolerationOpExists}, // Tolerate all taints
			},
			Containers: []corev1.Container{
				{
					Name:            "mountpoint",
					Image:           r.mountpointPodConfig.Container.Image,
					ImagePullPolicy: r.mountpointPodConfig.Container.ImagePullPolicy,
					Command:         []string{r.mountpointPodConfig.Container.Command},
					Args:            mountOptions,
					SecurityContext: &corev1.SecurityContext{
						Privileged: func(b bool) *bool { return &b }(true), // Needed for mounting
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      mppod.CommunicationDirName,
							MountPath: "/" + mppod.CommunicationDirName,
						},
						{
							Name:             "kubelet-dir",
							MountPath:        "/var/lib/kubelet",
							MountPropagation: func(v corev1.MountPropagationMode) *corev1.MountPropagationMode { return &v }(corev1.MountPropagationBidirectional),
						},
						{
							Name:      "cache-dir",
							MountPath: "/tmp/mp-cache",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "VOLUME_ID",
							Value: s3pa.Spec.VolumeID,
						},
						{
							Name:  "MOUNT_OPTIONS",
							Value: s3pa.Spec.MountOptions,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: mppod.CommunicationDirName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
				{
					Name: "kubelet-dir",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/kubelet",
							Type: func(t corev1.HostPathType) *corev1.HostPathType { return &t }(corev1.HostPathDirectory),
						},
					},
				},
				{
					Name: "cache-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	return pod
}