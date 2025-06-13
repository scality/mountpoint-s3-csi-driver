# Override S3 Region

This example demonstrates how to override the S3 region for a specific volume using the `region` mount option.

## Features

- Override driver's global S3 region setting
- Access buckets in different regions from same cluster
- Useful for multi-region deployments

## Deploy

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-region-override-pv
spec:
  capacity:
    storage: 1200Gi # Arbitrary value - not used for S3
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  claimRef:
    namespace: default
    name: s3-region-override-pvc
  mountOptions:
    - allow-delete
    - region=eu-west-1  # Override S3 region for this specific volume
  csi:
    driver: s3.csi.scality.com
    volumeHandle: s3-csi-region-override-volume # Must be unique across all PVs
    volumeAttributes:
      bucketName: my-eu-west-bucket
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-region-override-pvc
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  resources:
    requests:
      storage: 1200Gi # Ignored but required
  volumeName: s3-region-override-pv # Must match PV `metadata.name`
---
apiVersion: v1
kind: Pod
metadata:
  name: s3-region-app
spec:
  containers:
    - name: app
      image: ubuntu
      command: ["/bin/sh"]
      args: ["-c", "echo 'Data from EU West region!' >> /data/$(date -u).txt; tail -f /dev/null"]
      volumeMounts:
        - name: persistent-storage
          mountPath: /data
  volumes:
    - name: persistent-storage
      persistentVolumeClaim:
        claimName: s3-region-override-pvc
EOF
```

## Key Mount Option

- `region=eu-west-1` - Override S3 region for this specific volume

## Important Notes

⚠️ **Region Must Match**: The region specified must match the actual region where your S3 bucket is located.

## Check Pod-Level Access to the Mounted S3 Volume

```bash
kubectl get pod s3-region-app
kubectl exec s3-region-app -- ls -la /data
```

## Cleanup

```bash
kubectl delete pod s3-region-app
kubectl delete pvc s3-region-override-pvc
kubectl delete pv s3-region-override-pv
```

## Download YAML

[📁 override-region.yaml](assets/override-region.yaml)
