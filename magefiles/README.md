# Mage Targets for CSI Driver Development

Simple build targets for CSI driver development and testing.

## Core Commands

### Command Comparison

| Command | Purpose | Source | Image | Chart | Environment Variables |
|---------|---------|--------|-------|-------|----------------------|
| **`mage up`** | Local development | Local source code | Builds locally (`local` tag) | Uses local Helm chart | Optional: `S3_ENDPOINT_URL`, `CSI_NAMESPACE`, `CONTAINER_TAG`, `KIND_CLUSTER_NAME`, `VERBOSE` |
| **`mage install`** | Production/testing | OCI registry | Uses published images | Downloads from registry | Required: `SCALITY_CSI_VERSION`<br>Optional: `S3_ENDPOINT_URL`, `CSI_NAMESPACE`, `VERBOSE` |
| **`mage down`** | Cleanup | - | - | - | Optional: `CSI_NAMESPACE` |
| **`mage status`** | Check status | - | - | - | Optional: `CSI_NAMESPACE` |

### Quick Reference

- **`mage up`** - Build and install CSI driver from local source
- **`mage install`** - Install specific CSI version from OCI registry (requires `SCALITY_CSI_VERSION`)
- **`mage down`** - Remove CSI driver and cleanup (works for both up and install)
- **`mage status`** - Show current installation status

### When to Use Each Command

| Use Case | Command | Example | Environment Variables |
|----------|---------|---------|----------------------|
| Developing new features | `mage up` | `mage up` | None required |
| Testing local changes | `mage up` | `S3_ENDPOINT_URL=http://192.0.0.2:8000 mage up` | `S3_ENDPOINT_URL` |
| Installing stable version | `mage install` | `SCALITY_CSI_VERSION=1.2.0 mage install` | `SCALITY_CSI_VERSION` (required) |
| Testing upgrades | `mage install` → `mage up` | See [Upgrade Testing](#upgrade-testing) | `SCALITY_CSI_VERSION` then optional |
| Switching versions | `mage install` | `SCALITY_CSI_VERSION=1.1.0 mage install` | `SCALITY_CSI_VERSION` (required) |
| Cleanup (any method) | `mage down` | `mage down` | None required |

## Usage

### Local Development Workflow

```bash
# Basic workflow
mage up      # Install everything
mage status  # Check status
mage down    # Remove everything

# With custom S3 endpoint
S3_ENDPOINT_URL=http://192.0.0.2:8000 mage up

# With custom namespace
CSI_NAMESPACE=kube-system mage up

# With verbose/debug output
VERBOSE=1 mage up
```

### Installing Published Versions

```bash
# Install specific version from OCI registry (REQUIRED)
SCALITY_CSI_VERSION=1.2.0 mage install

# Install different version
SCALITY_CSI_VERSION=1.1.0 mage install

# Check status
mage status

# Remove (same command works for both up and install)
mage down

# ERROR: Version required
mage install  # Will error with usage instructions
```

## Command Details

### What Each Command Does

| Step | `mage up` | `mage install` | Environment Variables Used |
|------|-----------|----------------|----------------------------|
| 1 | Loads credentials from `tests/e2e/integration_config.json` | Loads credentials from `tests/e2e/integration_config.json` | None |
| 2 | **Builds container image** with tag `local` | - | `CONTAINER_TAG` |
| 3 | **Loads image to cluster** (kind/minikube) | - | `CONTAINER_TAG`, `KIND_CLUSTER_NAME` |
| 4 | Configures DNS mapping: `s3.example.com` → S3 endpoint | Configures DNS mapping: `s3.example.com` → S3 endpoint | `S3_ENDPOINT_URL` or `S3_HOST`/`S3_PORT`/`S3_HTTPS` |
| 5 | Creates Kubernetes secret with S3 credentials | Creates Kubernetes secret with S3 credentials | `CSI_NAMESPACE` |
| 6 | Installs CSI driver via Helm using **local chart** | Installs CSI driver via Helm from **OCI registry** | `CSI_NAMESPACE`, `VERBOSE` / `SCALITY_CSI_VERSION` (required) |
| 7 | Uses **locally built image** | Uses **published images** from specified version | - |
| 8 | Verifies pods are ready | Verifies pods are ready | `CSI_NAMESPACE` |

## Common Workflows

### Upgrade Testing

```bash
# 1. Install old version from registry
SCALITY_CSI_VERSION=1.2.0 mage install

# 2. Verify installation
mage status

# 3. Build and upgrade to local development version
mage up  # Builds image and upgrades to local chart

# 4. Clean up (works regardless of installation method)
mage down
```

### Version Switching

```bash
# Install v1.1.0 from registry
SCALITY_CSI_VERSION=1.1.0 mage install

# Switch to v1.2.0 from registry  
SCALITY_CSI_VERSION=1.2.0 mage install

# Switch to local development build
mage up

# Clean up (always the same)
mage down
```

## Environment Variables

### Environment Variables Reference

| Variable | Description | Default | Used By | Required |
|----------|-------------|---------|---------|----------|
| `SCALITY_CSI_VERSION` | Chart version to install from OCI registry | None | `mage install` | **Yes** (for install) |
| `CSI_NAMESPACE` | Target Kubernetes namespace | `default` | All commands | No |
| `S3_ENDPOINT_URL` | Complete S3 endpoint URL (e.g., `http://192.0.0.2:8000`) | `http://localhost:8000` | `mage up`, `mage install` | No |
| `S3_HOST` | S3 host/IP (alternative to `S3_ENDPOINT_URL`) | `localhost` | `mage up`, `mage install` | No |
| `S3_PORT` | S3 port (used with `S3_HOST`) | `8000` | `mage up`, `mage install` | No |
| `S3_HTTPS` | Use HTTPS if set to `true` (used with `S3_HOST`) | `false` | `mage up`, `mage install` | No |
| `CONTAINER_TAG` | Docker image tag for local builds | `local` | `mage up` | No |
| `CONTAINER_IMAGE` | Custom container image name | `ghcr.io/scality/mountpoint-s3-csi-driver` | `mage up` | No |
| `KIND_CLUSTER_NAME` | Kind cluster name for image loading | `kind` (default cluster) | `mage up` | No |
| `VERBOSE` | Enable verbose/debug output | None | All commands | No |

### S3 Configuration (choose one approach)

#### Option 1: Complete URL

- `S3_ENDPOINT_URL` - Complete S3 endpoint URL (e.g., "<http://192.168.1.100:8000>")

#### Option 2: Component-based

- `S3_HOST` - S3 host/IP (default: "localhost")
- `S3_PORT` - S3 port (default: "8000")
- `S3_HTTPS` - Use HTTPS if "true" (default: HTTP)

## DNS Mapping Feature

The CSI driver uses `[PROTOCOL]://s3.example.com:[PORT]` as its endpoint (matching your S3 config). Mage automatically:

- Maps `s3.example.com` to your actual S3 service IP via CoreDNS
- Uses the configured protocol (HTTP/HTTPS) and port from your S3 configuration
- Allows changing S3 endpoints without reinstalling the CSI driver
- Run `mage configureS3DNS` to update mapping, `mage showS3DNSStatus` to check

**Note**: For CloudServer, add `"s3.example.com": "us-east-1"` to the `restEndpoints` configuration:

```json
"restEndpoints": {
    "localhost": "us-east-1",
    "127.0.0.1": "us-east-1",
    "s3.example.com": "us-east-1"
}
```

## Individual Steps (for debugging)

**Note**: Use `mage up` or `mage install` for normal workflow. Individual steps are for debugging only.

### Debugging Commands

| Command | Purpose | Used By | Environment Variables |
|---------|---------|---------|----------------------|
| `mage buildImage` | Build container image | `mage up` only | `CONTAINER_TAG` |
| `mage loadImageToCluster` | Load image to cluster (kind/minikube) | `mage up` only | `CONTAINER_TAG`, `KIND_CLUSTER_NAME` |
| `mage loadCredentials` | Load credentials from `integration_config.json` | Both | None |
| `mage createSecret` | Create K8s secret with S3 credentials | Both | `CSI_NAMESPACE` |
| `mage installCSI` | Install via Helm using local chart | `mage up` only | `CSI_NAMESPACE`, `S3_ENDPOINT_URL`, `VERBOSE` |
| `mage installCSIWithVersion` | Install via Helm from OCI registry | `mage install` only | `SCALITY_CSI_VERSION` (required), `CSI_NAMESPACE`, `S3_ENDPOINT_URL`, `VERBOSE` |
| `mage uninstallCSI` | Remove Helm release | `mage down` | `CSI_NAMESPACE` |
| `mage removeSecret` | Delete K8s secret | `mage down` | `CSI_NAMESPACE` |
| `mage configureS3DNS` | Configure DNS mapping for s3.example.com | Both | `S3_ENDPOINT_URL` or `S3_HOST`+`S3_PORT`+`S3_HTTPS` |
| `mage removeS3DNS` | Remove DNS mapping only | `mage down` | None |
| `mage showS3DNSStatus` | Show current DNS config and test resolution | Standalone | None |

### Example Debug Usage

```bash
# Manually configure DNS mapping
S3_ENDPOINT_URL=http://192.0.0.2:8000 mage configureS3DNS

# Check DNS status
mage showS3DNSStatus

# Build and load image without installing
mage buildImage
mage loadImageToCluster

# Load image to specific Kind cluster
KIND_CLUSTER_NAME=helm-test-cluster mage loadImageToCluster

# Create secret manually
mage createSecret
```

## Prerequisites

- kind or minikube cluster running
- kubectl configured
- helm installed
- Docker running
- Credentials in `tests/e2e/integration_config.json`
