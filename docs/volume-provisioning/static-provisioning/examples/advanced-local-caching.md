# Advanced Local Caching

This example demonstrates how to enable local disk caching with advanced options for improved S3 performance.

## Features

- Local disk caching at `/tmp/s3-pv1-cache`
- Configurable cache size (500 MB)
- Metadata TTL presets: `minimal` and `indefinite`
- Custom metadata TTL values in seconds (3 seconds in this example)

## Deploy

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv
spec:
  capacity:
    storage: 1200Gi # ignored, required
  accessModes:
    - ReadWriteMany # supported options: ReadWriteMany
  storageClassName: "" # Required for static provisioning
  claimRef: # To ensure no other PVCs can claim this PV
    namespace: default # Namespace is required even though it's in "default" namespace.
    name: s3-pvc # Name of your PVC
  mountOptions:
    - cache /tmp/s3-pv1-cache # specify cache directory, relative to root host filesystem
                              # the directory must be unique per mount on a host
    - metadata-ttl 3 # 3 second time to live
    - max-cache-size 500 # 500 MB maximum size
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-caching-volume # Must be unique across all PVs
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
      args: ["-c", "echo 'Hello from the container!' >> /data/$(date -u).txt; tail -f /dev/null"]
      volumeMounts:
        - name: persistent-storage
          mountPath: /data
  volumes:
    - name: persistent-storage
      persistentVolumeClaim:
        claimName: s3-pvc
EOF
```

## Key Mount Options

- `cache /tmp/s3-pv1-cache` - Cache directory path (must be unique per volume)
- `metadata-ttl 3` - Cache metadata for 3 seconds
- `max-cache-size 500` - Limit cache to 500 MB

## Metadata TTL Configuration

The `metadata-ttl` flag controls how long Mountpoint considers its file system metadata (file existence, size, object etag, etc) accurate before re-fetching from S3.
Mountpoint will typically perform fewer requests to the mounted S3 bucket, but will not guarantee that the information it reports is up to date with the content of the mounted S3 bucket.

With local caching enabled, the stored data is considered accurate until the metadata TTL expires.
After this period, Mountpoint revalidates if the cached data is still accurate by verifying the object's etag hasn't changed.

### TTL Presets

Mountpoint provides two presets which trade off consistency and performance/cost optimization:

- **`metadata-ttl minimal`** - For scenarios where the mounted S3 bucket content is modified by another client and you require recently up-to-date information
- **`metadata-ttl indefinite`** - For workloads that don't require consistency, such as when the S3 bucket content doesn't change

### Custom TTL Values

You can also specify custom TTL values in seconds:

- `metadata-ttl 300` - Allows Mountpoint to delay updates for up to 300 seconds, reducing S3 requests
- `metadata-ttl 3` - Short TTL for more frequent validation (as shown in this example)

## Important Notes

⚠️ **Cache Path Uniqueness**: Each volume must use a unique cache path on each node to avoid conflicts.

## Check Pod-Level Access to the Mounted S3 Volume

```bash
kubectl get pod s3-app
# Check cache directory on the node
kubectl exec s3-app -- ls -la /data
# Check cache directory on the node as specified in the mountoption.
# if copied from the example, it should be /tmp/s3-pv1-cache
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 static_provisioning_with_advanced_local_caching.yaml](assets/advanced_local_caching.yaml)
