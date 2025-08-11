# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is the Mountpoint S3 CSI Driver - a Kubernetes CSI driver that enables mounting S3 buckets as filesystem volumes in Kubernetes pods.
It's designed for Scality RING S3 but works with S3-compatible storage.

## Common Development Commands

### Building

- `make bin` - Build all binaries
- `make fmt` - Format Go code
- `make lint` - Run linting checks
- `make precommit` - Run pre-commit hooks before committing

### Testing

- `make unit-test` - Run unit tests
- `make csi-compliance-test` - Run CSI spec compliance tests
- `make controller-integration-test` - Run controller integration tests
- `make test` - Run all tests
- `make e2e-all S3_ENDPOINT_URL=<url>` - Run full E2E test suite
- To run a single test: `go test -v ./path/to/package -run TestName`

### Installation & E2E Testing

- `make csi-install S3_ENDPOINT_URL=<url>` - Install CSI driver to cluster
- `make csi-uninstall` - Uninstall CSI driver
- `make e2e S3_ENDPOINT_URL=<url>` - Run E2E tests on installed driver

### Documentation

- `make docs` - Build and serve documentation locally
- `make docs-clean` - Clean documentation artifacts

## Architecture

### Core Components

The driver consists of multiple binaries, each with a specific responsibility:

1. **scality-csi-driver** (`cmd/scality-csi-driver/`) - Main CSI driver implementing the node service that handles volume mounting/unmounting on each node

2. **scality-csi-controller** (`cmd/scality-csi-controller/`) - Controller for managing mount pods when using experimental pod-based mounting

3. **scality-csi-mounter** (`cmd/scality-csi-mounter/`) - Helper binary that performs the actual mount operations using mount-s3

4. **install-mp** (`cmd/install-mp/`) - Temporary solution to install mount-s3 binary until full containerization

### Mounting Strategies

The driver supports two mounting strategies:

- **SystemD Mounter** (default): Uses systemd services to manage mount lifecycle. Each mount creates a systemd service that runs the mount-s3 binary.

- **Pod Mounter** (experimental): Creates dedicated pods for each mount operation, providing better isolation and resource management.

**WARNING: This is completely experimental, not supported, and should NOT be used in production environments.**

### Key Package Structure

- `pkg/driver/node/` - Core node service implementation handling CSI NodePublish/NodeUnpublish
- `pkg/driver/node/mounter/` - Mounting strategy implementations (systemd vs pod-based)
- `pkg/driver/node/credentialprovider/` - Handles AWS credential management from various sources
- `pkg/podmounter/` - Pod-based mounting controller and implementation
- `pkg/cluster/` - Kubernetes API interactions
- `pkg/system/` - System-level operations (systemd, pts handling)

### CSI Implementation Details

- Implements CSI spec v1.11.0
- Only supports static provisioning (pre-created S3 buckets)
- Driver name: `s3.csi.scality.com`
- No controller service (no dynamic provisioning)
- Node service handles all mount operations

### Testing Architecture

- Unit tests are co-located with code (`*_test.go`)
- CSI compliance tests in `tests/sanity/`
- Controller integration tests use envtest framework
- E2E tests in `tests/e2e/` with custom test suites for S3-specific scenarios

## Key Environment Variables

When running or testing the driver:

- `AWS_ENDPOINT_URL` - S3 endpoint URL (required)
- `CSI_NODE_NAME` - Node identifier for CSI operations
- `MOUNTPOINT_VERSION` - Version string to report
- `MOUNTPOINT_NAMESPACE` - Namespace for mount pods (pod mounter only)
- `USE_POD_MOUNTER` - Enable experimental pod mounter (NOT for production use)

## Credential Management

The driver supports multiple credential sources:

1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
2. Kubernetes secrets referenced in volumes
3. AWS profile configuration

## Development Notes

- Always run `make fmt` and `make lint` before committing
- The project uses pre-commit hooks - ensure they pass
- When modifying mounting logic, test both systemd and pod mounter strategies
- E2E tests require a real S3-compatible endpoint
- The driver is designed for Scality RING but should work with any S3-compatible storage
