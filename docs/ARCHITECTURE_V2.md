# Architecture v2: Controller-Based Pod Creation

## Overview

Version 2.0 of the Scality CSI Driver introduces a controller-based architecture for managing Mountpoint Pods. This design provides better lifecycle management, SELinux support, and resource optimization.

## Key Components

### 1. Node Component (DaemonSet)
The node component runs on each Kubernetes node and is responsible for:
- Receiving CSI mount requests from kubelet
- Creating MountpointS3PodAttachment Custom Resources (CRDs)
- Waiting for Mountpoint Pods to be created by the controller
- Managing the FUSE mount connection

### 2. Controller Component (Deployment)
The controller component runs as a single replica deployment and is responsible for:
- Watching MountpointS3PodAttachment CRDs
- Creating and managing Mountpoint Pods based on CRDs
- Looking up workload pods and persistent volumes
- Managing pod lifecycle (deleting succeeded pods, handling failures)

### 3. MountpointS3PodAttachment CRD
This custom resource represents a request to mount an S3 bucket for a workload pod:
```yaml
apiVersion: s3.csi.scality.com/v2
kind: MountpointS3PodAttachment
metadata:
  name: <node-name>-<volume-id>
spec:
  nodeName: worker-node-1
  persistentVolumeName: pv-123
  volumeID: vol-456
  mountOptions: "--cache /tmp/cache"
  mountpointS3PodAttachments:
    mp-pod-name:
      - workloadPodUID: abc-123
        attachmentTime: 2024-01-01T00:00:00Z
```

## Mount Flow

1. **Volume Mount Request**: When a pod requests an S3 volume, kubelet calls the CSI driver's NodePublishVolume
2. **CRD Creation**: The node component creates a MountpointS3PodAttachment CRD with mount details
3. **Controller Reconciliation**: The controller watches for new CRDs and:
   - Looks up the workload pod by UID
   - Retrieves the PersistentVolume specification
   - Creates a Mountpoint Pod using the Creator class
4. **Pod Creation**: The Mountpoint Pod is created with:
   - Proper labels for tracking
   - Owner reference to the CRD for cleanup
   - Resource requirements from workload pod
   - Node affinity to ensure co-location
5. **Mount Establishment**: The node component waits for the pod and establishes the FUSE mount
6. **Cleanup**: When unmounting, the node component updates the CRD, and the controller handles pod deletion

## Benefits of Controller-Based Architecture

### Separation of Concerns
- Node component focuses on CSI operations and FUSE mounting
- Controller handles Kubernetes resource management
- Clear ownership and lifecycle management through CRDs

### Better Error Handling
- Controller can retry pod creation with backoff
- Failed pods are properly tracked and can be recreated
- CRDs provide audit trail of mount requests

### Resource Optimization
- Controller has cluster-wide view for resource decisions
- Support for priority classes and preemption
- Headroom management for efficient scheduling

### SELinux Support
- Pod-based mounting provides proper SELinux context
- Each mount runs in its own security context
- Compatible with restricted environments

## Migration from v1

### Automatic Migration
- Existing systemd mounts continue to work
- On pod restart, mounts automatically transition to pod mounter
- No manual intervention required

### Backward Compatibility
- Node component can operate without controller (degraded mode)
- Falls back to minimal pod specs when workload pod not found
- Existing configurations continue to work

## Configuration

### Controller Environment Variables
- `MOUNTPOINT_NAMESPACE`: Namespace for Mountpoint Pods
- `MOUNTPOINT_VERSION`: Version of Mountpoint binary
- `MOUNTPOINT_PRIORITY_CLASS_NAME`: Default priority class
- `MOUNTPOINT_PREEMPTING_PRIORITY_CLASS_NAME`: Priority class for preemption
- `MOUNTPOINT_HEADROOM_PRIORITY_CLASS_NAME`: Priority class for headroom pods
- `MOUNTPOINT_IMAGE`: Container image for Mountpoint Pods
- `MOUNTPOINT_HEADROOM_IMAGE`: Pause container image for headroom pods

### Node Environment Variables
- `NODE_NAME`: Name of the Kubernetes node (required for CRD creation)
- `KUBELET_PATH`: Path to kubelet directory
- All existing CSI driver environment variables

## RBAC Requirements

### Controller Permissions
```yaml
- apiGroups: ["s3.csi.scality.com"]
  resources: ["mountpoints3podattachments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["persistentvolumes"]
  verbs: ["get", "list", "watch"]
```

### Node Permissions
```yaml
- apiGroups: ["s3.csi.scality.com"]
  resources: ["mountpoints3podattachments"]
  verbs: ["create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
```

## Monitoring and Debugging

### Key Metrics
- Number of active MountpointS3PodAttachments
- Mountpoint Pod creation latency
- Failed pod creation attempts
- Resource utilization of Mountpoint Pods

### Debugging Commands
```bash
# List all pod attachments
kubectl get mountpoints3podattachments -A

# Check controller logs
kubectl logs -n kube-system deployment/s3-csi-controller

# Check node logs
kubectl logs -n kube-system daemonset/s3-csi-node

# Check Mountpoint Pod status
kubectl get pods -n mount-s3
```

## Future Enhancements

- Support for pod attachment sharing across multiple workloads
- Advanced scheduling hints for Mountpoint Pods
- Metrics and observability improvements
- Support for volume snapshots