# Shared Bucket Dynamic Provisioning

This example demonstrates dynamic provisioning using a shared bucket where multiple PVCs get unique prefixes within the same bucket.

## Features

- **Shared bucket**: Multiple PVCs use the same pre-existing S3 bucket
- **Unique prefixes**: Each PVC gets its own prefix within the shared bucket
- **Cost effective**: Reduces the number of buckets needed
- **Multiple applications**: Shows how different applications can share storage infrastructure

## Prerequisites

The shared bucket must be created manually before using this StorageClass:

```bash
# Create the shared bucket (using AWS CLI as example)
aws s3 mb s3://my-shared-s3-bucket --region us-west-2 --endpoint-url https://s3.your-scality.com
```

## YAML Configuration

```yaml
# StorageClass for shared bucket dynamic provisioning
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-shared
provisioner: s3.csi.scality.com
parameters:
  # Shared bucket configuration
  bucketNaming: shared
  bucketPrefix: my-shared-s3-bucket  # Pre-existing bucket name
  s3Region: us-west-2
volumeBindingMode: Immediate
reclaimPolicy: Delete
---
# First PVC - will get prefix "volumes/pvc-uuid-1/"
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app1-storage
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi-shared
  resources:
    requests:
      storage: 50Gi
---
# Second PVC - will get prefix "volumes/pvc-uuid-2/"
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app2-storage
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi-shared
  resources:
    requests:
      storage: 75Gi
---
# First application using its own prefix
apiVersion: v1
kind: Pod
metadata:
  name: app1
spec:
  containers:
    - name: app1
      image: ubuntu
      command: ["/bin/sh"]
      args: ["-c", "echo 'Data from App 1' > /data/app1-data.txt; ls -la /data/; tail -f /dev/null"]
      volumeMounts:
        - name: storage
          mountPath: /data
  volumes:
    - name: storage
      persistentVolumeClaim:
        claimName: app1-storage
---
# Second application using its own prefix
apiVersion: v1
kind: Pod
metadata:
  name: app2
spec:
  containers:
    - name: app2
      image: ubuntu
      command: ["/bin/sh"]
      args: ["-c", "echo 'Data from App 2' > /data/app2-data.txt; ls -la /data/; tail -f /dev/null"]
      volumeMounts:
        - name: storage
          mountPath: /data
  volumes:
    - name: storage
      persistentVolumeClaim:
        claimName: app2-storage
```

## Deployment Steps

1. **Create the shared bucket** (if not already created):
   ```bash
   # Using AWS CLI with custom endpoint
   aws s3 mb s3://my-shared-s3-bucket --region us-west-2 --endpoint-url https://s3.your-scality.com
   ```

2. **Apply the configuration**:
   ```bash
   kubectl apply -f shared-bucket-provisioning.yaml
   ```

3. **Verify the StorageClass**:
   ```bash
   kubectl get storageclass s3-csi-shared
   ```

4. **Check PVC status**:
   ```bash
   kubectl get pvc
   # Both PVCs should show "Bound" status
   ```

5. **Verify the pods**:
   ```bash
   kubectl get pods
   kubectl logs app1
   kubectl logs app2
   ```

6. **Check bucket contents**: Each application's data will be stored under different prefixes:
   - App1 data: `volumes/app1-storage-{uuid}/app1-data.txt`
   - App2 data: `volumes/app2-storage-{uuid}/app2-data.txt`

## Advanced Configuration with Mount Options

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-shared-advanced
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: shared
  bucketPrefix: my-shared-s3-bucket
  s3Region: us-west-2
  # Custom mount options applied to all volumes
  mountOptions: |
    --allow-delete
    --cache /tmp/s3-cache
    --metadata-ttl 300
volumeBindingMode: Immediate
reclaimPolicy: Delete
mountOptions:
  - allow-delete
  - cache /tmp/shared-cache
  - metadata-ttl 300
```

## Directory Structure in Shared Bucket

With shared bucket provisioning, your S3 bucket will have a structure like:

```text
my-shared-s3-bucket/
├── volumes/
│   ├── app1-storage-abc123/
│   │   └── app1-data.txt
│   ├── app2-storage-def456/
│   │   └── app2-data.txt
│   └── other-app-xyz789/
│       └── other-data.txt
└── other-files-not-managed-by-csi/
```

## Cleanup

```bash
# Delete the applications and PVCs
kubectl delete pod app1 app2
kubectl delete pvc app1-storage app2-storage

# The bucket remains, but the volume-specific prefixes are cleaned up
# The shared bucket itself is NOT deleted
```

## Use Cases

### Shared bucket provisioning is ideal for

- **Multi-tenant environments**: Where different applications need isolated storage within shared infrastructure
- **Cost optimization**: Reducing the number of S3 buckets while maintaining isolation
- **Centralized management**: Having all application data in a single, well-known bucket
- **Limited permissions**: When you can't create new buckets but can use existing ones

### Consider dedicated buckets when

- **Strict isolation** is required between applications
- **Independent lifecycle** management is needed for different volumes
- **Bucket-level policies** need to be different for each volume

## Security Notes

- Each prefix acts as an isolated namespace within the shared bucket
- Applications cannot access each other's prefixes when mounted
- The shared bucket must exist before creating PVCs
- Deleting a PVC only removes objects under that PVC's prefix
- The bucket itself and other prefixes remain untouched