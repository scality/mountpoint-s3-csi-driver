# File and Directory Permissions

This example demonstrates how to configure custom file and directory permissions using mount options.

## Features

- Custom file permissions (0640 - rw-r-----)
- Custom directory permissions (0750 - rwxr-x---)
- File overwrite capability enabled
- Non-root user access with specific UID/GID

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
    - uid=1000
    - gid=2000
    - allow-other
    - file-mode=0640 # Files: rw-r-----
    - dir-mode=0750  # Directories: rwxr-x---
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-file-perms-volume # Must be unique across all PVs
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
  securityContext:
    runAsUser: 1000
    runAsGroup: 2000
  containers:
    - name: app
      image: ubuntu
      command: ["/bin/sh"]
      args: ["-c", "echo 'Hello from the container!' > /data/test.txt; mkdir -p /data/testdir; ls -la /data; tail -f /dev/null"]
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

- `file-mode=0640` - Sets file permissions to rw-r----- (owner: read/write, group: read, others: none)
- `dir-mode=0750` - Sets directory permissions to rwxr-x--- (owner: full, group: read/execute, others: none)
- `allow-overwrite` - Permits overwriting existing S3 objects
- `uid=1000` / `gid=2000` - Sets ownership to specific user and group IDs

## Permission Breakdown

| Permission | Files (0640) | Directories (0750) |
|------------|--------------|-------------------|
| Owner      | rw- (read/write) | rwx (read/write/execute) |
| Group      | r-- (read only) | r-x (read/execute) |
| Others     | --- (no access) | --- (no access) |

## Check Pod-Level Access to the Mounted S3 Volume Permissions

```bash
kubectl get pod s3-app
kubectl exec s3-app -- ls -la /data
# Should show:
# -rw-r----- 1 1000 2000 ... test.txt
# drwxr-x--- 2 1000 2000 ... testdir
```

## Use Cases

- Restricting file access to specific users/groups
- Compliance with security policies
- Multi-tenant environments with shared storage
- Applications requiring specific permission models

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 file-permissions.yaml](assets/file-permissions.yaml)
