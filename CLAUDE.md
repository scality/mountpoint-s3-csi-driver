# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The Scality CSI Driver for S3 (version 1.2.0) enables Kubernetes applications to mount Scality S3 buckets as file system volumes using the Container Storage Interface (CSI) specification.
It's optimized for Scality S3 storage and uses Mountpoint for Amazon S3 binary for S3 mounting operations.

## Development Commands

### Mage Commands (Recommended for Development)

```bash
# Core development workflow
mage up                          # Build and install CSI driver from local source (default)
mage down                        # Remove CSI driver and cleanup
mage status                      # Show current installation status

# Install published versions
SCALITY_CSI_VERSION=1.2.0 mage install  # Install specific version from OCI registry

# With custom S3 endpoint
S3_ENDPOINT_URL=http://192.0.0.2:8000 mage up
S3_ENDPOINT_URL=http://192.0.0.2:8000 mage verifyStaticProvisioning

# DNS management
mage configureS3DNS              # Configure DNS mapping for s3.example.com
mage showS3DNSStatus             # Show current S3 DNS configuration and test it

# Upgrade testing
mage setupUpgradeTests           # Create test resources for upgrade testing
mage verifyUpgradeTests          # Verify provisioning after upgrade
mage cleanupUpgradeTests         # Clean up test resources

# Individual provisioning tests
mage setupStaticProvisioning     # Create static provisioning test resources
mage verifyStaticProvisioning    # Verify static provisioning works
mage cleanupStaticProvisioning   # Clean up static test resources

# Environment variables for mage commands
VERBOSE=1 mage up                # Enable verbose output
CSI_NAMESPACE=kube-system mage up # Use specific namespace
CONTAINER_TAG=dev mage up        # Use custom image tag
KIND_CLUSTER_NAME=test mage up   # Use specific kind cluster
```

### Make Commands

```bash
# Build binaries
make bin                         # Build all binaries (scality-s3-csi-driver, scality-csi-controller, scality-s3-csi-mounter, install-mp)
make container                   # Build container image (default tag: local)
make container CONTAINER_TAG=1.1.3  # Build with custom tag

# Testing
make test                        # Run unit tests with race detection and CSI compliance tests  
make unit-test                   # Run only unit tests with coverage report
make csi-compliance-test         # Run CSI sanity tests
make controller-integration-test # Run controller integration tests using envtest
make cover                       # Generate HTML coverage report

# Code quality
make fmt                         # Format Go code
make lint                        # Check Go formatting (strict validation)
make precommit                   # Run all pre-commit hooks

# License management
make check-licenses              # Verify dependency licenses against allowed list
make generate-licenses           # Generate license files for all dependencies
```

### CSI Driver Operations (Make)

```bash
# Prerequisites: Load credentials before operations
source tests/e2e/scripts/load-credentials.sh

# Installation
make csi-install S3_ENDPOINT_URL=https://s3.example.com
make csi-install S3_ENDPOINT_URL=https://s3.example.com CSI_NAMESPACE=custom-ns CSI_IMAGE_TAG=v1.14.0

# Uninstall
make csi-uninstall                      # Interactive uninstall
make csi-uninstall-clean                # Uninstall and delete custom namespace
make csi-uninstall-force                # Force uninstall

# End-to-end testing
make e2e S3_ENDPOINT_URL=https://s3.example.com        # Run tests on installed driver
make e2e-go S3_ENDPOINT_URL=https://s3.example.com     # Run only Go-based e2e tests
make e2e-verify                                        # Run verification tests only
make e2e-all S3_ENDPOINT_URL=https://s3.example.com    # Install driver and run all tests

# Documentation
make docs                        # Build and serve documentation with MkDocs
make validate-helm               # Validate Helm charts
```

### Running Single Tests

```bash
# Run specific unit test
go test -v ./pkg/driver/node/... -run TestNodePublishVolume

# Run specific e2e test with focus
cd tests/e2e && go test -v -tags=e2e -ginkgo.focus="Basic Functionality"

# Run CSI sanity test subset
go test -v ./tests/sanity/... -ginkgo.skip="Node Service"
```

## High-Level Architecture

The driver implements the CSI specification through three main components that work together to enable S3 bucket mounting:

### Core Components

1. **CSI Driver Service** (`scality-s3-csi-driver`)
   - Implements CSI Node Service RPC for volume operations
   - Runs as a DaemonSet on each Kubernetes node
   - Coordinates with mounter component for S3 operations
   - Manages credential providers (AWS profiles, K8s secrets, environment variables)

2. **Controller Component** (`scality-csi-controller`)
   - Manages volume lifecycle and provisioning operations
   - Uses controller-runtime (v0.21.0) for Kubernetes controller pattern
   - Handles storage class parameters and credential validation
   - Runs as a Deployment

3. **Mounter Component** (`scality-s3-csi-mounter`)
   - Executes and monitors mountpoint-s3 processes
   - Supports pod-based and systemd mounting strategies
   - Handles mount argument construction and process lifecycle

### Key Package Structure

- **`pkg/driver/`** - CSI driver implementation
  - `node/` - Node service with mounting logic
    - `mounter/` - Pod-based and systemd mounting strategies
    - `credentialprovider/` - AWS credential management
    - `envprovider/` - Environment variable credential provider
  - `controller/` - Controller service and credential provider
  - `storageclass/` - Storage class parameter parsing

- **`pkg/podmounter/`** - Mountpoint Pod management
  - `mppod/` - Pod creation and lifecycle management
  - `mountoptions/` - Mount options parsing and validation

- **`pkg/mountpoint/`** - Mountpoint-s3 integration
  - `runner/` - Process execution (foreground/background)
  - `mounter/` - Platform-specific mounting implementations

- **`pkg/cluster/`** - Kubernetes cluster utilities
- **`pkg/s3client/`** - S3 client operations
- **`pkg/system/`** - System utilities (pts, systemd integration)

### Mounting Strategies

The driver supports two distinct mounting approaches:

1. **Pod-based Mounting**: Creates dedicated pods for mount operations, providing better isolation and resource management
2. **Systemd Mounting**: Integrates with systemd for mount operations, suitable for system-level mounting requirements

### Credential Management

Multiple credential providers are supported for flexible authentication:

- AWS profiles from configuration files
- Kubernetes secrets for secure credential storage
- Environment variables for development and testing
- Driver-level credentials for fallback authentication

### Mage vs Make Commands

- **Mage (`mage up`)**: Best for local development - builds from source, loads image to cluster, uses local Helm chart
- **Mage (`mage install`)**: For testing published versions - uses OCI registry images and charts
- **Make**: Traditional build system, more granular control, used in CI/CD pipelines

## Key Implementation Details

- **Static Provisioning Only**: No dynamic bucket creation; buckets must exist beforehand
- **Multi-node Access**: Supports MULTI_NODE_MULTI_WRITER and MULTI_NODE_READER_ONLY access modes
- **CSI Compliance**: Implements CSI v1.11.0 specification with some limitations (skips ValidateVolumeCapabilities, SingleNodeWriter tests)
- **S3 Compatibility**: Works with any S3-compatible storage, optimized for Scality
- **Go Version**: 1.25.0 with Kubernetes API v0.33.2
- **Testing**: Comprehensive unit, integration, e2e, and CSI compliance test suites
- **DNS Mapping**: Automatically maps `s3.example.com` to actual S3 endpoint for easier configuration

## Environment Variables

Key environment variables used by the driver:

- `ACCOUNT1_ACCESS_KEY` - S3 access key for credentials
- `ACCOUNT1_SECRET_KEY` - S3 secret key for credentials  
- `S3_ENDPOINT_URL` - S3 endpoint URL for testing
- `SCALITY_CSI_VERSION` - Version for `mage install` command (required)
- `CSI_NAMESPACE` - Target Kubernetes namespace (default: `default` for mage, `kube-system` for make)
- `VERBOSE` - Enable verbose/debug output for mage commands
- `CONTAINER_TAG` - Docker image tag for local builds (default: `local`)
- `KIND_CLUSTER_NAME` - Kind cluster name for image loading
- `KUBECONFIG` - Path to Kubernetes configuration file
- `E2E_REGION` - AWS region for e2e tests (default: us-east-1)

always run make precommit before greating a git commit
