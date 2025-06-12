# Multiple Pods Sharing One Volume

This example shows how to share a single S3 volume across multiple pods using a Deployment.

## Features

- Single PV shared across multiple pods
- Uses Deployment for pod management
- 3 replicas accessing the same S3 bucket

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
    - region us-east-1
  csi:
    driver: s3.csi.scality.com # required
    volumeHandle: s3-csi-shared-volume # Must be unique across all PVs
    volumeAttributes:
      bucketName: s3-csi-bucket-name
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
apiVersion: apps/v1
kind: Deployment
metadata:
  name: s3-app
  labels:
    app: s3-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: s3-app
  template:
    metadata:
      labels:
        app: s3-app
    spec:
      containers:
      - name: s3-app
        image: ubuntu
        command: ["/bin/sh"]
        args: ["-c", "echo 'Hello from the container!' >> /data/$(date -u).txt; tail -f /dev/null"]
        volumeMounts:
        - name: persistent-storage
          mountPath: /data
        ports:
        - containerPort: 80
      volumes:
      - name: persistent-storage
        persistentVolumeClaim:
          claimName: s3-pvc
EOF
```

## Key Points

- `ReadWriteMany` access mode allows multiple pods to mount the volume
- All pods share the same S3 bucket data
- Deployment manages pod lifecycle and scaling

## Verify

```bash
kubectl get deployment s3-app
kubectl get pods -l app=s3-app
kubectl exec deployment/s3-app -- ls -la /data
```

## Scale the Deployment

```bash
kubectl scale deployment s3-app --replicas=5
```

## Cleanup

```bash
kubectl delete deployment s3-app
kubectl delete pvc s3-pvc
kubectl delete pv s3-pv
```

## Download YAML

[📁 multiple_pods_one_pv.yaml](assets/multiple_pods_one_pv.yaml)
