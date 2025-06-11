# Installation Guide

This guide provides comprehensive instructions for installing the Scality S3 CSI Driver in a Kubernetes cluster with production-ready configurations and security best practices.

## Prerequisites

Before starting, ensure all requirements outlined in the [Prerequisites](prerequisites.md) guide are met.

## Installation Overview


The installation process consists of:

1. Creating a namespace for the driver (recommended for production)
2. Creating S3 credentials as a Kubernetes Secret
3. Configuring and installing the Helm chart
4. Verifying the installation of the driver

## Option A: Minimal Installation (Helm)

If you need a quick deployment for development or evaluation, install the driver with the bare‑minimum Helm parameters.  
Make sure you have exported—or replace inline—the following shell variables already used later in this guide:  
`NAMESPACE`, `SECRET_NAME`, `S3_ENDPOINT_URL`, `ACCESS_KEY_ID`, and `SECRET_ACCESS_KEY`.

```bash
# 1  Create the namespace if it does not exist
kubectl create namespace ${NAMESPACE} || true

# 2  Create the credentials secret (test‑only method)
kubectl create secret generic ${SECRET_NAME} \
  --from-literal=access_key_id="${ACCESS_KEY_ID}" \
  --from-literal=secret_access_key="${SECRET_ACCESS_KEY}" \
  --namespace ${NAMESPACE}

# 3  Install the driver
helm install scality-s3-csi \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --set node.s3EndpointUrl="${S3_ENDPOINT_URL}" \
  --set s3CredentialSecret.name="${SECRET_NAME}" \
  --namespace ${NAMESPACE}
```

> **Tip**  
> Proceed to **Option B** for hardened, production‑ready settings.

---

## Option B: Detailed Installation (Helm)

**Step 1: Confirm or Create Namespace (optional):**

Creating a dedicated namespace provides better security isolation and resource management:

```bash
export NAMESPACE="scality-s3-csi"  # Or preferred namespace name
kubectl create namespace ${NAMESPACE}
```

**Step 2: Create S3 Credentials Securely:**

!!! warning "Temporary Credentials"
    The driver does not communicate with RING STS service. If session tokens are used, the credentials will not be refreshed automatically.

Set the secret name and create temporary credential files:

```bash
export SECRET_NAME="s3-credentials"  # Customize secret name if needed

# Create credential files directly (avoids shell history)
echo -n "YOUR_ACCESS_KEY_ID" > /tmp/access_key_id
echo -n "YOUR_SECRET_ACCESS_KEY" > /tmp/secret_access_key
# echo -n "YOUR_SESSION_TOKEN" > /tmp/session_token  # Only if using temporary credentials
```

Create the Kubernetes secret from files:

```bash
# Without session token
kubectl create secret generic ${SECRET_NAME} \
  --from-file=access_key_id=/tmp/access_key_id \
  --from-file=secret_access_key=/tmp/secret_access_key \
  --namespace ${NAMESPACE}

# OR with session token (if needed)
# kubectl create secret generic ${SECRET_NAME} \
#   --from-file=access_key_id=/tmp/access_key_id \
#   --from-file=secret_access_key=/tmp/secret_access_key \
#   --from-file=session_token=/tmp/session_token \
#   --namespace ${NAMESPACE}
```

**Important**: Clean up temporary files immediately:

```bash
rm -f /tmp/access_key_id /tmp/secret_access_key /tmp/session_token
```

**Step 3: Create Custom Values File:**

Create a `values-production.yaml` file with your configuration:

```yaml
# values-production.yaml
node:
  # REQUIRED: Scality RING S3 endpoint URL
  # For S3 endpoint URL, port number can be added if needed; example: `http://s3.example.com:8000`
  # Port number can be omitted for default port `80` for HTTP or `443` for HTTPS
  s3EndpointUrl: "https://s3.example.com"  # Replace with your actual endpoint


  # Optional: Default AWS region for S3 requests
  # Can be overridden per-volume using PersistentVolume mountOptions
  # Must match the region configured in your RING setup
  s3Region: "us-east-1"  # Adjust based on your RING configuration, default is `us-east-1`

  # Resource limits for the CSI node DaemonSet pods (one pod per worker node)
  # These apply to the main s3-plugin container that handles volume mount operations
  # resources:
  #   requests:
  #     cpu: 50m        # Baseline CPU needed for volume operations
  #     memory: 128Mi   # Memory for caching and S3 operations
  #   limits:
  #     cpu: 200m       # Maximum CPU during heavy I/O
  #     memory: 256Mi   # Memory limit to prevent resource contention

  # Node selector for the CSI node DaemonSet - controls which nodes run the driver
  # The driver MUST run on every node where you want to mount S3 volumes
  # nodeSelector:
  #   node-role.kubernetes.io/worker: "true"     # Only run on worker nodes
  #   storage-enabled: "true"                    # Only on storage-capable nodes

  # Tolerations for the CSI node DaemonSet - allows driver to run on tainted nodes
  # Essential if you have tainted nodes where S3 volumes should be mountable
  # tolerations:
  # - key: "node-role.kubernetes.io/control-plane"  # Allow on control plane nodes
  #   effect: "NoSchedule"
  # - key: "dedicated"                               # Allow on dedicated storage nodes
  #   operator: "Equal"
  #   value: "storage"
  #   effect: "NoSchedule"

s3CredentialSecret:
  # Reference the Kubernetes Secret containing S3 credentials created in Step 2
  # This secret must exist in the same namespace as the driver installation
  name: "s3-credentials"  # Must match the SECRET_NAME from Step 2

# Sidecar container resources - these run alongside the main s3-plugin in each node pod
# Each node pod contains: s3-plugin (main) + 2 sidecars (node-driver-registrar & livenessprobe)
sidecars:
  nodeDriverRegistrar:
    # Registers the CSI driver with kubelet on each node - required for volume operations
    resources:
      requests:
        cpu: 10m        # Minimal CPU for registration operations
        memory: 40Mi    # Memory for kubelet communication
      limits:
        cpu: 50m        # Burst CPU for initial registration
        memory: 100Mi   # Memory limit for registration process
  livenessProbe:
    # Monitors health of the CSI driver and reports to Kubernetes
    resources:
      requests:
        cpu: 10m        # Minimal CPU for health checks
        memory: 40Mi    # Memory for probe operations
      limits:
        cpu: 50m        # Burst CPU during health checks
        memory: 100Mi   # Memory limit for probe container
```

For a complete list of configurable parameters, see the [Helm Chart Configuration Reference](../concepts-and-reference/helm-chart-configuration-reference.md) reference.

**Step 4: Install the Helm Chart:**

Install the driver using the custom values file:

```bash
helm install scality-s3-csi \
  oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
  --values values-production.yaml \
  --namespace ${NAMESPACE}
```


## Verification

### Step 1: Check Driver Pods

Verify that the driver pods are running:

```bash
kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
```

Expected output: One `s3-csi-node-*` pod per eligible worker node, all in `Running` state.

### Step 2: Verify CSI Driver Registration

```bash
kubectl get csidriver s3.csi.scality.com
```

Expected output should show the driver with `attachRequired: false`.

### Step 3: Check Driver Logs (Optional)

To troubleshoot or verify operation:

```bash
# Get logs from a node plugin pod
kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -c s3-plugin
```

## Advanced Configuration

### Node Selectors and Tolerations

To control where the driver pods run:

```yaml
# values-production.yaml
node:
  nodeSelector:
    node-role.kubernetes.io/worker: "true"
    storage-type: "ssd"

  tolerations:
  - key: "dedicated"
    operator: "Equal"
    value: "storage"
    effect: "NoSchedule"
```

### Resource Limits

For production workloads, adjust resource limits based on your requirements:

```yaml
node:
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi
```

## Security Considerations

<!-- markdownlint-disable MD046 -->
!!! important "Production Security Checklist"
    - ✅ Use dedicated namespace with appropriate RBAC
    - ✅ Create secrets from files, not command line arguments
    - ✅ Delete temporary credential files immediately after use
    - ✅ Implement least-privilege IAM policies
    - ✅ Enable audit logging for cluster operations
    - ✅ Consider using IAM roles for service accounts (IRSA) where supported
    - ✅ Regularly rotate credentials
    - ✅ Use TLS/HTTPS for S3 endpoints
<!-- markdownlint-enable MD046 -->

## Uninstallation

!!! note "If Volumes Were Provisioned"
    If any applications (Kubernetes pods) were using PersistentVolumes or PersistentVolumeClaims provisioned using the S3 CSI driver,
    follow the complete [uninstallation guide](uninstallation.md) to properly clean up all resources.

If no volumes were provisioned, you can uninstall the driver with these simple steps:

These steps assume that environment variables, `NAMESPACE` and `SECRET_NAME` are set per step 2 of this guide.

**Step 1. Uninstall the Helm release:**

```bash
helm uninstall scality-s3-csi -n ${NAMESPACE}
```

**Step 2. Delete the S3 credentials secret:**

```bash
kubectl delete secret ${SECRET_NAME} -n ${NAMESPACE}
```

**Step 3. Delete the namespace (if created):**

```bash
# Only if you created a dedicated namespace and want to remove it
kubectl delete namespace ${NAMESPACE}
```

**Step 4. Verify removal:**

- Check that CSI driver is removed

    ```bash
    kubectl get csidriver s3.csi.scality.com
    ```

- Check that no driver pods remain

    ```bash
    kubectl get pods --all-namespaces -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
    ```

## Next Steps

**Volume Provisioning**: See the [volume provisioning guides](../volume-provisioning/prerequisites.md) to learn how to use S3 buckets with your applications.
