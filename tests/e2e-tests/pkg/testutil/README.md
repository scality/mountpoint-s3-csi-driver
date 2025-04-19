# Test Utilities Package

This package provides utility functions for verification and common operations used in the E2E tests for the Scality S3 CSI Driver.

## Overview

The test utilities package includes helpers for:

- Verifying Kubernetes resource states (PVs, PVCs, Pods, etc.)
- Common test operations
- Resource validation utilities

## Functions

### Kubernetes Resource Verification

- `VerifyPVCreated(ctx, clientset, pvName)`: Checks if a PV exists and is in the expected state
- `VerifyPVCBound(ctx, clientset, namespace, pvcName)`: Checks if a PVC is bound to a PV
- `VerifyPodHasVolumeMounted(ctx, clientset, namespace, podName, volumeName)`: Checks if a Pod has a volume mounted
- `VerifyPodRunning(ctx, clientset, namespace, podName)`: Checks if a Pod is in the Running state
- `VerifyStorageClassExists(ctx, clientset, storageClassName)`: Checks if a StorageClass exists

## Usage

```go
import (
    "context"
    
    "github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/testutil"
    "k8s.io/client-go/kubernetes"
)

func TestExample(t *testing.T) {
    ctx := context.Background()
    clientset := kubernetes.NewForConfigOrDie(config)
    
    // Verify a persistent volume was created
    err := testutil.VerifyPVCreated(ctx, clientset, "my-pv")
    if err != nil {
        t.Fatalf("Failed to verify PV: %v", err)
    }
    
    // Verify a persistent volume claim is bound
    err = testutil.VerifyPVCBound(ctx, clientset, "default", "my-pvc")
    if err != nil {
        t.Fatalf("Failed to verify PVC: %v", err)
    }
    
    // Verify a pod is running with the volume mounted
    err = testutil.VerifyPodHasVolumeMounted(ctx, clientset, "default", "my-pod", "my-volume")
    if err != nil {
        t.Fatalf("Failed to verify Pod volume: %v", err)
    }
} 