# Allow Root Access

This example demonstrates the `allow-root` mount option, which permits root access to the filesystem even when non-root UID/GID are specified.

## Features

- Root access enabled despite non-root UID/GID settings
- Files owned by specified non-root user (1000:2000)
- Root can still read/write to the volume

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
    - ReadWriteMany # supported options: ReadWriteMany / ReadOnlyMany
  storageClassName: "" # Required for static provisioning
  claimRef: # To ensure no other PVCs can claim this PV
    namespace: default # Namespace is required even though it's in "default" namespace.
    name: s3-pvc # Name of your PVC
  mountOptions:
    - allow-delete
    - uid=1000
    - gid=2000
    - allow-root # Allow root access even with non-root uid/gid
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-allow-root-volume # Must be unique across all PVs
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
      args: ["-c", "echo 'Root access test' > /data/root-test.txt; ls -la /data; tail -f /dev/null"]
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

- `allow-root` - Permits root user access even when `uid` and `gid` are set to non-root values

## Behavior

Without `allow-root`:

- When `uid=1000` and `gid=2000` are set, only user 1000 can access the mount
- Root access is restricted

With `allow-root`:

- User 1000 can access the mount (as specified by uid/gid)
- Root can also access the mount despite the non-root uid/gid settings
- Files are still owned by 1000:2000

## Use Cases

- Administrative access to volumes with non-root ownership
- Debugging and troubleshooting scenarios
- Mixed access patterns where both root and specific users need access
- Container init processes that run as root but application runs as non-root

## Verify

```bash
kubectl get pod s3-app
kubectl exec s3-app -- ls -la /data
# Files should be owned by 1000:2000 but accessible by root (UID 0)
kubectl exec s3-app -- id
# Should show uid=0(root) gid=0(root)
```

## Security Considerations

- Use `allow-root` carefully in security-sensitive environments
- Consider using `allow-other` instead for broader non-root access
- Ensure your security policies permit root access to the volume

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 allow-root.yaml](assets/allow-root.yaml)
