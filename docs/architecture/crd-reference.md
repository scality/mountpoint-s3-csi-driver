# Custom Resource Definition Reference

This document provides a reference for the Custom Resource Definitions (CRDs) used by the Scality CSI Driver for S3.

## MountpointS3PodAttachment

The `MountpointS3PodAttachment` CRD tracks volume attachments between workload pods and Mountpoint Pods. It enables volume sharing and coordinates the lifecycle of Mountpoint Pods.

### Resource Information

| Property | Value |
|----------|-------|
| **API Group** | `s3.csi.scality.com` |
| **API Version** | `v2` |
| **Kind** | `MountpointS3PodAttachment` |
| **Scope** | Cluster |
| **Short Name** | `s3pa` |

### Spec Fields

| Field | Type | Description |
|-------|------|-------------|
| `nodeName` | string | Name of the Kubernetes node where the volume is mounted |
| `persistentVolumeName` | string | Name of the PersistentVolume being mounted |
| `volumeID` | string | CSI volume identifier (matches S3 bucket name for dynamic provisioning) |
| `mountOptions` | string | Comma-separated mount options from the PV or StorageClass |
| `workloadFSGroup` | string | Pod security context fsGroup value (empty string if not set) |
| `mountpointS3PodAttachments` | map | Maps Mountpoint Pod names to their workload attachments |

### WorkloadAttachment Structure

Each entry in `mountpointS3PodAttachments` contains a list of `WorkloadAttachment` objects:

| Field | Type | Description |
|-------|------|-------------|
| `workloadPodUID` | string | Unique identifier (UID) of the attached workload pod |
| `attachmentTime` | timestamp | When the workload pod was attached to the Mountpoint Pod |

### Selectable Fields

The CRD supports field selectors for efficient querying:

```bash
# Find attachments on a specific node
kubectl get s3pa --field-selector spec.nodeName=worker-1

# Find attachments for a specific PV
kubectl get s3pa --field-selector spec.persistentVolumeName=my-s3-pv

# Find attachments with specific fsGroup
kubectl get s3pa --field-selector spec.workloadFSGroup=1000
```

### Print Columns

When listing resources, the following columns are displayed:

| Column | JSON Path | Description |
|--------|-----------|-------------|
| Node | `.spec.nodeName` | The node where the volume is mounted |
| PV Name | `.spec.persistentVolumeName` | The persistent volume name |
| Mount Options | `.spec.mountOptions` | Comma-separated mount options |
| Age | `.metadata.creationTimestamp` | Resource age |

### Example Resource

```yaml
apiVersion: s3.csi.scality.com/v2
kind: MountpointS3PodAttachment
metadata:
  name: s3pa-worker1-my-s3-pv-abc123
spec:
  nodeName: worker-1
  persistentVolumeName: my-s3-pv
  volumeID: csi-s3-12345678-abcd-1234-abcd-123456789012
  mountOptions: "--read-only,--cache /tmp/cache"
  workloadFSGroup: "1000"
  mountpointS3PodAttachments:
    mp-pod-xyz789:
      - workloadPodUID: "abc-123-def-456"
        attachmentTime: "2024-01-15T10:30:00Z"
      - workloadPodUID: "ghi-789-jkl-012"
        attachmentTime: "2024-01-15T10:35:00Z"
```

### Lifecycle

#### Creation

The Pod Reconciler creates a MountpointS3PodAttachment when:

1. A workload pod is scheduled and needs an S3 volume
2. No existing CRD matches the volume configuration on that node
3. The Mountpoint Pod is created first, then the CRD is created with the assignment

#### Updates

The Pod Reconciler updates the CRD when:

1. Additional workloads with matching configuration need the same volume
2. Workloads terminate and need to be removed from the attachment list

#### Deletion

The Pod Reconciler deletes the CRD when:

1. No workload attachments remain after all workloads terminate
2. The associated Mountpoint Pod has been marked for deletion

### Volume Sharing Logic

Two workloads can share a Mountpoint Pod when their CRD entries match on all of:

- `nodeName` - Same Kubernetes node
- `persistentVolumeName` - Same PersistentVolume
- `volumeID` - Same underlying volume
- `mountOptions` - Same mount configuration
- `workloadFSGroup` - Same security context

When these match, the Pod Reconciler adds both workloads to the same Mountpoint Pod's attachment list rather than creating a new pod.

### Troubleshooting

#### List All Attachments

```bash
kubectl get s3pa
```

#### Describe a Specific Attachment

```bash
kubectl describe s3pa <name>
```

#### Check Attachments on a Node

```bash
kubectl get s3pa --field-selector spec.nodeName=<node-name>
```

#### View Detailed YAML

```bash
kubectl get s3pa <name> -o yaml
```

#### List Mountpoint Pods

Mountpoint Pods are created in the `mount-s3` namespace (configurable):

```bash
kubectl get pods -n mount-s3
```

#### Common Issues

| Symptom | Possible Cause | Resolution |
|---------|----------------|------------|
| CRD stuck with no Mountpoint Pod | Pod Reconciler not running | Check controller deployment |
| Stale CRD after workload deletion | Cleanup delay (up to 2 minutes) | Wait for background cleanup |
| Multiple CRDs for same volume | Different mount options or fsGroup | Expected behavior |
