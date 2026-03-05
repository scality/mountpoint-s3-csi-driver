# Quick Start Guide

This guide provides a fast way to deploy the Scality CSI Driver for S3 using Helm. It's designed for testing and evaluation purposes.

## Prerequisites

Before starting, ensure all requirements outlined in the **[Prerequisites](prerequisites.md)** guide are met.

<!-- markdownlint-disable MD046 -->
!!! warning "For Testing Only"
    The quick start guide is intended for testing purposes only. The installation uses default values including:

    - Kubernetes Namespace for driver installation: `default`
    - Kubernetes S3 Credentials Secret name: `s3-secret`
    - Default S3 Region (can be overridden at volume level): `us-east-1`

    For production deployments and to customize these values or use a different namespace, see the [installation guide](installation-guide.md).
<!-- markdownlint-enable MD046 -->

## Installation

**Step 1. Set configuration variables:**

!!! note "S3 Endpoint URL"
    For S3 endpoint URL, port number can be added if needed; example: `http://s3.example.com:8000`
    Port number can be omitted for default port `80` for HTTP or `443` for HTTPS

Replace these values with actual S3 endpoint and credentials.

```bash
export S3_ENDPOINT_URL="https://s3.example.com"
export ACCESS_KEY_ID="YOUR_ACCESS_KEY_ID"
export SECRET_ACCESS_KEY="YOUR_SECRET_ACCESS_KEY"
```

**Step 2. Create S3 credentials secret:**

```bash
kubectl create secret generic s3-secret \
  --from-literal=access_key_id="${ACCESS_KEY_ID}" \
  --from-literal=secret_access_key="${SECRET_ACCESS_KEY}"
```

**Step 3. Install the Scality S3 CSI driver:**

```bash
helm install \
  scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --version 2.1.1 \
  --set node.s3EndpointUrl="${S3_ENDPOINT_URL}"
```

**Step 4. Check installation:**

Check the status of the Helm release:

```bash
helm status scality-mountpoint-s3-csi-driver
```

Check if the CSI driver pods are running:

```bash
kubectl get pods -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
```

Expected output: One `s3-csi-node-*` pod per worker node, all in `Running` state.

Check CSI driver registration:

```bash
kubectl get csidriver s3.csi.scality.com
```

Verify CRD installation:

```bash
kubectl get crd mountpoints3podattachments.s3.csi.scality.com
```

!!! info "v2.0 Features"
    Version 2.0 introduces the MountpointS3PodAttachment CRD and pod-based mounter. The `mount-s3` namespace will be automatically created when volumes are first mounted.

## Uninstallation

!!! note "If Volumes Were Provisioned"
    If any applications (Kubernetes pods) were using PersistentVolumes or PersistentVolumeClaims provisioned using the S3 CSI driver,
    follow the complete [uninstallation guide](uninstallation.md) to properly clean up all resources.

For a quick start installation with no volumes provisioned, the driver can uninstall the driver with these simple steps:

**Step 1. Uninstall the Helm release:**

```bash
helm uninstall scality-mountpoint-s3-csi-driver
```

**Step 2. Delete the S3 credentials secret:**

```bash
kubectl delete secret s3-secret
```

**Step 3. Check removal:**

- Check that CSI driver is removed

    ```bash
    kubectl get csidriver s3.csi.scality.com
    ```

- Check that no driver pods remain

    ```bash
    kubectl get pods -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
    ```

## Next Steps

**Volume Provisioning**: See the [volume provisioning guides](../volume-provisioning/static-provisioning/overview.md) to learn how to use S3 buckets as volumes with applications.

**For Production Deployments**: Follow the [installation guide](installation-guide.md) for:

- Namespace isolation
- Secure credential management
- Custom helm configurations
