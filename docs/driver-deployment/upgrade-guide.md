# Upgrade Guide

This guide provides instructions for upgrading the Scality S3 CSI Driver to newer versions.

## Prerequisites

Before upgrading, ensure all requirements outlined in the **[Prerequisites](prerequisites.md)** guide are met for the target version.

## Pre-Upgrade Steps

**Step 1. Set namespace variable:**

Set the namespace where the driver is currently installed:

```bash
export NAMESPACE="scality-s3-csi"  # Replace with your actual namespace
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

**Step 4. Dry run upgrade (recommended):**

```bash
# Test upgrade without applying changes
helm upgrade scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --namespace ${NAMESPACE} \
  --reuse-values \
  --dry-run
```

## Upgrade Options

!!! warning
    If any application pods using the S3 buckets as filesystems are restarted during the upgrade they will lose access to the buckets.
    Once the upgrade is complete, the application pods will automatically regain access to the buckets.

Choose one of the following upgrade options:

### Option A: Upgrade with Default Values

For installations using existing configuration:

```bash
helm upgrade scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --namespace ${NAMESPACE} \
  --reuse-values
```

### Option B: Upgrade with Custom Values

For installations with custom configuration file:

```bash
helm upgrade scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
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

## Rollback (If Needed)

!!! warning
    If any application pods using the S3 buckets as filesystems are restarted during the rollback they will lose access to the buckets.
    Once the rollback is complete, the application pods will automatically regain access to the buckets.

If issues occur after upgrade you can rollback to the previous version using the following steps:

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
