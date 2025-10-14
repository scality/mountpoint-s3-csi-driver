# StorageClass Reference

This reference guide covers all parameters and configuration options available in StorageClasses for dynamic provisioning with the Scality S3 CSI driver.

## Basic StorageClass Structure

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-dynamic
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
parameters:
  # CSI-specific parameters
mountOptions:
  # Mount options for all volumes using this StorageClass
```

## Required Fields

| Field | Value | Description |
|-------|-------|-------------|
| `provisioner` | `s3.csi.scality.com` | Must match the CSI driver name |
| `reclaimPolicy` | `Delete` or `Retain` | Controls bucket fate when PV is deleted (bucket deletion only occurs if empty) |
| `volumeBindingMode` | `Immediate` or `WaitForFirstConsumer` | When to create the bucket |

For more information on parameters, see the [Kubernetes StorageClass documentation](https://kubernetes.io/docs/concepts/storage/storage-classes/).

### Basic Examples for different secret configurations

```yaml title="Separate provisioner and node secrets"
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-basic
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
parameters:
  csi.storage.k8s.io/provisioner-secret-name: s3-provisioner-secret
  csi.storage.k8s.io/provisioner-secret-namespace: kube-system
  csi.storage.k8s.io/node-publish-secret-name: s3-node-secret
  csi.storage.k8s.io/node-publish-secret-namespace: kube-system
mountOptions:
  - allow-delete
  - allow-other
```

```yaml title="Same secret for both provisioner and node operations"
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-shared-secret
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
parameters:
  # Both secrets point to the same Secret
  csi.storage.k8s.io/provisioner-secret-name: s3-shared-credentials
  csi.storage.k8s.io/provisioner-secret-namespace: kube-system
  csi.storage.k8s.io/node-publish-secret-name: s3-shared-credentials
  csi.storage.k8s.io/node-publish-secret-namespace: kube-system
mountOptions:
  - allow-delete
  - allow-other
```

!!! warning "Single Secret Configuration Not Supported"
    Configuring only `provisioner-secret` OR only `node-publish-secret` is **not recommended** and may not work as expected.
    The controller uses `provisioner-secret` presence to determine if secret-based authentication is enabled (CSI spec limitation).
    Always configure both secrets together, pointing to the same Secret if you don't need separate admin/user credentials.

```yaml title="No secrets - Driver level secrets will be used for CreateBucket, DeleteBucket and mount operations"
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-basic
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
parameters:
mountOptions:
  - allow-delete
  - allow-other
```

### Volume Binding Mode Examples

```yaml title="Immediate binding (Default) - Bucket created immediately when PVC is created"
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-immediate
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
volumeBindingMode: Immediate  # Default behavior
parameters:
  csi.storage.k8s.io/provisioner-secret-name: s3-provisioner-secret
  csi.storage.k8s.io/provisioner-secret-namespace: kube-system
  csi.storage.k8s.io/node-publish-secret-name: s3-node-secret
  csi.storage.k8s.io/node-publish-secret-namespace: kube-system
mountOptions:
  - allow-delete
  - allow-other
```

```yaml title="WaitForFirstConsumer - Bucket creation delayed until pod is scheduled"
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-wait-for-consumer
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer  # Wait for pod scheduling
parameters:
  csi.storage.k8s.io/provisioner-secret-name: s3-provisioner-secret
  csi.storage.k8s.io/provisioner-secret-namespace: kube-system
  csi.storage.k8s.io/node-publish-secret-name: s3-node-secret
  csi.storage.k8s.io/node-publish-secret-namespace: kube-system
mountOptions:
  - allow-delete
  - allow-other
```

```yaml title="WaitForFirstConsumer with ${pv.name} templating - Requires delayed binding"
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-pv-name-templating
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer  # Required for ${pv.name} templating
parameters:
  # Using ${pv.name} templating requires WaitForFirstConsumer
  csi.storage.k8s.io/provisioner-secret-name: "${pv.name}-secret"
  csi.storage.k8s.io/provisioner-secret-namespace: "${pvc.namespace}"
  csi.storage.k8s.io/node-publish-secret-name: "${pv.name}-secret"
  csi.storage.k8s.io/node-publish-secret-namespace: "${pvc.namespace}"
mountOptions:
  - allow-delete
  - allow-other
```

### Usage Examples

```yaml title="PVC using StorageClass for dynamic provisioning"
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-app-storage
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-immediate  # References the StorageClass
  resources:
    requests:
      storage: 100Gi  # Arbitrary value for S3 - actual size is unlimited
```

```yaml title="Pod using the above PVC"
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
        claimName: my-app-storage  # References the PVC created above
```

```yaml title="Pod with inline PVC using StorageClass"
apiVersion: v1
kind: Pod
metadata:
  name: s3-inline-app
spec:
  containers:
    - name: app
      image: ubuntu
      command: ["/bin/sh"]
      args: ["-c", "echo 'Hello from inline PVC!' >> /data/$(date -u).txt; tail -f /dev/null"]
      volumeMounts:
        - name: persistent-storage
          mountPath: /data
  volumes:
    - name: persistent-storage
      persistentVolumeClaim:
        claimName: app-storage  # Inline PVC defined below
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-storage
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-immediate  # References StorageClass for dynamic provisioning
  resources:
    requests:
      storage: 50Gi
```
