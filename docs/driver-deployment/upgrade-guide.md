# Upgrade Guide

This guide provides instructions for upgrading the Scality CSI Driver for S3 from version 1.2.0 to 2.0.

!!! info "Version Compatibility"
    This upgrade guide is specifically for upgrading from **v1.2.0 to v2.0**.
    **Upgrading from earlier versions**: Versions earlier than v1.2.0 must first be upgraded to v1.2.0 before proceeding with this guide.
    Follow the standard upgrade procedure to reach v1.2.0, then use this guide to upgrade to v2.0.

## Prerequisites

Before upgrading, ensure all requirements outlined in the **[Prerequisites](prerequisites.md)** guide are met for the target version.

## Pre-Upgrade Steps

**Step 1. Set namespace variable:**

Set the namespace where the driver is currently installed:

```bash
export NAMESPACE="scality-s3-csi"  # Replace with actual namespace
```

**Step 2. Check current installation:**

```bash
# Get current release info
helm list --all-namespaces | grep scality-mountpoint-s3-csi-driver

# Check current version
kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -o jsonpath='{.items[0].spec.containers[0].image}'
```

**Step 3. Review changes:**

Check the [Release Notes](../release-notes.md) for version-specific changes and breaking changes.

**Step 4. Install/Update CRDs (Required for v2.0):**

!!! warning "CRD Installation Required"
    Version 2.0 introduces the `MountpointS3PodAttachment` CRD for tracking volume attachments.
    Helm v3 does not automatically update CRDs on upgrades, so you **must** install/update CRDs manually before upgrading.

Install CRDs using kustomize (recommended):

```bash
# Install from GitHub repository
kubectl apply -k github.com/scality/mountpoint-s3-csi-driver
```

Or, if the repository has been cloned locally:

```bash
# Install from local repository root
kubectl apply -k .
```

Verify CRD installation:

```bash
kubectl get crd mountpoints3podattachments.s3.csi.scality.com
```

## Upgrade Path

### Step 1: Ensure Running v1.2.0

!!! warning "Prerequisite Version Required"
    Before upgrading to v2.0, the driver must be running version 1.2.0. If already on v1.2.0, skip to [Upgrading to v2.0](#upgrading-to-v20).

If running a version earlier than v1.2.0, upgrade to v1.2.0 first:

!!! important "Version Specification Required"
    The Helm chart repository will default to the latest released version. Version 1.2.0 must be explicitly specified in the upgrade command.

```bash
helm upgrade scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --version 1.2.0 \
  --namespace ${NAMESPACE} \
  --reuse-values
```

Verify the upgrade to v1.2.0:

```bash
# Check that version 1.2.0 is running
kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -o jsonpath='{.items[0].spec.containers[0].image}'
```

### Step 2: Dry Run Upgrade to v2.0 (Recommended)

```bash
# Test upgrade to v2.0 without applying changes
helm upgrade scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --version 2.1.1 \
  --namespace ${NAMESPACE} \
  --reuse-values \
  --dry-run
```

## Upgrading to v2.0

!!! warning "Important Notes for v2.0 Upgrade"
    - **Pod Restart Impact**: If any application pods using the S3 buckets as filesystems are restarted during the upgrade, they will lose access to the buckets.
    Once the upgrade is complete, the application pods will automatically regain access.
    - **Mounter Strategy Change**: Version 2.0 changes the default mounter from systemd to pod-based mounter. Existing systemd mounts will continue working until pods restart.
    - **Automatic Transition**: When application pods restart after the upgrade, mounts will automatically transition to the new pod-based mounter with zero downtime.
    - **Mount-s3 Namespace**: The new pod mounter creates pods in the `mount-s3` namespace. This namespace is automatically created on first mount.

Choose one of the following upgrade options:

### Option A: Upgrade with Default Values

For installations using existing configuration:

```bash
helm upgrade scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --version 2.1.1 \
  --namespace ${NAMESPACE} \
  --reuse-values
```

### Option B: Upgrade with Custom Values

For installations with custom configuration file:

```bash
helm upgrade scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --version 2.1.1 \
  --values values-production.yaml \
  --namespace ${NAMESPACE}
```

## Post-Upgrade Verification

**Step 1. Check upgrade status:**

```bash
helm status scality-mountpoint-s3-csi-driver -n ${NAMESPACE}
```

**Step 2. Verify pods are running:**

```bash
kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
```

**Step 3. Check driver version:**

```bash
kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -o jsonpath='{.items[0].spec.containers[0].image}'
```

**Step 4. Verify v2.0 specific components:**

Check CRD installation:

```bash
kubectl get crd mountpoints3podattachments.s3.csi.scality.com
```

Check mount-s3 namespace (created on first mount):

```bash
kubectl get namespace mount-s3
```

If volumes are currently mounted, verify mounter pods:

```bash
kubectl get pods -n mount-s3
```

Check MountpointS3PodAttachment resources (if volumes are mounted):

```bash
kubectl get mountpoints3podattachments -A
# Or use the short alias
kubectl get s3pa -A
```

## Rollback (If Needed)

!!! warning
    If any application pods using the S3 buckets as filesystems are restarted during the rollback they will lose access to the buckets.
    Once the rollback is complete, the application pods will automatically regain access to the buckets.

If issues occur after upgrade, rollback to the previous version using the following steps:

```bash
# Check rollback history
helm history scality-mountpoint-s3-csi-driver -n ${NAMESPACE}

# Rollback to previous version
helm rollback scality-mountpoint-s3-csi-driver -n ${NAMESPACE}
```

## Troubleshooting

These are quick checks to verify the upgrade was successful. For detailed troubleshooting, refer to the [troubleshooting guide](../troubleshooting.md).

**Check pod status:**

The driver pod should be in a `Running` state.

```bash
kubectl describe pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
```

**Check CSI driver registration:**

```bash
kubectl get csidriver s3.csi.scality.com
```
