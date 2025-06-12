# Shared Cache Configuration

This example demonstrates basic shared cache configuration for improved S3 performance.

## Features

- Shared cache directory at `/tmp/s3-csi-driver-cache`
- Improved read performance through local caching
- Simple cache setup without size limits

## Deploy

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv
spec:
  capacity:
    storage: 1200Gi # Ignored, required
  accessModes:
    - ReadWriteMany # Supported options: ReadWriteMany
  storageClassName: "" # Required for static provisioning
  claimRef: # To ensure no other PVCs can claim this PV
    namespace: default # Namespace is required even though it's in "default" namespace.
    name: s3-pvc # Name of your PVC
  mountOptions:
    - allow-delete
    - region us-west-2
    - cache /tmp/s3-csi-driver-cache # Local cache option for improved performance. More information: https://github.com/awslabs/mountpoint-s3/blob/main/doc/CONFIGURATION.md#caching-configuration
  csi:
    driver: s3.csi.scality.com # Required
    volumeHandle: s3-csi-shared-cache-volume # Must be unique across all PVs
    volumeAttributes:
      bucketName: s3-csi-driver
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-pvc
spec:
  accessModes:
    - ReadWriteMany # Supported options: ReadWriteMany
  storageClassName: "" # Required for static provisioning
  resources:
    requests:
      storage: 1200Gi # Ignored, required
  volumeName: s3-pv # Name of your PV
---
apiVersion: v1
kind: Pod
metadata:
  name: s3-app
spec:
  containers:
    - name: app
      image: ubuntu
      command: ["/bin/sh"]
      args: ["-c", "echo 'Hello from the container!' >> /data/$(date -u).txt; echo 'Shared cache test' >> /data/shared_cache_test.txt; cat /data/shared_cache_test.txt; tail -f /dev/null"]
      volumeMounts:
        - name: persistent-storage
          mountPath: /data
  volumes:
    - name: persistent-storage
      persistentVolumeClaim:
        claimName: s3-pvc
EOF
```

## Key Mount Option

- `cache /tmp/s3-csi-driver-cache` - Enables shared caching at specified directory

## Benefits

- Faster read access for frequently accessed files
- Reduced S3 API calls
- Improved application performance

## Important Notes

⚠️ **Cache Directory**: Ensure the cache directory has sufficient disk space and proper permissions.

## Check Pod-Level Access to the Mounted S3 Volume

```bash
kubectl get pod s3-app
kubectl exec s3-app -- cat /data/shared_cache_test.txt
# Check cache directory on the node
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 static_provisioning_with_shared_cache.yaml](assets/static_provisioning_with_shared_cache.yaml)
