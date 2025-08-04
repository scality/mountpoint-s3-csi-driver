# Secret-Based Authentication

This example demonstrates using Kubernetes Secrets to provide S3 credentials for volume-level authentication.

## Features

- Volume-level S3 credentials using Kubernetes Secrets
- Different credentials per PV
- Secure credential management

## Deploy

```bash
kubectl apply -f - <<EOF
# Secret Authentication Example
# This example demonstrates using a Kubernetes Secret to provide S3 credentials for the Mountpoint S3 CSI Driver.
# This authentication method is particularly useful for:
# 1. The user wants to set their own credentials which are different than the driver level credentials
# 2. Using different credentials for different persistent volumes

# First, create a Secret containing the S3 credentials
apiVersion: v1
kind: Secret
metadata:
  name: s3-credentials
  namespace: default
type: Opaque
stringData:
  # Using plain text values
  access_key_id: "AKIAXXXXXXXXXXXXXXXXX"
  secret_access_key: "SECRETXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

  # You can also create the secret using kubectl:
  # kubectl create secret generic s3-credentials \
  #     --from-literal=access_key_id="ACCESS_KEY_ID" \
  #     --from-literal=secret_access_key="SECRET_ACCESS_KEY"

---
# Next, create a PersistentVolume that references the Secret
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv
spec:
  capacity:
    storage: 1000Gi # ignored, required
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: "" # Required for static provisioning
  mountOptions:
    - allow-delete
  csi:
    driver: s3.csi.scality.com
    volumeHandle: s3-csi-secret-auth-volume # Must be unique across all PVs
    volumeAttributes:
      bucketName: my-bucket
      authenticationSource: secret # Set auth source to use the Secret
    nodePublishSecretRef:
      name: s3-credentials # Reference to the Secret containing credentials
      namespace: default
---
# Create a PersistentVolumeClaim that references the PV
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  resources:
    requests:
      storage: 1000Gi # Ignored, required
  volumeName: s3-pv
---
# Finally, create a Pod that uses the volume
apiVersion: v1
kind: Pod
metadata:
  name: s3-app
  namespace: default
spec:
  containers:
  - name: app
    image: busybox
    command: ["tail", "-f", "/dev/null"]
    volumeMounts:
    - name: s3-storage
      mountPath: /data
  volumes:
  - name: s3-storage
    persistentVolumeClaim:
      claimName: s3-pvc
EOF
```

## Alternative: Create Secret with kubectl

```bash
kubectl create secret generic s3-credentials \
    --from-literal=access_key_id="YOUR_ACCESS_KEY_ID" \
    --from-literal=secret_access_key="YOUR_SECRET_ACCESS_KEY"
```

## Key Configuration

- `authenticationSource: secret` - Enables secret-based authentication
- `nodePublishSecretRef` - References the Kubernetes Secret

## Check Pod-Level Access to the Mounted S3 Volume

```bash
kubectl get secret s3-credentials
kubectl get pod s3-app
kubectl exec s3-app -- ls -la /data
```

## Cleanup

```bash
kubectl delete pod s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
kubectl delete secret s3-credentials
```

## Download YAML

[📁 secret_authentication.yaml](assets/secret_authentication.yaml){:download}
