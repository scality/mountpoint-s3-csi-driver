# Prerequisites

Before installing the Scality S3 CSI Driver, ensure the environment meets the following requirements:

## Kubernetes Requirements

### Kubernetes Version

- Kubernetes **v1.30.0** or newer is required. The driver relies on features and API versions available in these Kubernetes releases.

### Tools

- `kubectl` configured to communicate with the cluster.
- [Helm](https://helm.sh/docs/intro/install/) v3 installed.

### RBAC (Role-Based Access Control)

- The Helm chart will create the necessary `ServiceAccount`, `ClusterRole`, and `ClusterRoleBinding` for the driver to function.
- Ensure the user or tool performing the Helm installation has sufficient permissions to create these RBAC resources at the cluster scope.

## RING Storage Requirements

RING version 9.4 or newer is required.

### S3 Resources

- S3 endpoint URL is required for CSI driver installation using Helm.

### IAM Credentials

- IAM credentials consisting of access key ID and secret access key for an IAM entity. These credentials will be stored as a Kubernetes Secret and accessed by the driver.
- The IAM entity whose credentials are used must have appropriate permissions. See [this document](../permissions.md) for detailed permission requirements.
- Optional: Session Token (required only when using temporary credentials).

!!! note "Credentials Refresh"
    The driver does not automatically refresh credentials when using session token (temporary credentials).

### Network Connectivity

- Kubernetes worker nodes must have network connectivity to the Scality S3 endpoint (RING).
- This includes DNS resolution of the S3 endpoint hostname and network access to the S3 service on the appropriate ports
  (typically 80 for HTTP or 443 for HTTPS, unless a specific port is specified in the S3 endpoint URL).

## Container Image Requirements

The CSI driver deployment requires access to several container images. Ensure the Kubernetes cluster can pull images from the following registries:

| Component | Image | Registry | Purpose |
|-----------|-------|----------|---------|
| **Scality S3 CSI Driver** | `ghcr.io/scality/mountpoint-s3-csi-driver:1.0.0` | GitHub Container Registry (GHCR) | Main CSI driver functionality |
| **CSI Node Driver Registrar** | `ghcr.io/scality/mountpoint-s3-csi-driver/csi-node-driver-registrar:v2.14.0` | GitHub Container Registry (GHCR) | Registers CSI driver with kubelet |
| **Liveness Probe** | `ghcr.io/scality/mountpoint-s3-csi-driver/livenessprobe:v2.15.0` | GitHub Container Registry (GHCR) | Health monitoring for CSI driver pods |

!!! note "Private Registry Configuration"
    If using a private container registry or image mirroring, update the `image.repository` values in the Helm chart configuration accordingly.
    Ensure appropriate `imagePullSecrets` are configured if authentication is required.

## Next Steps

Once all prerequisites are verified and met, proceed with:

- **[Quick Start Guide](quick-start.md)** – Fast deployment for testing
- **[Detailed Installation](detailed-installation.md)** – Step-by-step installation with custom configuration

<!-- markdownlint-disable MD046 -->
!!! warning "Testing vs Production"
    The quick start guide demonstrates basic credential handling for testing purposes. Be aware that:

    - Environment variables expose credentials in shell history and process lists
    - Commands with credentials are visible to other users via `ps` commands
    - The driver by default uses the `kube-system` namespace which has elevated privileges
    - Always follow production security practices for real deployments.
<!-- markdownlint-enable MD046 -->
