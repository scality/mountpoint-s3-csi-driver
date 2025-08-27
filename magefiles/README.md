# Mage Targets for CSI Driver Development

Simple build targets for CSI driver development and testing.

## Core Commands

- **`mage up`** - Build and install CSI driver from local source
- **`mage down`** - Remove CSI driver and cleanup
- **`mage status`** - Show current installation status

## Usage

```bash
# Basic workflow
mage up      # Install everything
mage status  # Check status
mage down    # Remove everything

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
4. Configures DNS mapping: `s3.example.com` → S3 endpoint IP (from S3 config)
5. Creates Kubernetes secret with S3 credentials
6. Installs CSI driver via Helm (uses `[PROTOCOL]://s3.example.com:[PORT]`)
7. Verifies pods are ready

## Environment Variables

- `CSI_NAMESPACE` - Target namespace (default: "default")
- `CONTAINER_TAG` - Image tag (default: "local")  
- `VERBOSE` - Enable verbose/debug output (set to "1" or any value)

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

**Note**: Use `mage up` for normal workflow. Individual steps are for debugging only.

```bash
mage buildImage          # Build container image
mage loadImageToCluster  # Load image to cluster  
mage createSecret        # Create K8s secret (auto-loads credentials)
mage installCSI          # Install via Helm
mage uninstallCSI        # Remove Helm release
mage removeSecret        # Delete secret
mage configureS3DNS      # Configure S3 DNS mapping (use S3_ENDPOINT_URL=http://<IP>:<PORT> or S3_HOST=<IP> S3_PORT=<PORT>)
mage removeS3DNS         # Remove S3 DNS mapping only
mage showS3DNSStatus     # Show current S3 DNS configuration and test resolution
```

## Prerequisites

- kind or minikube cluster running
- kubectl configured
- helm installed
- Docker running
- Credentials in `tests/e2e/integration_config.json`
