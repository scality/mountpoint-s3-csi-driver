# KMS Server-Side Encryption

This example demonstrates using AWS KMS for server-side encryption of S3 objects.

## Features

- Server-side encryption with AWS KMS
- Customer-managed KMS key
- Secure data encryption at rest

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
    - sse aws:kms # Use customer managed KMS key for server side encryption
    - sse-kms-key-id arn:aws:kms:us-west-2:012345678900:key/00000000-0000-0000-0000-000000000000 # set key id (optional)
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-kms-volume # Must be unique across all PVs
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

- `sse aws:kms` - Enables KMS server-side encryption
- `sse-kms-key-id <ARN>` - Specifies the KMS key ARN (optional)

## Prerequisites

- KMS key must exist in the same region as the S3 bucket
- IAM permissions for KMS key usage
- Replace the KMS key ARN with your actual key

## Verify

```bash
kubectl get pod s3-app
kubectl exec s3-app -- echo "encrypted data" > /data/test.txt
# Check S3 console for encryption status
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 kms_sse.yaml](assets/kms_sse.yaml)
