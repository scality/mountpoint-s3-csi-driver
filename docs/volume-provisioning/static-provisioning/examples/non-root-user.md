# Non-Root User Access

This example shows how to configure S3 volumes for pods running as non-root users.

## Features

- Pod runs as non-root user (UID 1000, GID 2000)
- Mount options configured for non-root access
- Uses `allow-other` for proper permissions

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
    - uid=1000
    - gid=2000
    - allow-other
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-non-root-volume # Must be unique across all PVs
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
  securityContext:
    runAsUser: 1000
    runAsGroup: 2000
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

- `uid=1000` - Sets file ownership to user ID 1000
- `gid=2000` - Sets file ownership to group ID 2000  
- `allow-other` - Allows non-root users to access the mount

## Verify

```bash
kubectl get pod s3-app
kubectl exec s3-app -- id
kubectl exec s3-app -- ls -la /data
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 non_root.yaml](assets/non_root.yaml)
