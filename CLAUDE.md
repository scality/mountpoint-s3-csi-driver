# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This repository contains the Scality CSI Driver for S3 (version 1.2.0), a Container Storage Interface (CSI) driver that enables Kubernetes applications to mount Scality S3 buckets as file system volumes. It's forked from the AWS Mountpoint for Amazon S3 CSI Driver and optimized for Scality S3 storage.

The driver implements the CSI specification and uses the Mountpoint for Amazon S3 binary underneath for actual S3 mounting operations.

## Key Commands

### Building and Testing
- `make bin` - Build all binaries (scality-s3-csi-driver, scality-csi-controller, scality-s3-csi-mounter, install-mp)
- `make container` - Build container image with default tag "local"
- `make test` - Run unit tests with race detection and CSI compliance tests
- `make unit-test` - Run only unit tests with coverage report
- `make csi-compliance-test` - Run CSI sanity tests (skips ValidateVolumeCapabilities, Node Service, SingleNodeWriter)
- `make controller-integration-test` - Run controller integration tests using envtest
- `make lint` - Check Go formatting (strict validation)
- `make fmt` - Format Go code
- `make precommit` - Run all pre-commit hooks (formatting, linting, docs, helm)

### License Management
- `make check-licenses` - Verify dependency licenses against allowed list
- `make generate-licenses` - Generate license files for all dependencies
- `make download-tools` - Download Go tools and dependencies

### CSI Driver Operations
- `make csi-install S3_ENDPOINT_URL=<url>` - Install CSI driver to Kubernetes cluster
- `make csi-uninstall` - Uninstall CSI driver interactively
- `make csi-uninstall-clean` - Uninstall driver and delete custom namespace
- `make csi-uninstall-force` - Force uninstall CSI driver
- `make e2e S3_ENDPOINT_URL=<url>` - Run end-to-end tests on installed driver
- `make e2e-go S3_ENDPOINT_URL=<url>` - Run only Go-based e2e tests
- `make e2e-verify` - Run verification tests only
- `make e2e-all S3_ENDPOINT_URL=<url>` - Install driver and run all tests

### Documentation
- `make docs` - Build and serve documentation with MkDocs (strict mode)
- `make docs-clean` - Clean documentation build artifacts
- `make validate-helm` - Validate Helm charts for correctness

## Architecture

### Core Components

The Scality CSI Driver follows the CSI specification with three main components:

1. **Controller Component** (`scality-csi-controller`): 
   - Manages volume lifecycle and dynamic provisioning operations
   - Uses controller-runtime for Kubernetes controller pattern
   - Handles credential validation and storage class parameters
   - Coordinates with S3 storage for bucket operations

2. **Node Component** (`scality-s3-csi-driver`): 
   - Handles volume mounting/unmounting on each Kubernetes node
   - Implements CSI Node Service RPC
   - Coordinates with mounter component for actual S3 mounting
   - Manages credential providers (AWS profiles, Kubernetes secrets)

3. **Mounter Component** (`scality-s3-csi-mounter`): 
   - Spawns and monitors mountpoint-s3 processes
   - Supports both pod-based and systemd mounting strategies
   - Handles mount argument construction and process lifecycle

4. **Install Component** (`install-mp`): 
   - Utility for installing mountpoint-s3 binary (temporary solution)

### Key Package Structure

- `pkg/driver/` - Main CSI driver implementation
  - `node/` - Node service with mounting logic
    - `mounter/` - Different mounting strategies (pod-based, systemd)
    - `credentialprovider/` - AWS credential management and providers
    - `envprovider/` - Environment variable credential provider
    - `targetpath/` - Target path validation and utilities
    - `volumecontext/` - Volume context parsing and validation
  - `controller/credentialprovider/` - Controller credential provider
  - `storageclass/` - StorageClass parameter parsing and validation
  - `controller.go` - Controller service implementation
  - `identity.go` - CSI identity service
  - `server.go` - gRPC server setup and management
- `pkg/podmounter/` - Mountpoint Pod management and coordination
  - `mountoptions/` - Mount options parsing and validation
  - `mppod/` - Mountpoint Pod creation and lifecycle management
- `pkg/cluster/` - Kubernetes cluster utilities and client management
- `pkg/mountpoint/` - Mountpoint-s3 argument handling and runner
  - `runner/` - Process execution and foreground/background runners
  - `mounter/` - Platform-specific mounting implementations
- `pkg/s3client/` - S3 client utilities and operations
- `pkg/system/` - System utilities (pts, systemd integration)
- `pkg/constants/` - Driver constants and field definitions
- `pkg/util/` - Common utilities (environment, file operations, kubelet)
- `cmd/` - Binary entry points for all components
- `tests/` - Test suites (unit, integration, e2e, helm validation)

### Technology Stack

- **Go**: 1.24.5
- **Kubernetes API**: v0.33.2  
- **CSI Specification**: v1.11.0
- **AWS SDK v2**: For S3 compatibility (even with non-AWS S3 storage)
- **Controller-runtime**: v0.21.0 for Kubernetes controller pattern
- **systemd integration**: via godbus v5.1.0 for systemd mounting strategy
- **Testing**: Ginkgo/Gomega for behavior-driven testing
- **Documentation**: MkDocs for documentation site generation
- **Pre-commit hooks**: For code quality and formatting validation

### Key Features

- **Dual Mounting Strategies**: Pod-based and systemd mounting approaches
- **Credential Flexibility**: AWS profiles, Kubernetes secrets, environment variables
- **Dynamic Provisioning**: Controller-based volume lifecycle management
- **S3 Compatibility**: Works with any S3-compatible storage (optimized for Scality)
- **Comprehensive Testing**: Unit, integration, e2e, and CSI compliance tests
- **Documentation Site**: MkDocs-based documentation with examples and guides
- **CI/CD Integration**: GitHub Actions workflows for testing and releases

## Testing Strategy

### Test Structure
- `tests/sanity/` - CSI specification compliance tests
- `tests/controller/` - Controller integration tests using envtest
- `tests/e2e/` - End-to-end tests against real S3 storage
- Unit tests co-located with source files

### E2E Testing Prerequisites
1. Load credentials: `source tests/e2e/scripts/load-credentials.sh`
2. Provide S3_ENDPOINT_URL parameter
3. Ensure Kubernetes cluster access

## License Management

Uses go-licenses tool to manage dependency licenses:
- Allowed licenses: Apache-2.0, BSD-2-Clause, BSD-3-Clause, ISC, MIT
- `make check-licenses` - Verify compliance
- `make generate-licenses` - Generate license files

## Development Notes

- Static provisioning only (no dynamic bucket creation)
- Mount operations use the mountpoint-s3 binary for actual S3 mounting
- Credential management supports AWS profiles and Kubernetes secrets
- CSI sanity tests skip controller capabilities due to static provisioning limitation

## Best Practices

- Always give a commit message in the end and ask user to commit
- When giving a commit message, also provide a PR description