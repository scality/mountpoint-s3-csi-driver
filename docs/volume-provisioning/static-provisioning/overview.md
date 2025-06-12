# Static Provisioning

!!! note "Bucket Pre-Creation Required"
    For static provisioning, the S3 bucket must be pre-created and bucket name must be provided in the PV specification.

Static provisioning allows using an existing S3 bucket as a persistent volume in a Kubernetes cluster. The S3 bucket must be pre-created and the PersistentVolume (PV) resource manually defined.

## Key Requirements

| Configuration | Location | Value | Required | Description |
|---------------|----------|-------|----------|-------------|
| **Storage Capacity** | `spec.capacity.storage` (PV)<br/>`spec.resources.requests.storage` (PVC) | Example: `1200Gi` | **Yes** | Can be any arbitrary value as S3 is not block storage. Required by Kubernetes but ignored |
| **Access Mode** | `spec.accessModes` | `ReadWriteMany` | **Yes** | Only access mode supported. Required for both PV and PVC |
| **Storage Class** | `spec.storageClassName` | `""` (empty) | **Yes** | Must be empty for static provisioning. Required for both PV and PVC |
| **Volume Name** | `spec.volumeName` (PVC only) | PV name | **Yes** | Must match PV `metadata.name`. Links PVC to specific PV |
| **Claim Reference** | `spec.claimRef` (PV only) | PVC reference | **Yes** | Binds PV to specific PVC to prevent other PVCs from claiming it |

## CSI Configuration

### `spec.csi` Attributes

These attributes are specific to the CSI driver and control how it interacts with the S3 bucket.

| Attribute (`spec.csi.*`) | Description | Example Value | Required |
|--------------------------|-------------|---------------|----------|
| `driver` | The name of the CSI driver. Must be `s3.csi.scality.com` | `s3.csi.scality.com` | **Yes** |
| `volumeHandle` | A unique identifier for this volume within the driver. Can be any string, but it's common practice to use the bucket name or a descriptive ID | `my-s3-bucket-pv` | **Yes** |
| `volumeAttributes.bucketName` | The name of the S3 bucket to mount. Bucket must be pre-created | `"my-application-data"` | **Yes** |
| `volumeAttributes.authenticationSource` | Specifies the source of AWS credentials for this volume. If set to `"secret"`, `nodePublishSecretRef` must also be provided. If omitted or set to `"driver"`, global driver credentials are used | `"secret"` or `"driver"` (or omit) | No |
| `nodePublishSecretRef.name` | The name of the Kubernetes Secret containing S3 credentials (`access_key_id`, `secret_access_key`) for this specific volume. Used when `authenticationSource` is `"secret"` | `"my-volume-credentials"` | Conditionally |
| `nodePublishSecretRef.namespace` | The namespace of the Kubernetes Secret specified in `name`. Must be the same namespace as the PersistentVolumeClaim that will bind to this PV | `"my-secret-namespace"` | Conditionally |

### `spec.mountOptions`

Additional options to customize S3 mounting behavior. See [mount-options.md](mount-options.md) for the complete list of supported options.

## Basic Structure

Static provisioning workflow uses three Kubernetes resources:

1. **PersistentVolume (PV)** - Defines the S3 bucket and configuration
2. **PersistentVolumeClaim (PVC)** - Requests the PV for pod usage
3. **Pod/Application** - Consumes the storage by mounting the PVC

## Basic Example

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv
spec:
  capacity:
    storage: 1200Gi # Arbitrary value - not used for S3
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  claimRef:
    namespace: default
    name: s3-pvc
  mountOptions:
    - allow-delete
    - region us-west-2
    # See mount-options.md for all available options
  csi:
    driver: s3.csi.scality.com
    volumeHandle: s3-csi-driver-volume-basic # Must be unique across all PVs
    volumeAttributes:
      bucketName: s3-csi-driver
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-pvc
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  resources:
    requests:
      storage: 1200Gi # Ignored but required
  volumeName: s3-pv # Must match PV `metadata.name`
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
      args: ["-c", "echo 'Hello!' >> /data/$(date -u).txt; tail -f /dev/null"]
      volumeMounts:
        - name: persistent-storage
          mountPath: /data
  volumes:
    - name: persistent-storage
      persistentVolumeClaim:
        claimName: s3-pvc
```

## Examples

- [Basic Static Provisioning](examples/basic-static-provisioning.md) - Simple S3 bucket mounting
- [Shared Cache Configuration](examples/shared-cache.md) - Basic shared cache setup
- [Secret-Based Authentication](examples/secret-authentication.md) - Volume-level credentials
- [Non-Root User Access](examples/non-root-user.md) - Non-root user configuration
- [Multiple Pods Sharing Volume](examples/multiple-pods-shared-volume.md) - Shared volume across pods
- [Multiple Buckets in One Pod](examples/multiple-buckets.md) - Multiple buckets in single pod
- [Local Caching](examples/local-caching.md) - Advanced caching with size limits
- [KMS Server-Side Encryption](examples/kms-encryption.md) - AWS KMS encryption
- [Retry Configuration](examples/retry-configuration.md) - S3 request retry settings
- [Debug Logging](examples/debug-logging.md) - Enable debug and verbose logging
- [File and Directory Permissions](examples/file-permissions.md) - Custom file/directory permissions
- [Allow Root Access](examples/allow-root.md) - Root access with non-root UID/GID
- [Bucket Prefix Mounting](examples/bucket-prefix.md) - Mount specific bucket prefix/folder
- [Override S3 Region](examples/override-region.md) - Override S3 region for specific volumes
