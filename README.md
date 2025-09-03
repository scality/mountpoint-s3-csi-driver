# Scality CSI Driver for S3

This repository hosts the Container Storage Interface (CSI) driver enabling Kubernetes applications to mount Scality S3 buckets as file system volumes.

**Version 2.0 Features:**
- **SELinux Support**: Full support for SELinux-enabled Kubernetes workloads through pod-based mounting
- **Pod Sharing**: Multiple pods can efficiently share the same S3 volume using reference-counted locking
- **Resource Optimization**: Headroom management ensures mount pods can be scheduled efficiently
- **Controller-Based Architecture**: Improved lifecycle management with CRD-based pod creation
- **Automatic Migration**: Existing systemd mounts seamlessly transition to pod mounter on pod restart

For architecture details, see [Architecture v2 Documentation](docs/ARCHITECTURE_V2.md).

Refer to the [official documentation site](https://scality.github.io/mountpoint-s3-csi-driver/) for detailed installation, usage, and features.

[![Linting and Formatting](https://github.com/scality/mountpoint-s3-csi-driver/actions/workflows/linting-and-formatting.yaml/badge.svg)](https://github.com/scality/mountpoint-s3-csi-driver/actions/workflows/linting-and-formatting.yaml)
[![Code Quality Tests](https://github.com/scality/mountpoint-s3-csi-driver/actions/workflows/code-quality-tests.yaml/badge.svg)](https://github.com/scality/mountpoint-s3-csi-driver/actions/workflows/code-quality-tests.yaml)
[![E2E Integration Tests with RING S3](https://github.com/scality/mountpoint-s3-csi-driver/actions/workflows/e2e-tests.yaml/badge.svg)](https://github.com/scality/mountpoint-s3-csi-driver/actions/workflows/e2e-tests.yaml)
