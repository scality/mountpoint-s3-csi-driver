# Local Cache Configuration

This example demonstrates basic local cache configuration for improved S3 performance.

## Features

- Local cache directory at `/tmp/s3-csi-driver-cache`
- Improved read performance through local caching
- Automatic cache size management with 5% free space protection
- Default metadata TTL of 1 minute (60 seconds).

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
    - cache /tmp/s3-csi-driver-cache # Local cache option for improved performance
  csi:
    driver: s3.csi.scality.com # Required
    volumeHandle: s3-csi-local-cache-volume # Must be unique across all PVs
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
      args: ["-c", "echo 'Hello from the container!' >> /data/$(date -u).txt; echo 'Local cache test' >> /data/local_cache_test.txt; cat /data/local_cache_test.txt; tail -f /dev/null"]
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

- `cache /tmp/s3-csi-driver-cache` - Enables local caching at specified directory

## Cache Behavior

By default, Mountpoint will limit the maximum size of the local cache such that the free space on the file system does not fall below 5%,
and will automatically evict the least recently used content from the local cache when caching new content.

Configuring a local cache will also enable caching of metadata in memory using a default time-to-live (TTL) of 1 minute (60 seconds), which can be configured with the `--metadata-ttl` argument.
For detailed metadata TTL configuration options, see [Advanced Local Caching](advanced-local-caching.md).

## Benefits

- Faster read access for frequently accessed files
- Reduced S3 API calls
- Improved application performance
- Automatic cache management with LRU eviction

## Important Notes

⚠️ **Cache Directory**: Ensure the cache directory has sufficient disk space and proper permissions.

## Check Pod-Level Access to the Mounted S3 Volume

```bash
kubectl get pod s3-app
kubectl exec s3-app -- cat /data/local_cache_test.txt
# Check cache directory on the node as specified in the mountoption.
# if copied from the example, it should be /tmp/s3-csi-driver-cache
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 static_provisioning_with_local_cache.yaml](assets/local_cache.yaml)
