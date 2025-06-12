# Multiple Buckets in One Pod

This example shows how to mount multiple S3 buckets in a single pod.

## Features

- Two separate S3 buckets mounted in one pod
- Unique volume handles for each bucket
- Different mount paths (`/data` and `/data2`)

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
    - region us-west-2
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-multi-bucket-1-volume # Must be unique across all PVs
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
kind: PersistentVolume
metadata:
  name: s3-pv-2
spec:
  capacity:
    storage: 1200Gi # ignored, required
  accessModes:
    - ReadWriteMany # supported options: ReadWriteMany
  storageClassName: "" # Required for static provisioning
  claimRef: # To ensure no other PVCs can claim this PV
    namespace: default # Namespace is required even though it's in "default" namespace.
    name: s3-pvc-2 # Name of your PVC
  mountOptions:
    - allow-delete
    - region us-west-2
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-multi-bucket-2-volume # Must be unique across all PVs
    volumeAttributes:
      bucketName: s3-csi-driver-2
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-pvc-2
spec:
  accessModes:
    - ReadWriteMany # supported options: ReadWriteMany
  storageClassName: "" # required for static provisioning
  resources:
    requests:
      storage: 1200Gi # ignored, required
  volumeName: s3-pv-2 # Name of your PV
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
      args: ["-c", "echo 'Hello from the container!' >> /data/$(date -u).txt; echo 'Hello from the container!' >> /data2/$(date -u).txt; tail -f /dev/null"]
      volumeMounts:
        - name: persistent-storage
          mountPath: /data
        - name: persistent-storage-2
          mountPath: /data2
  volumes:
    - name: persistent-storage
      persistentVolumeClaim:
        claimName: s3-pvc
    - name: persistent-storage-2
      persistentVolumeClaim:
        claimName: s3-pvc-2
EOF
```

## Key Points

- Each PV must have a unique `volumeHandle`
- Each bucket requires separate PV and PVC resources
- Pod mounts both volumes at different paths

## Verify

```bash
kubectl get pod s3-app
kubectl exec s3-app -- ls -la /data
kubectl exec s3-app -- ls -la /data2
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc s3-pvc-2
kubectl delete pv s3-pv s3-pv-2
```

## Download YAML

[📁 multiple_buckets_one_pod.yaml](assets/multiple_buckets_one_pod.yaml)
