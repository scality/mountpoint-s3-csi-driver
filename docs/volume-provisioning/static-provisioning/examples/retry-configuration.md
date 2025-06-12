# Retry Configuration

This example shows how to configure S3 request retry behavior using the `aws-max-attempts` mount option.

## Features

- Configures S3 request retries to 5 attempts
- Useful for unstable network conditions
- Improves resilience for S3 operations

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
    - aws-max-attempts 5 # Configure number of retries for S3 requests
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-retry-volume # Must be unique across all PVs
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

## Key Mount Option

- `aws-max-attempts 5` - Sets the maximum number of retry attempts for S3 requests

## Use Cases

- Unstable network connections
- High-latency environments
- Improved fault tolerance

## Verify

```bash
kubectl get pod s3-app
kubectl logs s3-app
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 aws_max_attempts.yaml](assets/aws_max_attempts.yaml)
