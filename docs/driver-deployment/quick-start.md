# Quick Start Guide

This guide provides a fast way to deploy the Scality S3 CSI Driver using Helm. It's designed for testing and evaluation purposes.

## Prerequisites

Before starting, ensure all requirements outlined in the **[Prerequisites](prerequisites.md)** guide are met.

<!-- markdownlint-disable MD046 -->
!!! warning "For Testing Only"
    The quick start guide is intended for testing purposes only. The installation uses default values including:

    - Kubernetes Namespace: default
    - Kubernetes S3 Credentials Secret name: s3-secret
    - DefaultS3 Region(can be overridden at volume level): us-east-1

    For production deployments and to customize these values or use a different namespace, see the [detailed installation guide](detailed-installation.md).
<!-- markdownlint-enable MD046 -->

## Installation

**Step 1. Set configuration variables:**

Replace these values with actual S3 endpoint and credentials:

```bash
export S3_ENDPOINT_URL="http://s3.example.com:8000"
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
  --set node.s3EndpointUrl="${S3_ENDPOINT_URL}"
```

**Step 4. Verify installation:**

Check the status of the Helm release:

```bash
helm status scality-mountpoint-s3-csi-driver
```

Check if the CSI driver pods are running:

```bash
kubectl get pods -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
```

You should see one `s3-csi-node-*` pod per worker node, all in `Running` state.

Check CSI driver registration:

```bash
kubectl get csidriver s3.csi.scality.com
```

## Uninstallation

If no volumes were provisioned, uninstall the driver using the following command:

```bash
helm uninstall scality-mountpoint-s3-csi-driver
```

The S3 sredentials secret can be deleted using the following command:

```bash
kubectl delete secret s3-secret
```

If volumes were provisioned, the driver can be uninstalled using the [uninstallation guide](uninstallation.md).

## Next Steps

- **For Production**: Follow the [detailed installation guide](detailed-installation.md) for:
  - Namespace isolation
  - Secure credential management  
  - Custom configurations

- **Volume Provisioning**: See the [volume provisioning guides](../volume-provisioning/how-to/index.md) to learn how to use S3 buckets with your applications
