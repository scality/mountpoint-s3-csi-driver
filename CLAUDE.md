# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The Scality S3 CSI Driver is a Kubernetes Container Storage Interface (CSI) driver that enables mounting S3-compatible storage buckets as persistent volumes in Kubernetes clusters. It's a fork of the Mountpoint for Amazon S3 CSI driver, optimized specifically for Scality storage solutions.

## Key Commands

### Building and Testing
- `make bin` - Build all driver binaries (scality-s3-csi-driver, scality-csi-controller, scality-s3-csi-mounter, install-mp)
- `make test` - Run unit tests with race detection and CSI compliance tests
- `make unit-test` - Run only unit tests with coverage
- `make csi-compliance-test` - Run CSI sanity tests (skips controller capabilities)
- `make controller-integration-test` - Run controller integration tests with envtest
- `make lint` - Check Go formatting
- `make fmt` - Format Go code
- `make precommit` - Run pre-commit hooks for all files

### Documentation
- `make docs` - Build and serve documentation using MkDocs (requires Python virtual environment)
- `make docs-clean` - Clean documentation build artifacts

### CSI Driver Operations
- `make csi-install S3_ENDPOINT_URL=<url>` - Install CSI driver (requires credentials loaded)
- `make csi-uninstall` - Uninstall CSI driver interactively
- `make e2e S3_ENDPOINT_URL=<url>` - Run end-to-end tests on installed driver
- `make e2e-all S3_ENDPOINT_URL=<url>` - Install driver and run all tests

### Prerequisites for CSI Operations
Before running CSI driver commands, load credentials:
```bash
source tests/e2e/scripts/load-credentials.sh
```

## Architecture

### Core Components
- **Driver (`pkg/driver/`)**: Main CSI driver implementation with identity, controller, and node services
- **Node Service (`pkg/driver/node/`)**: Handles volume mounting/unmounting operations
  - **Mounter (`pkg/driver/node/mounter/`)**: Manages different mounting strategies (pod-based, systemd)
  - **Credential Provider (`pkg/driver/node/credentialprovider/`)**: Handles AWS credentials from secrets or profiles
- **Pod Mounter (`pkg/podmounter/`)**: Manages mountpoint-s3 pods for volume mounting
- **Cluster Management (`pkg/cluster/`)**: Kubernetes cluster interaction utilities

### Binaries
- `scality-s3-csi-driver` - Main CSI driver daemon (node service)
- `scality-csi-controller` - CSI controller service (runs as deployment)
- `scality-s3-csi-mounter` - Dedicated mounter process
- `install-mp` - Mountpoint-s3 installer utility

### Key Design Patterns
- Static provisioning only (no dynamic bucket creation)
- Uses mountpoint-s3 binary for actual S3 mounting
- Supports both systemd and pod-based mounting strategies
- Credential management through Kubernetes secrets or AWS profiles

## Testing Strategy

### Test Structure
- `tests/sanity/` - CSI specification compliance tests
- `tests/controller/` - Controller integration tests using envtest
- `tests/e2e/` - End-to-end tests against real S3 storage
- Unit tests are co-located with source files (`*_test.go`)

### E2E Testing
E2E tests require a running Kubernetes cluster and S3-compatible storage. Tests validate:
- Volume provisioning and mounting
- Multi-pod access patterns
- Credential handling
- Mount options and configurations
- Performance characteristics

## Development Notes

### Go Version
- Uses Go 1.24.5
- Kubernetes API version: v0.33.2
- CSI spec version: v1.11.0

### Key Dependencies
- `github.com/aws/aws-sdk-go-v2` - AWS SDK for credential and S3 operations
- `github.com/container-storage-interface/spec` - CSI specification
- `k8s.io/client-go` - Kubernetes client libraries
- `sigs.k8s.io/controller-runtime` - Controller runtime for Kubernetes operators

### License Management
The project uses `go-licenses` tool to manage dependency licenses:
- `make check-licenses` - Verify all dependencies use allowed licenses
- `make generate-licenses` - Generate license files for all dependencies
- Allowed licenses: Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC, MIT

### Helm Chart
Helm chart is located in `charts/scality-mountpoint-s3-csi-driver/` and can be validated with:
```bash
make validate-helm
```