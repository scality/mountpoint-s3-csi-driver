# Basic Dynamic Provisioning

This example demonstrates the simplest dynamic provisioning setup using dedicated buckets.

## Features

- **Dedicated bucket per PVC**: Each PVC gets its own S3 bucket
- **Automatic cleanup**: Buckets are deleted when PVCs are deleted
- **Default configuration**: Uses default parameters for region and bucket naming
- **Single pod usage**: Shows how to use the dynamically provisioned volume in a pod

## YAML Configuration

```yaml
# StorageClass for dynamic provisioning
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi
provisioner: s3.csi.scality.com
parameters:
  # Default configuration - creates dedicated buckets
  bucketNaming: dedicated
  s3Region: us-east-1
volumeBindingMode: Immediate
reclaimPolicy: Delete
---
# PersistentVolumeClaim requesting storage
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-dynamic-pvc
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi
  resources:
    requests:
      storage: 100Gi  # Arbitrary value - S3 storage is unlimited
---
# Pod using the dynamically provisioned volume
apiVersion: v1
kind: Pod
metadata:
  name: s3-dynamic-app
spec:
  containers:
    - name: app
      image: ubuntu
      command: ["/bin/sh"]
      args: ["-c", "echo 'Hello from dynamic S3 storage!' > /data/hello.txt; cat /data/hello.txt; tail -f /dev/null"]
      volumeMounts:
        - name: s3-storage
          mountPath: /data
  volumes:
    - name: s3-storage
      persistentVolumeClaim:
        claimName: s3-dynamic-pvc
```

## Deployment Steps

1. **Apply the configuration**:
   ```bash
   kubectl apply -f basic-dynamic-provisioning.yaml
   ```

2. **Verify StorageClass**:
   ```bash
   kubectl get storageclass s3-csi
   ```

3. **Check PVC status**:
   ```bash
   kubectl get pvc s3-dynamic-pvc
   # Should show "Bound" status
   ```

4. **Verify the pod**:
   ```bash
   kubectl get pod s3-dynamic-app
   kubectl logs s3-dynamic-app
   ```

5. **Check the created bucket**: The driver will create a bucket with name like `s3-csi-pvc-{uuid}`

## Cleanup

To delete all resources and the S3 bucket:

```bash
kubectl delete pod s3-dynamic-app
kubectl delete pvc s3-dynamic-pvc
# The bucket will be automatically deleted due to reclaimPolicy: Delete
```

## What Happens Behind the Scenes

1. **StorageClass** defines the provisioning behavior
2. **PVC creation** triggers the CSI driver's `CreateVolume` call
3. **Bucket creation**: Driver creates a new S3 bucket with sanitized name
4. **PV creation**: Driver creates a PV with bucket information in volume attributes
5. **Pod mounting**: Node driver mounts the S3 bucket using mountpoint-s3
6. **Cleanup**: When PVC is deleted, driver deletes all bucket contents and the bucket itself

## Notes

- The bucket name will be automatically generated based on the PVC name/UUID
- All bucket names start with `s3-csi-` prefix for safety
- The `reclaimPolicy: Delete` ensures buckets are cleaned up when PVCs are deleted
- Storage capacity is ignored for S3 but required by Kubernetes