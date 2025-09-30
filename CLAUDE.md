# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

The Scality CSI Driver for S3 is a Kubernetes CSI driver that enables mounting Scality RING S3 buckets as persistent volumes.
It uses mount-s3 (a FUSE-based filesystem) to provide POSIX-like access to S3 objects. The driver supports both static provisioning (pre-existing buckets) and dynamic provisioning (automatic bucket creation).

## Architecture

### Key Components

1. **CSI Driver Node Service** (`cmd/scality-csi-driver`): Main CSI driver implementing NodePublishVolume/NodeUnpublishVolume RPCs. Runs as a DaemonSet on each node and handles volume mount/unmount operations.

2. **CSI Controller Service** (`cmd/scality-csi-controller`): Implements CSI Controller Service for dynamic provisioning. Handles CreateVolume/DeleteVolume RPCs and manages S3 bucket lifecycle.
Includes a separate controller process that reconciles MountpointS3PodAttachment CRDs.

3. **CSI Mounter** (`cmd/scality-csi-mounter`): Helper binary that runs mount-s3 processes inside dedicated "mounter pods" for improved isolation and resource management.

4. **Install MP** (`cmd/install-mp`): Installation helper for mount-s3 binary.

### Package Structure

- `pkg/driver`: Core CSI driver implementation (controller, node, identity services)
- `pkg/driver/node/mounter`: Mounting logic with two strategies:
  - **systemd mounter**: Runs mount-s3 via systemd transient services (legacy)
  - **pod mounter**: Runs mount-s3 in dedicated Kubernetes pods (default)
- `pkg/driver/node/credentialprovider`: Credential resolution from secrets, driver defaults, or AWS profiles
- `pkg/driver/controller/credentialprovider`: Controller-side credential provider for dynamic provisioning
- `pkg/podmounter/mppod`: Mounter pod creation, management, and resource calculations
- `pkg/mountpoint`: mount-s3 argument construction and process execution
- `pkg/api/v2`: CRD definitions for MountpointS3PodAttachment (tracks volume attachments)
- `pkg/s3client`: S3 client wrapper for bucket operations
- `pkg/system`: Low-level system interactions (systemd, pts, namespaces)

### Critical Design Patterns

- **Dual Mounter Strategy**: The driver supports two mounting approaches. Pod mounter is enabled by default and recommended. Systemd mounter is legacy but still supported.
- **Credential Resolution Chain**: Credentials are resolved in order: secret-based → driver-level → AWS profile → IAM roles.
- **Volume Sharing**: Multiple pods can share the same S3 volume. MountpointS3PodAttachment CRD tracks these shared mounts.
- **Resource Management**: Mounter pods have resource requests/limits calculated based on cache size and mount options.

## Development Commands

### Building

```bash
# Build all binaries (cross-compiles to Linux)
make bin

# Build container image (default tag: local)
make container

# Build with custom tag
make container CONTAINER_TAG=v2.0.0
```

### Testing

```bash
# Run unit tests
make unit-test

# Run unit tests with race detection and coverage
make test

# Generate coverage report
make cover

# Run CSI compliance tests (sanity tests)
make csi-compliance-test

# Run controller integration tests (uses envtest)
make controller-integration-test
```

### Code Quality

```bash
# Format code
make fmt

# Run linters
make lint

# Run all pre-commit hooks
make precommit
```

### Documentation

```bash
# Build and serve documentation (MkDocs)
make docs

# Clean documentation artifacts
make docs-clean
```

### Code Generation

```bash
# Generate CRD manifests and deepcopy functions
make generate
```

This regenerates:

- CRD YAML in `charts/scality-mountpoint-s3-csi-driver/crds/`
- `zz_generated.deepcopy.go` for API types

### CRD Installation

```bash
# Install CRDs directly from repository using kustomize
kubectl apply -k github.com/scality/mountpoint-s3-csi-driver

# Or install from local directory
kubectl apply -k .
```

### License Management

```bash
# Check dependency licenses
make check-licenses

# Generate license files for dependencies
make generate-licenses
```

### E2E Testing Workflow

E2E tests require a Scality RING S3 endpoint and credentials.

```bash
# Load credentials from integration_config.json
source tests/e2e/scripts/load-credentials.sh

# Install driver, run all tests
make e2e-all S3_ENDPOINT_URL=https://s3.example.com

# Or use separate commands:
make csi-install S3_ENDPOINT_URL=https://s3.example.com
make e2e S3_ENDPOINT_URL=https://s3.example.com
make csi-uninstall

# Run only Go-based e2e tests
make e2e-go

# Run only verification tests
make e2e-verify

# Uninstall options
make csi-uninstall              # Interactive
make csi-uninstall-clean        # Delete custom namespace
make csi-uninstall-force        # Force uninstall
```

### Mage Targets (Alternative Development Workflow)

Mage provides higher-level tasks for local development with minikube/kind:

```bash
# Install mage
go install github.com/magefile/mage@latest

# Build and install CSI driver from local source
mage up

# Remove CSI driver and resources
mage down

# Install specific version from OCI registry
SCALITY_CSI_VERSION=v2.0.0 mage install

# Configure/remove DNS mapping for s3.example.com
mage configureS3DNS
mage removeS3DNS
mage showS3DNSStatus
```

## Testing Guidelines

### Unit Tests

- Unit tests should use fakes/mocks defined in `*test.go` files or dedicated `*/mocks/` directories
- Controller tests use `envtest` (real Kubernetes API server) for integration testing
- Mock generation uses `github.com/golang/mock`

### E2E Tests

- Located in `tests/e2e/`
- Use Ginkgo/Gomega testing framework
- Require real S3 infrastructure
- Test both static and dynamic provisioning scenarios
- Separate `go.mod` to isolate e2e dependencies

### Running Single Test

```bash
# Run specific test by name
go test -v ./pkg/driver/node/mounter -run TestPodMounter

# Run tests in specific package
go test -v ./pkg/driver/...
```

## Important Conventions

### Version Management

Version is set in Makefile (`VERSION=2.0.0`) and injected at build time via ldflags into `pkg/driver/version`.

### Commit Messages

Follow conventional commit style based on repository history:

- Use prefixes: `S3CSI-XXX:` for Jira ticket references
- Format: `S3CSI-XXX: Brief description of change`
- Focus on what changed and why

### Branch Strategy

- Main branch: `main`
- Feature branches: `feature/S3CSI-XXX-description`
- Improvement branches: `improvement/S3CSI-XXX-description`

### Platform-Specific Code

- Platform-specific files use build tags: `//go:build linux` or `//go:build darwin`
- Always provide Darwin stubs for Linux-only functionality to support local development on macOS
- CI runs with `GOOS=linux` to ensure Linux-specific code is analyzed

### Helm Chart

- Chart location: `charts/scality-mountpoint-s3-csi-driver/`
- Values can be customized for image repository, tag, resources, etc.
- CRDs are included in `crds/` subdirectory

## Key Environment Variables

- `CSI_NODE_NAME`: Node name for CSI driver
- `MOUNTPOINT_VERSION`: Mount-s3 version to report
- `MOUNTPOINT_NAMESPACE`: Namespace for mounter pods
- `S3_ENDPOINT_URL`: S3 endpoint URL (required for e2e tests)
- `ACCOUNT1_ACCESS_KEY` / `ACCOUNT1_SECRET_KEY`: S3 credentials (loaded via `load-credentials.sh`)

## Troubleshooting

- Check pod logs: `kubectl logs -n kube-system <pod-name>`
- Check systemd services (legacy mounter): `systemctl status mount-s3-*`
- Check mounter pods: `kubectl get pods -n kube-system -l app=mountpoint-s3-csi-mounter`
- Check CRDs: `kubectl get mountpoints3podattachments` or `kubectl get s3pa`
- Enable debug logging via mount option: `--log-level debug`

## Important Files

- `Makefile`: Primary build and test commands
- `magefiles/`: Mage-based development workflow
- `.pre-commit-config.yaml`: Pre-commit hooks configuration
- `.golangci.yaml`: Linter configuration
- `mkdocs.yml`: Documentation site configuration
- `integration_config.json`: E2E test credentials (not in repo, user-provided)
