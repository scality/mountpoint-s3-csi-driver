# Bucket Prefix Mounting

This example demonstrates how to mount only a specific prefix (folder) from an S3 bucket using the `prefix` mount option.

## Features

- Mounts only the `app-data/` prefix from the bucket
- The prefix becomes the root of the mount
- Isolates access to a specific "folder" within the bucket
- Useful for multi-tenant scenarios

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
    - allow-delete
    - allow-overwrite
    - prefix=app-data/ # Mount only the 'app-data/' prefix from the bucket
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-prefix-volume # Must be unique across all PVs
    volumeAttributes:
      bucketName: s3-csi-driver-test
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
      args: ["-c", "echo 'Data in app-data prefix' > /data/test-file.txt; mkdir -p /data/subdir; echo 'Nested data' > /data/subdir/nested.txt; ls -la /data; tail -f /dev/null"]
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

- `prefix=app-data/` - Mounts only objects with the `app-data/` prefix from the bucket

## How It Works

**Bucket Structure:**

```text
my-bucket/
├── app-data/           ← This becomes the mount root
│   ├── file1.txt
│   └── subdir/
│       └── file2.txt
├── other-data/         ← Not visible in the mount
│   └── file3.txt
└── root-file.txt       ← Not visible in the mount
```

**Mount View:**

```text
/data/                  ← Mount point
├── file1.txt           ← Actually app-data/file1.txt in S3
└── subdir/
    └── file2.txt       ← Actually app-data/subdir/file2.txt in S3
```

## Important Notes

- The prefix **must end with a forward slash** (`/`)
- Files created in the mount will be stored with the prefix in S3
- Only objects with the specified prefix are visible
- The prefix itself becomes the root directory of the mount

## Use Cases

- **Multi-tenancy**: Different applications accessing different prefixes of the same bucket
- **Data organization**: Isolating different types of data within a bucket
- **Security**: Restricting access to specific parts of a bucket
- **Migration**: Gradually moving data by mounting specific prefixes

## Check Pod-Level Access to the Mounted S3 Volume

```bash
kubectl get pod s3-app
kubectl exec s3-app -- ls -la /data
kubectl exec s3-app -- cat /data/test-file.txt
# Files will be stored as app-data/test-file.txt in the S3 bucket
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 bucket-prefix.yaml](https://github.com/scality/mountpoint-s3-csi-driver/blob/main/docs/volume-provisioning/static-provisioning/examples/assets/bucket-prefix.yaml)
