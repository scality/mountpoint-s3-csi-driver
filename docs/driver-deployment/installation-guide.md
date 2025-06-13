# Installation Guide

This guide provides comprehensive instructions for installing the Scality S3 CSI Driver in a Kubernetes cluster with production-ready configurations and security best practices.

## Prerequisites

Before starting, ensure all requirements outlined in the [Prerequisites](prerequisites.md) guide are met.

## Installation Overview

The installation process consists of:

1. Setting configuration variables
2. Creating a namespace for the driver (recommended for production)
3. Creating S3 credentials as a Kubernetes Secret
4. Configuring and installing the Helm chart
5. Checking the installation of the driver

## Step 1. Set Configuration Variables

- Set the namespace in which the s3 credentials secret will be created and the driver will be deployed. Replace `scality-s3-csi` with the preferred namespace name.

    ```bash
    export NAMESPACE="scality-s3-csi"
    ```

- Set the secret name in which the s3 credentials will be stored. Replace `s3-secret` with the preferred secret name.

    ```bash
    export SECRET_NAME="s3-secret"
    ```

- Set the access key ID. Replace `YOUR_ACCESS_KEY_ID` with the actual access key ID.

    ```bash
    export ACCESS_KEY_ID="YOUR_ACCESS_KEY_ID"
    ```

- Set the secret access key. Replace `YOUR_SECRET_ACCESS_KEY` with the actual secret access key.

    <!-- markdownlint-disable MD046 -->
    !!! note
        To avoid storing sensitive credentials in your shell history, history can be temporarily disabled before running commands with sensitive information:

        ```bash
        set +o history # temporarily turn off history

        # export SECRET_ACCESS_KEY=

        set -o history # turn it back on
        ```
    <!-- markdownlint-enable MD046 -->

    ```bash
    export SECRET_ACCESS_KEY="YOUR_SECRET_ACCESS_KEY"
    ```

- Set the session token (optional). Replace `YOUR_SESSION_TOKEN` with the actual session token. The driver does not communicate with RING STS service to refresh the session token.

    ```bash
    # export SESSION_TOKEN="YOUR_SESSION_TOKEN"
    ```

## Step 2. Create Namespace

Creating a dedicated namespace provides better security isolation and resource management:

```bash
kubectl create namespace ${NAMESPACE}
```

## Step 3. Create S3 Credentials Secret

```bash
kubectl create secret generic ${SECRET_NAME} \
  --from-literal=access_key_id="${ACCESS_KEY_ID}" \
  --from-literal=secret_access_key="${SECRET_ACCESS_KEY}" \
  --namespace ${NAMESPACE}
```

!!! warning "Temporary Credentials"
    The driver does not communicate with RING S3 Connector's STS service. If session tokens are used, the credentials will not be refreshed automatically.

OR with session token (if needed):

```bash
kubectl create secret generic ${SECRET_NAME} \
  --from-literal=access_key_id="${ACCESS_KEY_ID}" \
  --from-literal=secret_access_key="${SECRET_ACCESS_KEY}" \
  --from-literal=session_token="${SESSION_TOKEN}" \
  --namespace ${NAMESPACE}
```

---

## Step 4. Install the Driver

Choose one of the following installation options:

### Option A: Minimal Installation

!!! note "S3 Endpoint URL"
    For S3 endpoint URL, port number can be added if needed; example: `http://s3.example.com:8000`
    Port number can be omitted for default port `80` for HTTP or `443` for HTTPS

**Set the S3 endpoint URL:**

Replace `https://s3.example.com` with the actual RING S3 endpoint URL.

```bash
export S3_ENDPOINT_URL="https://s3.example.com"
```

**Install the Helm Chart:**

Deploy the driver with minimal configuration.

```bash
helm install scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --set node.s3EndpointUrl="${S3_ENDPOINT_URL}" \
  --set s3CredentialSecret.name="${SECRET_NAME}" \
  --namespace ${NAMESPACE}
```

### Option B: Advanced Installation

For environments requiring custom configuration:

**Create Custom Values File:**

Create a `values-production.yaml` file with preferred configuration.

```yaml
# values-production.yaml
node:
  # REQUIRED: Scality RING S3 endpoint URL
  # For S3 endpoint URL, port number can be added if needed; example: `http://s3.example.com:8000`
  # Port number can be omitted for default port `80` for HTTP or `443` for HTTPS
  s3EndpointUrl: "https://s3.example.com"  # Replace with the actual endpoint

  # Optional: Default AWS region for S3 requests
  # Can be overridden per-volume at PersistentVolume level.
  # Must match the region configured in the RING setup
  s3Region: "us-east-1"  # Adjust based on the RING configuration, default is `us-east-1`

  # Optional: Log verbosity level for the CSI driver (higher numbers = more verbose)
  # Default is 4.
  # 1-2: Basic operational info (recommended for production)
  # 3: Credential authentication info
  # 4: All CSI operations and mount details (default)
  # 5: Very detailed debug info (systemd signals, mount-s3 output)
  logLevel: 2

  # Resource limits for the CSI node DaemonSet pods (one pod per worker node)
  # These apply to the main s3-plugin container that handles volume mount operations
  # https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
  # resources:

  # Node selector for the CSI node DaemonSet - controls which nodes run the driver
  # nodeSelector:
  #   node-role.kubernetes.io/worker: "true"     # Only run on worker nodes
  #
  # Tolerations for the CSI node DaemonSet - allows driver to run on tainted nodes
  # https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
  # tolerations: []

s3CredentialSecret:
  # Reference the Kubernetes Secret containing S3 credentials created in Step 2
  # This secret must exist in the same namespace as the driver installation
  name: "s3-secret"

# Sidecar container resources - these run alongside the main s3-plugin in each node pod
# Each node pod contains: s3-plugin (main) + 2 sidecars (node-driver-registrar & livenessprobe)
# Resources are not set by default and are inherited from the node.resources configuration.
# sidecars:
  # nodeDriverRegistrar:
  #   # Registers the CSI driver with kubelet on each node - required for volume operations
  #   resources:
  # livenessProbe:
  #   # Monitors health of the CSI driver and reports to Kubernetes
  #   resources:
```

For a complete list of configurable parameters, see the [Helm Chart Configuration Reference](../concepts-and-reference/helm-chart-configuration-reference.md) reference.

**Install the Helm Chart:**

Deploy the driver using the custom values file.

```bash
helm install scality-mountpoint-s3-csi-driver \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --values values-production.yaml \
  --namespace ${NAMESPACE}
```

## Step 5. Verification

### Check Driver Pods

Check that the driver pods are running:

```bash
kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
```

Expected output: One `s3-csi-node-*` pod per eligible worker node, all in `Running` state.

### Check CSI Driver Registration

```bash
kubectl get csidriver s3.csi.scality.com
```

### Check Driver Logs (Optional)

To troubleshoot or check driver operation:

```bash
# Get logs from a node plugin pod
kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -c s3-plugin
```

You should see the following output:

```bash
Using systemd mounter
Listening for connections on address: &net.UnixAddr{Name:"/csi/csi.sock", Net:"unix"}
NodeGetInfo: called with args {}
```

## Uninstallation

!!! note "If Volumes Were Provisioned"
    If any applications (Kubernetes pods) were using PersistentVolumes or PersistentVolumeClaims provisioned using the S3 CSI driver,
    follow the complete [uninstallation guide](uninstallation.md) to properly clean up all resources.

If no volumes were provisioned, you can uninstall the driver with these simple steps:

These steps assume that environment variables, `NAMESPACE` and `SECRET_NAME` are set per the [installation steps above](#step-1-set-configuration-variables).

**Step 1. Uninstall the Helm release:**

```bash
helm uninstall scality-mountpoint-s3-csi-driver -n ${NAMESPACE}
```

**Step 2. Delete the S3 credentials secret:**

```bash
kubectl delete secret ${SECRET_NAME} -n ${NAMESPACE}
```

**Step 3. Delete the namespace (if created):**

```bash
kubectl delete namespace ${NAMESPACE}  # Only if you created a dedicated namespace
```

**Step 4. Check removal:**

Check that CSI driver is removed:

```bash
kubectl get csidriver s3.csi.scality.com
```

Check that no driver pods remain:

```bash
kubectl get pods --all-namespaces -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
```

## Next Steps

**Volume Provisioning**: See the [volume provisioning guides](../volume-provisioning/static-provisioning/overview.md) to learn how to use S3 buckets as volumes with kubernetes applications.
