# Prerequisites

Before installing the Scality CSI Driver for S3, ensure the environment meets the following requirements:

## Kubernetes Requirements

**Kubernetes Version:**

- Kubernetes **v1.30.0** or newer is required. The driver relies on features and API versions available in these Kubernetes releases.

**Tools:**

- `kubectl` configured to communicate with the cluster.
- [Helm](https://helm.sh/docs/intro/install/) v3.8.0 or newer installed.
- `jq` (optional but recommended) for parsing JSON output in troubleshooting commands.

**RBAC (Role-Based Access Control):**

- The Helm chart will create the necessary `ServiceAccount`, `ClusterRole`, and `ClusterRoleBinding` for the driver to function.
- Ensure the user or tool performing the Helm installation has sufficient permissions to create these RBAC resources at the cluster scope.

## Container Image Requirements

The deployment of the Scality CSI Driver for S3 requires access to several container images. Ensure the Kubernetes cluster can pull images from the following registries:

| Component | Image | Registry | Purpose |
|-----------|-------|----------|---------|
| **Scality CSI Driver for S3** | `ghcr.io/scality/mountpoint-s3-csi-driver:2.0.1` | GitHub Container Registry (GHCR) | Main CSI driver functionality |
| **CSI Node Driver Registrar** | `ghcr.io/scality/mountpoint-s3-csi-driver/csi-node-driver-registrar:v2.14.0` | GitHub Container Registry (GHCR) | Registers CSI driver with kubelet |
| **Liveness Probe** | `ghcr.io/scality/mountpoint-s3-csi-driver/livenessprobe:v2.16.0` | GitHub Container Registry (GHCR) | Health monitoring for CSI driver pods |
| **CSI Provisioner** | `ghcr.io/scality/mountpoint-s3-csi-driver/csi-provisioner:v5.3.0` | GitHub Container Registry (GHCR) | External provisioner for CSI driver (Dynamic provisioning feature) |
| **Pause Container** | `ghcr.io/scality/mountpoint-s3-csi-driver/pause:3.10` | GitHub Container Registry (GHCR) | Headroom pods for pod mounter resource management |

!!! note "Private Registry Configuration"
    If using a private container registry or image mirroring, update the `image.repository` values in the Helm chart configuration accordingly.
    Ensure appropriate `imagePullSecrets` are configured if authentication is required.

## RING Storage Requirements

!!! note "Scality Support"
    The CSI driver is only officially supported by Scality when used with Scality RING S3.

**RING version:** RING v9.4.2 or newer is required.

**S3 Resources:** RING S3 endpoint URL is required for CSI driver installation using Helm.

**IAM Credentials:**

- IAM credentials consisting of access key ID and secret access key for an IAM entity. These credentials will be stored as a Kubernetes Secret and accessed by the driver.
- The IAM entity whose credentials are used must have appropriate permissions for the S3 operations the driver will perform.
- Optional: Session Token (required only when using temporary credentials).

!!! note "Credentials Refresh"
    The driver does not automatically refresh credentials when using session token (temporary credentials).

**Network Connectivity:**

- Kubernetes worker nodes must have network connectivity to the Scality RING S3 endpoint.
- This includes DNS resolution of the S3 endpoint hostname and network access to the S3 service on the appropriate ports
  (typically 80 for HTTP or 443 for HTTPS, unless a specific port is specified in the S3 endpoint URL).

## Next Steps

Once all prerequisites are verified and met, proceed with:

- **[Quick Start Guide](quick-start.md)** – Fast deployment for testing
- **[Installation Guide](installation-guide.md)** – Step-by-step installation with custom configuration

!!! warning "Testing vs Production"
    The [quick start guide](quick-start.md) demonstrates basic installation and is recommended for testing purposes only.
    For production deployments follow the steps outlined in the [installation guide](installation-guide.md).
