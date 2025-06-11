# Scality Container Storage Interface S3 Driver Documentation

The Scality S3 Container Storage Interface (CSI) Driver allows Kubernetes applications to access Scality S3 objects through a file system interface.
This driver is a fork of the [Mountpoint for Amazon S3 CSI Driver](https://github.com/awslabs/mountpoint-s3-csi-driver).
It has been engineered and optimized specifically for use with Scality S3-compatible storage solutions.

Scality CSI driver presents an S3 bucket as a storage volume accessible by containers in Kubernetes clusters using [Mountpoint for Amazon S3](https://github.com/awslabs/mountpoint-s3).
It implements the [CSI specification](https://github.com/container-storage-interface/spec/blob/master/spec.md) for container orchestrators to manage storage volumes.

---

## Key Features

- **Static Provisioning Only**: Integrate existing S3 buckets as persistent storage in Kubernetes. Dynamic provisioning is not supported.
- **Familiar File Access**: Access S3 objects as files and directories, simplifying application integration.
- **Customizable Mounts**: Fine-tune volume mounts with a variety of supported options for performance and behavior.
- **Scality Integration**: Optimized for Scality S3 storage solutions like [Scality RING](https://www.scality.com/ring/).

---

## Documentation Overview

| Topic | Description | Documentation |
|-------|-------------|---------------|
| **Driver Deployment** | | |
| Prerequisites | Kubernetes cluster, RING storage, credentials, and network requirements before installation | [Prerequisites](driver-deployment/prerequisites.md) |
| Quick Start | Three commands to install the driver and mount a test bucket | [Quick Start Guide](driver-deployment/quick-start.md) |
| Installation Guide | Step‑by‑step Helm install with custom values, upgrades, and rollbacks | [Installation Guide](driver-deployment/installation-guide.md) |
| Uninstallation | Safely remove driver pods, CRDs, and secrets from the cluster | [Uninstallation Guide](driver-deployment/uninstallation.md) |

## Container Images

Container images for the Scality S3 CSI Driver are hosted on GHCR:

| Driver Version | Image URL                                                                 |
|----------------|---------------------------------------------------------------------------|
| 1.0.0          | `ghcr.io/scality/mountpoint-s3-csi-driver:1.0.0`                          |

*Note: Please check the [releases page](https://github.com/scality/mountpoint-s3-csi-driver/releases) for the latest available versions.*

## Support and Community

For issues or questions:

1. Search existing [GitHub Issues](https://github.com/scality/mountpoint-s3-csi-driver/issues)
2. Open a new [GitHub Issue](https://github.com/scality/mountpoint-s3-csi-driver/issues) if the problem is not already addressed
