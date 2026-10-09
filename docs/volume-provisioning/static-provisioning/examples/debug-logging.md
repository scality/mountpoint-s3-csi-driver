# Debug Logging

This example demonstrates how to enable debug logging for troubleshooting S3 CSI driver issues.

## Features

- Mountpoint debug logging enabled
- CRT client verbose logging
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

- `debug` - Enables Mountpoint debug logging to Mountpoint Pod container logs
- `debug-crt` - Enables verbose AWS Common Runtime (CRT) S3 client logging

## Viewing Debug Logs

Debug logs are written to the Mountpoint Pod container logs. Use `kubectl logs` to view them:

```bash
# Find the Mountpoint Pod serving your volume
kubectl get pods -n kube-system -l app=mountpoint-s3-csi-mounter

# View logs from the Mountpoint Pod
kubectl logs -n kube-system <mountpoint-pod-name>

# Stream logs in real-time
kubectl logs -n kube-system <mountpoint-pod-name> -f
```

## Use Cases

- Troubleshooting mount failures
- Debugging S3 connectivity issues
- Performance analysis
- Understanding S3 request patterns

## Check Pod-Level Access to the Mounted S3 Volume

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
