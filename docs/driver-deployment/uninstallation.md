# Uninstallation Guide

This guide provides instructions for completely removing the Scality S3 CSI Driver and all associated resources from your Kubernetes cluster.

## Before You Begin

<!-- markdownlint-disable MD046 -->
!!! warning "Data Persistence"
    - Uninstalling the CSI driver does **not** delete data in S3 buckets
    - Existing PersistentVolumes with `Retain` policy will preserve bucket data
    - Ensure all workloads using S3 volumes are stopped before uninstalling.
<!-- markdownlint-enable MD046 -->

!!! danger "Access to Data"
    If the driver is uninstalled while applications are still using S3 volumes, those applications will lose access to their data and may fail.
    Always stop all workloads using S3 volumes before uninstalling the driver. (TODO: verify this)

## Uninstallation Steps

### Step 1: Remove Workloads Using S3 Volumes (TODO: verify this)

First, identify and delete all pods using S3 volumes:

```bash
# Find pods with S3 PVCs
kubectl get pods --all-namespaces -o json | jq -r '.items[] | select(.spec.volumes[]?.persistentVolumeClaim) | "\(.metadata.namespace)/\(.metadata.name)"'

# Delete your application pods that use S3 volumes
```

### Step 2: Remove PVCs and PVs

Delete all PersistentVolumeClaims using the S3 CSI driver:

```bash
# List PVCs using S3 volumes
kubectl get pvc --all-namespaces -o json | jq -r '.items[] | select(.spec.volumeName | startswith("s3-")) | "\(.metadata.namespace)/\(.metadata.name)"'

# Delete PVCs as needed
```

Delete PersistentVolumes:

```bash
# List PVs using the S3 CSI driver
kubectl get pv -o json | jq -r '.items[] | select(.spec.csi.driver == "s3.csi.scality.com") | .metadata.name'

# Delete PVs as needed
```

### Step 3: Uninstall the S3 CSI Driver Helm Release

Detect the namespace where the driver is installed and export it as an environment variable:

```bash
export NAMESPACE=$(kubectl get pods --all-namespaces -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -o jsonpath='{.items[0].metadata.namespace}')
echo "Scality S3 CSI Driver found in namespace: ${NAMESPACE}"
```

Get the secret name from the Helm release:

```bash
export SECRET_NAME=$(helm get values scality-mountpoint-s3-csi-driver -n ${NAMESPACE} -o json | jq -r '.s3CredentialSecret.name // "s3-secret"')
echo "Secret name: ${SECRET_NAME}"
```

**Uninstall the release:**

```bash
helm uninstall scality-mountpoint-s3-csi-driver -n ${NAMESPACE}
```

**Delete the S3 credentials secret:**

```bash
kubectl delete secret ${SECRET_NAME} -n ${NAMESPACE}
```

### Step 4: Remove Namespace (Optional)

If you created a dedicated namespace and no longer need it:

```bash
# Verify namespace is empty first
kubectl get all -n ${NAMESPACE}

# Delete namespace
kubectl delete namespace ${NAMESPACE}
```

### Step 5: Verify Complete Removal

Ensure all CSI driver components are removed:

```bash
# Check for remaining pods
kubectl get pods --all-namespaces | grep s3-csi

# Check CSI driver registration
kubectl get csidriver s3.csi.scality.com

# Check for remaining service accounts
kubectl get sa --all-namespaces | grep s3-csi

# Check for remaining cluster roles
kubectl get clusterrole | grep s3-csi
kubectl get clusterrolebinding | grep s3-csi
```
