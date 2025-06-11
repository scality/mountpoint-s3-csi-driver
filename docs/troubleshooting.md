# Troubleshooting

This guide helps diagnose and resolve common issues with the Scality S3 CSI Driver.

## Common Issues

| Symptom | Probable Cause | Fix |
|---------|----------------|-----|
| Pod stuck in `ContainerCreating` | Mount operation failed | Check driver logs: `kubectl logs -n kube-system <driver-pod>` |
| "Permission denied" accessing files | Missing `allow-other` mount option | Add `allow-other` to PV mountOptions |
| Cannot delete files | Missing `allow-delete` mount option | Add `allow-delete` to PV mountOptions |
| Mount fails with "Transport endpoint not connected" | S3 endpoint unreachable | Verify network connectivity to S3 endpoint |

## Diagnostic Commands

### Check Driver Status

```bash
kubectl get pods -n kube-system -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
kubectl logs -n kube-system <driver-pod-name> -c s3-plugin
```

### Check Mount Status (on node)

```bash
systemctl list-units --all | grep mount-s3
journalctl -u <mount-unit-name> -f
```

### Verify S3 Connectivity

```bash
curl -I https://s3.example.com
aws s3 ls s3://bucket-name --endpoint-url https://s3.example.com
```

## Common Error Messages

### "Failed to create mount process"

- **Cause**: Mountpoint binary not found or not executable
- **Solution**: Check initContainer logs, ensure `/opt/mountpoint-s3-csi/bin/mount-s3` exists

### "Access Denied"

- **Cause**: Invalid S3 credentials or insufficient permissions
- **Solution**: Verify secret, test credentials with AWS CLI, check bucket policy

### "InvalidBucketName"

- **Cause**: Bucket name doesn't meet S3 requirements
- **Solution**: Verify bucket name, ensure bucket exists, check for typos

!!! tip
    Enable debug logging before reproducing issues to capture detailed diagnostic information.


25-06-11T19:46:19.797074429Z F0611 19:46:19.796880       1 main.go:74] failed to create driver: AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function
2025-06-11T19:46:19.797097637Z F0611 19:46:19.796880       1 main.go:74] failed to create driver: AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function
2025-06-11T19:46:19.797099387Z F0611 19:46:19.796880       1 main.go:74] failed to create driver: AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function
2025-06-11T19:46:19.797100554Z F0611 19:46:19.796880       1 main.go:74] failed to create driver: AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function
2025-06-11T19:46:19.797101512Z F0611 19:46:19.796880       1 main.go:74] failed to create driver: AWS_ENDPOINT_URL environment variable must be set for the CSI driver to function


## Troubleshooting Uninstallation

### Issue: Namespace Stuck in Terminating State

If the namespace is stuck deleting:

```bash
# Check what's blocking deletion
kubectl get namespace ${NAMESPACE} -o json | jq '.status.conditions'

# Force remove finalizers if needed (use with caution)
kubectl get namespace ${NAMESPACE} -o json | jq '.spec = {"finalizers":[]}' | kubectl replace --raw /api/v1/namespaces/${NAMESPACE}/finalize -f -
```

### Issue: PVs Stuck in Terminating State

If PersistentVolumes won't delete:

```bash
# Check PV status
kubectl describe pv <pv-name>

# Remove finalizers if needed
kubectl patch pv <pv-name> -p '{"metadata":{"finalizers":null}}'
```

### Issue: Helm Release Not Found

If Helm can't find the release:

```bash
# Check all namespaces
helm list --all-namespaces

# If release is orphaned, manually clean up resources
kubectl delete all -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver --all-namespaces
kubectl delete sa,clusterrole,clusterrolebinding -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver --all-namespaces
```


## Troubleshooting

### Common Issues

1. **Driver pods not starting**:
   - Check node plugin logs: `kubectl logs -n ${NAMESPACE} <node-pod-name> -c s3-plugin`
   - Verify credentials secret exists and contains correct keys
   - Ensure S3 endpoint is reachable from nodes

2. **CSI driver not registered**:
   - Check kubelet logs on the nodes
   - Verify the driver pods are running on all expected nodes
   - Check for RBAC permission issues

3. **Performance issues**:
   - Consider adjusting resource limits in values file
   - Check network latency to S3 endpoint
   - Monitor driver pod resource usage
