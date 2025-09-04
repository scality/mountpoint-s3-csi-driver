# V2 Pod Sharing Behavior

## Overview

In the v2 architecture of the Scality CSI Driver for S3, the controller implements intelligent pod sharing for Mountpoint Pods to optimize resource usage and reduce overhead when multiple workloads access the same volume on the same node.

## Pod Sharing Logic

### When Pods Are Shared

When a new workload is scheduled on a node that already has a `MountpointS3PodAttachment` with existing Mountpoint Pods, the controller tries to assign the workload to an existing pod if all the following conditions are met:

- **Pod is available**: The pod is not annotated with `s3.csi.scality.com/needs-unmount` (being terminated)
- **Pod accepts workloads**: The pod is not annotated with `s3.csi.scality.com/no-new-workload` (explicitly forbidden)
- **Version compatibility**: The pod was created by the same CSI driver version
- **Pod exists**: The pod still exists in the cluster (not deleted)

### When New Pods Are Created

A new Mountpoint Pod is created only when no suitable existing pod is found that meets all the sharing criteria above.

### Key Constraints

- **Same node requirement**: Pod sharing only happens within the same node - workloads on different nodes cannot share Mountpoint Pods
- **Same PV requirement**: Pod sharing is scoped to the same Persistent Volume - the `MountpointS3PodAttachment` is indexed by node name and PV name
- **Automatic cleanup**: When all workloads using a shared Mountpoint Pod terminate, the pod is annotated for unmount and eventually cleaned up

## Implementation Details

### MountpointS3PodAttachment Structure

The `MountpointS3PodAttachment` custom resource tracks the relationship between Mountpoint Pods and workload pods:

```yaml
apiVersion: s3.csi.scality.com/v2
kind: MountpointS3PodAttachment
spec:
  nodeName: node-1
  persistentVolumeName: pv-123
  volumeID: bucket-name
  mountpointS3PodAttachments:
    mp-pod-abc123:  # Mountpoint Pod name
      - workloadPodUID: workload-pod-uid-1
        attachmentTime: "2025-01-01T00:00:00Z"
      - workloadPodUID: workload-pod-uid-2
        attachmentTime: "2025-01-01T00:01:00Z"
```

### Pod Lifecycle

1. **Creation**: First workload triggers Mountpoint Pod creation
2. **Sharing**: Additional workloads on same node/PV are assigned to existing pod
3. **Cleanup**: When last workload terminates, pod is annotated with `needs-unmount`
4. **Deletion**: Pod transitions to `Succeeded` state and is deleted by the controller

### Annotations

The controller uses these annotations to control pod sharing:

- `s3.csi.scality.com/needs-unmount`: Marks pod for cleanup when set to "true"
- `s3.csi.scality.com/no-new-workload`: Prevents new workloads from being assigned when set to "true"

## Benefits

- **Resource Efficiency**: Reduces number of pods needed for multiple workloads accessing same volume
- **Faster Mount Times**: Subsequent workloads can reuse already-mounted volumes
- **Simplified Management**: Fewer pods to monitor and manage

## Comparison with V1

| Aspect | V1 Behavior | V2 Behavior |
|--------|-------------|-------------|
| Pod Creation | One pod per workload | Shared pods when possible |
| Resource Usage | Higher pod count | Optimized pod count |
| Mount Operations | Mount per workload | Single mount, multiple users |
| Complexity | Simple 1:1 mapping | Intelligent sharing logic |