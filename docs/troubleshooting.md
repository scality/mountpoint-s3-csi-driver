# Troubleshooting

This guide helps diagnose and resolve common issues with the Scality CSI Driver for S3.

## Quick Diagnostics

### 1. Check Driver Health

```bash
# Check driver pods status
kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver

# View driver logs
kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -c s3-plugin --tail=50
```

### 2. Check S3 Connectivity

```bash
# Test endpoint connectivity
curl -I https://your-s3-endpoint.com

# Test S3 access with AWS CLI
aws s3 ls s3://your-bucket --endpoint-url https://your-s3-endpoint.com
```

## Common Issues and Solutions

### Pod Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| Pod stuck in `ContainerCreating` | Mount operation failed | 1. Check driver logs<br/>2. Check S3 credentials<br/>3. Check mount options<br/>4. Ensure unique `volumeHandle` |
| Pod stuck in `Terminating` | Mount point busy or corrupted | 1. Force delete pod: `kubectl delete pod <name> --force`<br/>2. Check for `subPath` issues (see below) |
| Pod fails with "Permission denied" | Missing mount permissions | Add `allow-other` to PV `mountOptions` |
| Pod cannot write/delete files | Missing write permissions | Add `allow-delete` and/or `allow-overwrite` to PV `mountOptions` |

### Mount Issues

| Error Message | Cause | Solution |
|---------------|-------|----------|
| "Transport endpoint not connected" | S3 endpoint unreachable | 1. Check network connectivity<br/>2. Check endpoint URL configuration<br/>3. Check security groups/firewall rules |
| "Failed to create mount process" | Mountpoint binary issue | 1. Check initContainer logs<br/>2. Check `/opt/mountpoint-s3-csi/bin/mount-s3` exists on node |
| "Access Denied" | Invalid S3 credentials | 1. Check secret contains `access_key_id` and `secret_access_key`<br/>2. Test credentials with AWS CLI<br/>3. Check bucket policy |
| "InvalidBucketName" | Bucket name issue | 1. Check bucket exists<br/>2. Check bucket name format<br/>3. Ensure no typos |
| "AWS_ENDPOINT_URL environment variable must be set" | Missing endpoint configuration | Set `s3EndpointUrl` in Helm values or driver configuration |

### Volume Issues

| Issue | Description | Solution |
|-------|-------------|----------|
| Multiple volumes fail in same pod | Duplicate `volumeHandle` | Ensure each PV has unique `volumeHandle` value |
| `subPath` returns "No such file or directory" | Empty directory removed by Mountpoint | Use `prefix` mount option instead of `subPath` (see below) |
| Volume not mounting | Misconfigured PV/PVC | Check `storageClassName: ""` for static provisioning |

## Known Limitations and Workarounds

### SubPath Behavior

When using `subPath` with S3 volumes, deleting all files in the directory causes the directory itself to disappear, making the mount unusable.

**Instead of:**

```yaml
volumeMounts:
  - mountPath: "/data"
    subPath: some-prefix
    name: vol
```

**Use prefix mount option:**

```yaml
# In PersistentVolume
mountOptions:
  - prefix=some-prefix/
```

### Multiple Volumes in Same Pod

Each PersistentVolume must have a unique `volumeHandle`:

```yaml
# ❌ WRONG - Duplicate volumeHandle
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv-1
spec:
  csi:
    volumeHandle: s3-csi-driver-volume # Duplicate!
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv-2
spec:
  csi:
    volumeHandle: s3-csi-driver-volume # Duplicate!

# ✅ CORRECT - Unique volumeHandles
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv-1
spec:
  csi:
    volumeHandle: s3-csi-driver-volume-1 # Unique
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv-2
spec:
  csi:
    volumeHandle: s3-csi-driver-volume-2 # Unique
```

## Uninstallation Issues

### Namespace Stuck Terminating

```bash
# Check blocking conditions
kubectl get namespace ${NAMESPACE} -o json | jq '.status.conditions'

# Force remove finalizers (use with caution)
kubectl get namespace ${NAMESPACE} -o json | \
  jq '.spec = {"finalizers":[]}' | \
  kubectl replace --raw /api/v1/namespaces/${NAMESPACE}/finalize -f -
```

### PersistentVolumes Stuck Terminating

```bash
# Check PV status
kubectl describe pv <pv-name>

# Remove finalizers if needed
kubectl patch pv <pv-name> -p '{"metadata":{"finalizers":null}}'
```

### Orphaned Helm Release

```bash
# List all releases
helm list --all-namespaces

# Manual cleanup if release is orphaned
kubectl delete all -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver --all-namespaces
kubectl delete sa,clusterrole,clusterrolebinding -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver --all-namespaces
```

## Debug Mode

Enable debug logging for detailed diagnostics:

```yaml
# In PersistentVolume
spec:
  mountOptions:
    - debug
    - debug-crt  # For AWS CRT client logs
```

View debug logs:

```bash
# On the node
journalctl -u mount-s3-* -f
```

## Performance Troubleshooting

| Symptom | Possible Cause | Action |
|---------|----------------|--------|
| Slow file operations | High S3 latency | 1. Check network latency to S3<br/>2. Enable caching with `cache` mount option<br/>3. Consider using closer S3 region |
| High memory usage | Large cache size | Limit cache with `max-cache-size` mount option |
| Slow directory listings | No metadata caching | Add `metadata-ttl` mount option (e.g., `metadata-ttl=60`) |

## Getting Help

If issues persist after following this guide:

1. Collect diagnostic information:

    ```bash
    # CSI driver logs (all containers)
    kubectl logs -n kube-system -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver --all-containers=true > csi-driver-logs.txt

    # Node plugin logs specifically
    kubectl logs -n kube-system -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -c s3-plugin > node-plugin-logs.txt

    # CSI node driver registrar logs
    kubectl logs -n kube-system -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -c node-driver-registrar > registrar-logs.txt

    # Your pod description and events
    kubectl describe pod <your-pod> > pod-description.txt

    # PV and PVC details
    kubectl describe pv <your-pv> > pv-description.txt
    kubectl describe pvc <your-pvc> > pvc-description.txt

    # S3 bucket configuration (if accessible)
    aws s3api get-bucket-location --bucket <bucket-name> --endpoint-url <endpoint> > bucket-location.txt
    aws s3api get-bucket-versioning --bucket <bucket-name> --endpoint-url <endpoint> > bucket-versioning.txt
    aws s3api get-bucket-policy --bucket <bucket-name> --endpoint-url <endpoint> > bucket-policy.txt 2>&1
    aws s3api list-objects-v2 --bucket <bucket-name> --max-items 10 --endpoint-url <endpoint> > bucket-list-sample.txt
    ```

2. Contact [Scality Support](https://support.scality.com/) with, Driver version, Kubernetes version, Error messages and all collected information in Step 1, PV/PVC/Pod YAML manifests (sanitized)
