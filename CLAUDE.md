# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Scality CSI Driver for S3 — a Kubernetes CSI driver that mounts Scality RING S3
buckets as persistent volumes using mount-s3 (FUSE-based). Supports static
provisioning (pre-existing buckets) and dynamic provisioning (automatic bucket
creation). This is a Scality fork of the AWS upstream CSI driver with additional
features: dynamic provisioning and RING S3 compatibility.

## Build & Test Commands

```bash
# Build
make bin                              # Cross-compile all Go binaries (CGO_ENABLED=0 GOOS=linux)
make container CONTAINER_TAG=local    # Build Docker image

# Test
make test                             # Unit tests + CSI compliance (race detection + coverage)
make controller-integration-test      # Controller tests with envtest (real K8s API server)
go test -v ./pkg/driver/node/mounter -run TestPodMounter  # Single test

# Code quality
make fmt                              # gofmt + goimports + gofumpt
make precommit                        # All pre-commit hooks (includes golangci-lint with GOOS=linux)

# Code generation (after modifying pkg/api/v2/ types)
make generate                         # Regenerates CRD YAML + zz_generated.deepcopy.go
```

### Local Development with Mage

Mage provides higher-level orchestration for local development with kind/minikube:

```bash
# Core workflow
mage up                               # Build from source, load image, install via Helm
mage status                           # Check installation
mage down                             # Remove driver and cleanup

# With S3 endpoint
S3_ENDPOINT_URL=http://192.0.0.2:8000 mage up

# Install published version (not from source)
SCALITY_CSI_VERSION=2.0.0 mage install

# DNS mapping for S3 endpoints (via CoreDNS)
mage configureS3DNS / removeS3DNS / showS3DNSStatus
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

E2E tests require a Scality RING S3 endpoint and credentials. The E2E workflow is orchestrated via Mage targets (Makefile targets delegate to Mage internally).

```bash
# Install mage (required)
go install github.com/magefile/mage@latest

# Full workflow: load credentials, install driver, run tests
S3_ENDPOINT_URL=https://s3.example.com mage e2e:all

# Or use separate commands:
S3_ENDPOINT_URL=https://s3.example.com mage e2e:install
S3_ENDPOINT_URL=https://s3.example.com mage e2e:test
mage e2e:uninstall

# Run only Go-based e2e tests (skip verification)
S3_ENDPOINT_URL=https://s3.example.com mage e2e:goTest

# Run only verification (driver health check)
mage e2e:verify

# Uninstall options
mage e2e:uninstall              # Helm uninstall + delete secret
mage e2e:uninstallClean         # Also delete custom namespace
mage e2e:uninstallForce         # Force uninstall + delete CSI driver registration

# With custom image (CI usage)
S3_ENDPOINT_URL=https://s3.example.com CSI_IMAGE_TAG=v2.0.0 CSI_IMAGE_REPOSITORY=ghcr.io/scality/mountpoint-s3-csi-driver mage e2e:install

# Makefile targets still work (they delegate to Mage):
# make e2e-all S3_ENDPOINT_URL=https://s3.example.com
# make csi-install S3_ENDPOINT_URL=https://s3.example.com
```

### OpenShift E2E (CRC-based)

```bash
# Run full E2E suite on OpenShift
mage e2e:openShiftAll

# Create image pull secret for GHCR
mage e2e:createPullSecret

# Apply SCCs manually (used by CI workflow)
oc apply -f .github/openshift/scc.yaml

# Configure DNS (dispatches to OpenShift path when CLUSTER_TYPE=openshift)
CLUSTER_TYPE=openshift mage e2e:configureCIDNS
```

### Mage Targets (Local Development Workflow)

Mage provides higher-level tasks for local development with minikube/kind:

```bash
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

Require real S3 infrastructure. Credentials loaded from `tests/e2e/integration_config.json`.

```bash
S3_ENDPOINT_URL=https://s3.example.com mage e2e:all     # Full workflow
S3_ENDPOINT_URL=https://s3.example.com mage e2e:install  # Install only
S3_ENDPOINT_URL=https://s3.example.com mage e2e:test     # Run tests only
mage e2e:verify                                           # Health check
mage e2e:uninstall                                        # Cleanup
mage e2e:uninstallForce                                   # Force cleanup + delete CSI registration
```

## Architecture

### Components (cmd/)

| Binary | Role |
|--------|------|
| `scality-csi-driver` | Node service — handles mount/unmount (NodePublish/NodeUnpublish RPCs). Runs as DaemonSet. |
| `scality-csi-controller` | Controller service — handles bucket creation/deletion (CreateVolume/DeleteVolume RPCs) + reconciles MountpointS3PodAttachment CRDs. |
| `scality-csi-mounter` | Helper binary running mount-s3 inside dedicated "mounter pods". |
| `install-mp` | Installation helper for mount-s3 binary. |

### Key Packages (pkg/)

- **`driver/node/`** — Node service implementation. Core mount/unmount logic.
  - `mounter/` — Pod mounter implementation (runs mount-s3 in dedicated K8s pods).
  - `credentialprovider/` — Credential resolution chain: secret → driver defaults → AWS profile → IAM roles.
- **`driver/controller/`** — Controller service for dynamic provisioning.
- **`api/v2/`** — CRD definitions for `MountpointS3PodAttachment` (API group: `s3.csi.scality.com/v2`, short name: `s3pa`). Tracks volume attachments to enable sharing across pods.
- **`podmounter/mppod/`** — Mounter pod creation, lifecycle management, resource calculations.
- **`mountpoint/`** — mount-s3 argument construction (`mounter/`) and process execution (`runner/`).
- **`s3client/`** — S3 client wrapper for bucket operations (create/delete/head).
- **`system/`** — Low-level system interactions: pseudo-terminal management (`pts/`).
- **`storageclass/`** — StorageClass parameter parsing.

### Design Patterns

- **Pod mounter**: mount-s3 runs in dedicated K8s pods for isolation and resource management.
- **Volume sharing**: Multiple workload pods can share the same S3 volume. `MountpointS3PodAttachment` CRD tracks these shared mounts with one CRD per (PV, node) pair.
- **Platform-specific code**: Linux-only functionality uses `//go:build linux` tags with Darwin stubs for macOS development. CI lints with `GOOS=linux`.

### Helm Chart

Located at `charts/scality-mountpoint-s3-csi-driver/`. Key values:

- `s3.endpointUrl` — S3 endpoint (required)
- `s3.region` — Default region (us-east-1)
- `image.repository` / `image.tag` — CSI driver image
- `s3CredentialSecret` — Secret name and key fields for S3 credentials

CRDs are in `charts/scality-mountpoint-s3-csi-driver/crds/`.

## Version Management

Version is set in `Makefile` (`VERSION=2.1.0`) and injected at build time via ldflags into `pkg/driver/version`.

## Conventions

- **Commit messages**: `S3CSI-XXX: Brief description` (Jira ticket prefix)
- **Branches**: `feature/S3CSI-XXX-description` or `improvement/S3CSI-XXX-description`
- **Go formatting**: gofmt → goimports → gofumpt (in that order)
- **Linting**: golangci-lint with `GOOS=linux` (errcheck, govet, ineffassign, staticcheck, unused)
- **Code generation trigger**: Any change to `pkg/api/v2/` types requires `make generate`
- **Testing workflow**: Before every commit run `make fmt && make precommit`.
  Then consult the "Testing Guide: From Diff to Done" table in
  `docs/dev/multi-cluster-setup.md` to determine which additional tests
  (unit, integration, E2E, upgrade) are required based on what files you
  changed. Run all required tests locally on your own Kind cluster before
  pushing. **You MUST actually run the tests, not just suggest them.**
  See the mandatory testing section below.

## Mandatory Local Testing

**Every code change MUST be tested on a local Kind cluster before considering
it done.** This is non-negotiable. Unit tests alone are insufficient — you
must verify the change works end-to-end in a real Kubernetes environment.

### Test Execution Requirements

1. **Always run first** (no cluster needed):

   ```bash
   make fmt && make precommit
   make unit-test
   ```

2. **Then build and deploy to your Kind cluster:**

   ```bash
   make container CONTAINER_TAG=local
   kind load docker-image scality/mountpoint-s3-csi-driver:local --name <cluster>
   kubectl config use-context kind-<cluster>
   KIND_CLUSTER_NAME=<cluster> S3_ENDPOINT_URL=http://192.168.64.1:8000 mage up
   ```

3. **Run smoke test** (quick validation — static + dynamic provisioning):
   Follow the smoke test instructions in `docs/dev/multi-cluster-setup.md`. For bug fixes, reproduce the customer's exact scenario (security context, mount options, provisioning mode) and verify it works.

4. **Run E2E tests** (full validation):

   ```bash
   S3_ENDPOINT_URL=http://192.168.64.1:8000 \
     KIND_CLUSTER_NAME=<cluster> \
     CONTAINER_TAG=local \
     CSI_IMAGE_REPOSITORY=scality/mountpoint-s3-csi-driver \
     mage e2e:all
   ```

### Credentials

S3 credentials are in `tests/e2e/integration_config.json`. For environment variables:

- `ACCOUNT1_ACCESS_KEY=accessKey1`, `ACCOUNT1_SECRET_KEY=verySecretKey1`
- S3 endpoint from Docker Desktop on macOS: `http://192.168.64.1:8000`

### Parallel Testing with Multiple Clusters

When running both smoke tests and E2E tests, use separate Kind clusters to avoid interference:

```bash
# Build image once
make container CONTAINER_TAG=local

# Load into both clusters
kind load docker-image scality/mountpoint-s3-csi-driver:local --name s3csi-1
kind load docker-image scality/mountpoint-s3-csi-driver:local --name s3csi-2

# Smoke test on cluster 1 (mage up installs in default namespace)
kubectl config use-context kind-s3csi-1
KIND_CLUSTER_NAME=s3csi-1 S3_ENDPOINT_URL=http://192.168.64.1:8000 mage up
# ... run smoke test manifests ...

# E2E on cluster 2 (e2e:all installs in kube-system namespace)
kubectl config use-context kind-s3csi-2
ACCOUNT1_ACCESS_KEY=accessKey1 ACCOUNT1_SECRET_KEY=verySecretKey1 \
  S3_ENDPOINT_URL=http://192.168.64.1:8000 KIND_CLUSTER_NAME=s3csi-2 \
  CONTAINER_TAG=local CSI_IMAGE_REPOSITORY=scality/mountpoint-s3-csi-driver \
  mage e2e:all
```

When spawning agent teams, assign each agent its own cluster. Never share a cluster between agents — E2E tests create/delete namespaces and resources that can conflict with smoke tests.

### When to Run Which Tests

Consult the "Testing Guide: From Diff to Done" table in `docs/dev/multi-cluster-setup.md` for the complete mapping. Key rules:

- **Any `pkg/` change**: `make test` + build image + `mage e2e:all`
- **`tests/e2e/` change**: build image + `mage e2e:all`
- **`charts/` change**: `make validate-helm` + build image + `mage e2e:all`
- **Bug fix for customer issue**: Reproduce the exact customer scenario as a smoke test, then run full E2E

## Key Environment Variables

| Variable | Used By | Purpose |
|----------|---------|---------|
| `S3_ENDPOINT_URL` | mage up, e2e | S3 endpoint URL |
| `SCALITY_CSI_VERSION` | mage install | Published version to install |
| `CONTAINER_TAG` | make container, mage up | Docker image tag (default: local) |
| `CSI_NAMESPACE` | mage | Target namespace (default: default for dev, kube-system for e2e) |
| `KIND_CLUSTER_NAME` | mage | Kind cluster name (default: kind) |
| `VERBOSE` | mage | Enable verbose output |

## Test Structure

- **Unit tests**: Standard Go tests with mocks (`*_test.go` files, `*/mocks/` directories). Mock generation via `github.com/golang/mock`.
- **CSI compliance**: `tests/sanity/` — Ginkgo-based CSI spec conformance.
- **Controller integration**: `tests/controller/` — Uses envtest with real K8s API server (Ginkgo).
- **E2E**: `tests/e2e/` — Separate `go.mod`, Ginkgo/Gomega, requires real S3.
  Custom suites in `tests/e2e/customsuites/` cover credentials, mount options,
  multi-volume, cache, file permissions, and dynamic provisioning variants.
  Performance tests exist but only run with `-performance` flag.
- **Upgrade**: `tests/upgrade/` — Version migration scenarios.
- **Helm validation**: `tests/helm/validate_charts.sh`.
