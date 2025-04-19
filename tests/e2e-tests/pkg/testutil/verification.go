package testutil

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// VerifyPVCreated checks if a PV with the given name exists and is in the expected state
func VerifyPVCreated(ctx context.Context, c clientset.Interface, pvName string) error {
	pv, err := c.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PV %s: %v", pvName, err)
	}

	if pv.Status.Phase != v1.VolumeBound && pv.Status.Phase != v1.VolumeAvailable {
		return fmt.Errorf("PV %s is not in bound or available state: %s", pvName, pv.Status.Phase)
	}

	return nil
}

// VerifyPVCBound checks if a PVC is bound to a PV
func VerifyPVCBound(ctx context.Context, c clientset.Interface, namespace, pvcName string) error {
	pvc, err := c.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC %s: %v", pvcName, err)
	}

	if pvc.Status.Phase != v1.ClaimBound {
		return fmt.Errorf("PVC %s is not bound: %s", pvcName, pvc.Status.Phase)
	}

	if pvc.Spec.VolumeName == "" {
		return fmt.Errorf("PVC %s is not bound to any PV", pvcName)
	}

	return nil
}

// VerifyPodHasVolumeMounted checks if a Pod has the volume mounted
func VerifyPodHasVolumeMounted(ctx context.Context, c clientset.Interface, namespace, podName, volumeName string) error {
	pod, err := c.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Pod %s: %v", podName, err)
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.Name == volumeName {
			return nil
		}
	}

	return fmt.Errorf("Pod %s does not have volume %s mounted", podName, volumeName)
}

// VerifyPodRunning checks if a Pod is in the Running state
func VerifyPodRunning(ctx context.Context, c clientset.Interface, namespace, podName string) error {
	pod, err := c.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Pod %s: %v", podName, err)
	}

	if pod.Status.Phase != v1.PodRunning {
		return fmt.Errorf("Pod %s is not running: %s", podName, pod.Status.Phase)
	}

	return nil
}

// VerifyStorageClassExists checks if a StorageClass exists
func VerifyStorageClassExists(ctx context.Context, c clientset.Interface, storageClassName string) error {
	_, err := c.StorageV1().StorageClasses().Get(ctx, storageClassName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get StorageClass %s: %v", storageClassName, err)
	}

	return nil
}
