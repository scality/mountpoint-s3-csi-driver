# Mage Targets for CSI Driver Development

Simple build targets for CSI driver development and testing.

## Core Commands

- **`mage up`** - Build and install CSI driver from local source
- **`mage down`** - Remove CSI driver and cleanup
- **`mage status`** - Show current installation status

## Usage

```bash
# Basic workflow
mage up                                    # Install everything
mage status                               # Check status  
mage down                                # Remove everything

# With custom S3 endpoint
S3_ENDPOINT_URL=http://s3.example.com:8000 mage up

# With custom namespace
CSI_NAMESPACE=kube-system mage up

# With verbose/debug output
VERBOSE=1 mage up
```

## What `mage up` does

1. Loads credentials from `tests/e2e/integration_config.json`
2. Builds container image with tag `local`
3. Loads image to cluster (auto-detects kind/minikube)
4. Creates Kubernetes secret with S3 credentials
5. Installs CSI driver via Helm chart

## Environment Variables

- `CSI_NAMESPACE` - Target namespace (default: "default")
- `S3_ENDPOINT_URL` - S3 endpoint (default: "http://localhost:8000")
- `CONTAINER_TAG` - Image tag (default: "local")
- `VERBOSE` - Enable verbose/debug output (set to "1" or any value)

## Individual Steps (for debugging)

**Note**: Use `mage up` for normal workflow. Individual steps are for debugging only.

```bash
mage buildImage          # Build container image
mage loadImageToCluster  # Load image to cluster  
mage createSecret        # Create K8s secret (auto-loads credentials)
mage installCSI          # Install via Helm
mage uninstallCSI        # Remove Helm release
mage removeSecret        # Delete secret
```

## Prerequisites

- kind or minikube cluster running
- kubectl configured
- helm installed
- Docker running
- Credentials in `tests/e2e/integration_config.json`