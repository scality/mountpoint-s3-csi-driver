# Local Caching

This example demonstrates how to enable local disk caching for improved S3 performance.

## Features

- Local disk caching at `/tmp/s3-pv1-cache`
- Configurable cache size (500 MB)
- Metadata TTL of 3 seconds

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
    - ReadWriteMany # supported options: ReadWriteMany / ReadOnlyMany
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

## Important Notes

⚠️ **Cache Path Uniqueness**: Each volume must use a unique cache path on each node to avoid conflicts.

## Verify

```bash
kubectl get pod s3-app
# Check cache directory on the node
kubectl exec s3-app -- ls -la /data
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 caching.yaml](assets/caching.yaml)
