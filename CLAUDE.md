# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scality CSI Driver for S3 — a Kubernetes CSI driver that mounts Scality RING S3 buckets
as persistent volumes using AWS Mountpoint (a FUSE-based `mount-s3` binary).
Scality fork of `awslabs/mountpoint-s3-csi-driver` with dynamic provisioning,
RING S3 support, pod-based mounting, and a Mage build workflow.

Go module: `github.com/scality/mountpoint-s3-csi-driver`
CSI driver name: `s3.csi.scality.com`

## Commands

### Build

```bash
make bin                              # Cross-compile Go binaries (GOOS=linux)
make container CONTAINER_TAG=local    # Build Docker image
```

### Test

```bash
make unit-test                        # Unit tests with coverage
make test                             # Unit tests (race detection) + CSI compliance
make csi-compliance-test              # CSI sanity tests only
make controller-integration-test      # Controller tests with envtest (real API server)

# Single test
go test -v ./pkg/driver/node/mounter -run TestPodMounter

# E2E (requires kind/minikube cluster + S3 backend)
S3_ENDPOINT_URL=http://s3.example.com:8000 mage e2e:all
```

### Code Quality

```bash
make fmt                              # gofmt
make precommit                        # All pre-commit hooks (golangci-lint runs with GOOS=linux)
gofumpt -w .                          # Strict formatting (required by pre-commit)
goimports -w .                        # Import organization (required by pre-commit)
```

### Code Generation

After modifying `pkg/api/v2/` types:

```bash
make generate                         # Regenerate CRD YAML + deepcopy methods
```

### Local Development (requires kind/minikube + mage)

```bash
S3_ENDPOINT_URL=http://192.0.0.2:8000 mage up    # Build image, load to cluster, install via Helm
mage status                                        # Check installation
mage down                                          # Uninstall + cleanup

SCALITY_CSI_VERSION=1.2.0 mage install             # Install published version from OCI registry
```

Credentials are loaded from `tests/e2e/integration_config.json`. Mage auto-detects kind/minikube/OpenShift and configures DNS mapping (`s3.example.com` → real S3 endpoint) via CoreDNS.

### Documentation

```bash
make docs                             # Build (strict) + serve MkDocs site
make docs-clean                       # Remove site/ build artifacts
```

## Architecture

### Three Binaries

1. **`scality-s3-csi-driver`** (`cmd/scality-csi-driver/`) — Combined CSI node + identity +
   controller gRPC server. Runs as DaemonSet. Handles `NodePublishVolume`/`NodeUnpublishVolume`.
   Can run controller-only with `CSI_CONTROLLER_ONLY=true`.

2. **`scality-csi-controller`** (`cmd/scality-csi-controller/`) — Kubernetes operator
   (controller-runtime). Watches workload pods, creates/manages Mountpoint Pods and
   `MountpointS3PodAttachment` CRDs. Runs stale attachment cleaner.

3. **`scality-s3-csi-mounter`** (`cmd/scality-csi-mounter/`) — Entrypoint for Mountpoint Pods. Receives mount options via Unix socket, spawns `mount-s3`, waits for unmount signal.

### Key Package Map

| Package | Purpose |
|---------|---------|
| `pkg/driver/` | Core CSI driver: `driver.go` (setup), `controller.go` (CreateVolume/DeleteVolume for dynamic provisioning) |
| `pkg/driver/node/` | CSI Node service: `node.go` (mount/unmount coordination) |
| `pkg/driver/node/mounter/` | `Mounter` interface + `PodMounter` implementation (source mount + bind mount) |
| `pkg/driver/node/credentialprovider/` | Credential resolution (secrets, env, driver config) |
| `pkg/driver/storageclass/` | StorageClass parameter parsing for dynamic provisioning |
| `pkg/api/v2/` | `MountpointS3PodAttachment` CRD types + field indexers |
| `pkg/podmounter/mppod/` | Mountpoint Pod creation, path management, headroom pods |
| `pkg/podmounter/mountoptions/` | Mount option serialization over Unix sockets |
| `pkg/s3client/` | `Client` interface for S3 operations (CreateBucket, DeleteBucket, BucketExists) |
| `pkg/system/` | Linux-specific syscalls (FUSE mount, bind mount) |
| `cmd/scality-csi-controller/csicontroller/` | Reconciler, expectations system, stale attachment cleaner |

### Dynamic Provisioning Flow (CRD Coordination)

The `MountpointS3PodAttachment` CRD decouples controller decisions from node mount actions:

1. User creates PVC → CSI external-provisioner calls `CreateVolume` → controller creates S3 bucket
2. Kubelet schedules workload pod → calls `NodePublishVolume` on node
3. **Pod Reconciler** (controller) detects workload pod, creates Mountpoint Pod + S3PA CRD
4. **Node driver** waits for S3PA, finds assigned Mountpoint Pod name
5. Node sends mount options to Mountpoint Pod via Unix socket → `mount-s3` performs FUSE mount
6. Node creates bind mount from source to workload container's target path

**Volume sharing**: Workloads with matching (node + PV + mountOptions + fsGroup) share one Mountpoint Pod.

### Expectations System

The reconciler (`csicontroller/expectations.go`) tracks pending S3PA creations in memory to handle Kubernetes eventual consistency — prevents duplicate CRD creation during informer cache lag.

## Test Organization

| Directory | Framework | What it tests |
|-----------|-----------|---------------|
| `{cmd,pkg}/*_test.go` | `go test` | Unit tests with mocked interfaces |
| `tests/sanity/` | `csi-test` (Ginkgo) | CSI spec compliance |
| `tests/controller/` | `envtest` (Ginkgo/Gomega) | Reconciler + CRD operations against real API server |
| `tests/e2e/` | Ginkgo + Mage | End-to-end on real K8s + S3 (custom suites in `customsuites/`) |
| `tests/upgrade/` | Mage | Driver version upgrade compatibility |
| `tests/helm/` | Shell scripts | Helm chart validation |

## Helm Chart

Located at `charts/scality-mountpoint-s3-csi-driver/`. Key templates: `node.yaml` (DaemonSet), `controller.yaml` (Deployment), CRD in `crds/`. Validate with `helm lint charts/scality-mountpoint-s3-csi-driver`.

## CI Workflows

| Workflow | Purpose |
|----------|---------|
| `code-quality-tests.yaml` | Unit tests + coverage |
| `linting-and-formatting.yaml` | golangci-lint, pre-commit hooks |
| `e2e-tests.yaml` | E2E on kind |
| `e2e-openshift.yaml` | E2E on OpenShift |
| `upgrade-test.yaml` | Upgrade compatibility |
| `release.yaml` | Build + publish to OCI registry |

## Conventions

- **Formatting**: `gofmt` + `goimports` + `gofumpt` (all three checked by pre-commit). golangci-lint runs with `GOOS=linux` to include Linux-only files.
- **Linting config**: `.golangci.yaml` — enables errcheck, govet, ineffassign, staticcheck, unused. Excludes `tests/controller` and `tests/sanity`.
- **Commit messages**: `S3CSI-XXX: Brief description` with body explaining what/why, then `Issue: <ISSUE-NUMBER>`.
- **Branch naming**: `feature/S3CSI-XXX-description` or `improvement/S3CSI-XXX-description`.
- **Docker image**: Multi-stage build — Amazon Linux 2 for Mountpoint binary (old glibc), Go builder, EKS Distro minimal runtime.

## Mermaid Diagrams

Do not add `<br/>` tags in Mermaid diagrams — MkDocs does not honor them.
