# Uninstalling the S3 CSI Driver

## Overview

This document describes how to properly uninstall the Scality S3 CSI Driver from your Kubernetes cluster.

## Basic Uninstall

To uninstall the S3 CSI Driver using Helm:

```bash
helm uninstall s3-csi-driver -n kube-system
```

## CRD Cleanup

By default, the uninstall process preserves Custom Resource Definitions (CRDs) and any associated resources to prevent accidental data loss. This means:

- `MountpointS3PodAttachment` CRDs remain in the cluster
- Active Mountpoint Pods continue running
- Existing S3 mounts remain accessible

### Manual CRD Cleanup

If you want to completely remove all CRDs and resources after uninstalling:

```bash
# Delete all MountpointS3PodAttachment CRDs
kubectl delete mountpoints3podattachments.s3.csi.scality.com --all

# Delete all Mountpoint Pods
kubectl delete pods -n mount-s3 -l app=mountpoint-s3

# Delete the CRD definition itself
kubectl delete crd mountpoints3podattachments.s3.csi.scality.com
```

### Automatic CRD Cleanup

You can enable automatic cleanup during helm uninstall by setting the `cleanupCRDOnUninstall` flag:

```bash
helm uninstall s3-csi-driver -n kube-system --set cleanupCRDOnUninstall=true
```

Or update your values file:

```yaml
cleanupCRDOnUninstall: true
```

**Warning**: Enabling automatic cleanup will:

- Forcefully terminate all active S3 mounts
- Delete all Mountpoint Pods immediately
- Remove all MountpointS3PodAttachment CRDs
- Potentially disrupt running applications using S3 volumes

## Pre-Uninstall Checklist

Before uninstalling the CSI driver:

1. **Check for active mounts**:

   ```bash
   kubectl get mountpoints3podattachments -A
   kubectl get pods -n mount-s3 -l app=mountpoint-s3
   ```

2. **Verify no PersistentVolumes are in use**:

   ```bash
   kubectl get pv -o json | jq '.items[] | select(.spec.csi.driver=="s3.csi.scality.com") | .metadata.name'
   ```

3. **Check for PersistentVolumeClaims**:

   ```bash
   kubectl get pvc -A -o json | jq '.items[] | select(.spec.storageClassName | contains("s3")) | {namespace: .metadata.namespace, name: .metadata.name}'
   ```

4. **Identify workloads using S3 volumes**:

   ```bash
   kubectl get pods -A -o json | jq '.items[] | select(.spec.volumes[]? | select(.persistentVolumeClaim)) | {namespace: .metadata.namespace, name: .metadata.name}'
   ```

## Graceful Migration

For production environments, follow this graceful migration process:

1. **Stop workloads using S3 volumes**:

   ```bash
   kubectl scale deployment <deployment-name> --replicas=0
   ```

2. **Delete PersistentVolumeClaims**:

   ```bash
   kubectl delete pvc <pvc-name> -n <namespace>
   ```

3. **Delete PersistentVolumes** (if using static provisioning):

   ```bash
   kubectl delete pv <pv-name>
   ```

4. **Uninstall the CSI driver**:

   ```bash
   helm uninstall s3-csi-driver -n kube-system
   ```

5. **Clean up remaining resources** (if needed):

   ```bash
   kubectl delete mountpoints3podattachments.s3.csi.scality.com --all
   kubectl delete pods -n mount-s3 -l app=mountpoint-s3
   ```

## Troubleshooting

### Stuck Resources

If resources are stuck in terminating state:

1. Check for finalizers:

   ```bash
   kubectl get mountpoints3podattachments -o json | jq '.items[] | {name: .metadata.name, finalizers: .metadata.finalizers}'
   ```

2. Remove finalizers if necessary:

   ```bash
   kubectl patch mountpoints3podattachment <name> -p '{"metadata":{"finalizers":null}}' --type=merge
   ```

### Orphaned Mounts

Check for orphaned mount points on nodes:

```bash
# On each node
mount | grep s3.csi.scality.com
ls -la /var/lib/kubelet/plugins/s3.csi.scality.com/mnt/
```

Clean up orphaned mounts:

```bash
# Unmount orphaned bind mounts
umount /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/*/mount

# Clean up source directories
rm -rf /var/lib/kubelet/plugins/s3.csi.scality.com/mnt/mp-*
```

## Post-Uninstall Verification

After uninstalling, verify complete removal:

```bash
# Check for remaining pods
kubectl get pods -A | grep -E "s3-csi|mount-s3|mountpoint"

# Check for remaining CRDs
kubectl get crd | grep s3.csi.scality.com

# Check for remaining services/deployments
kubectl get all -A | grep -E "s3-csi|mount-s3"

# Check for remaining RBAC resources
kubectl get clusterroles,clusterrolebindings | grep s3-csi
```

## Reinstallation

If you plan to reinstall the CSI driver:

1. Ensure all resources are completely removed
2. Wait for any terminating pods to complete
3. Reinstall using Helm:

   ```bash
   helm install s3-csi-driver ./charts/scality-mountpoint-s3-csi-driver -n kube-system
   ```
