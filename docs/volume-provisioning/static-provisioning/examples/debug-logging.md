# Debug Logging

This example demonstrates how to enable debug logging for troubleshooting S3 CSI driver issues.

## Features

- Mountpoint debug logging enabled
- AWS CRT client verbose logging
- Useful for troubleshooting connectivity and performance issues

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
    - debug # Enable Mountpoint debug logging
    - debug-crt # Enable verbose AWS CRT client logging
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-debug-volume # Must be unique across all PVs
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

- `debug` - Enables Mountpoint debug logging to systemd journal
- `debug-crt` - Enables verbose AWS Common Runtime (CRT) S3 client logging

## Viewing Debug Logs

Debug logs are written to the systemd journal on the node where the pod is running:

```bash
# Find the node where the pod is running
kubectl get pod s3-app -o wide

# SSH to the node and view logs
sudo journalctl -u kubelet -f | grep mountpoint
```

## Use Cases

- Troubleshooting mount failures
- Debugging S3 connectivity issues
- Performance analysis
- Understanding S3 request patterns

## Verify

```bash
kubectl get pod s3-app
kubectl exec s3-app -- ls -la /data
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 debug-logging.yaml](assets/debug-logging.yaml)
