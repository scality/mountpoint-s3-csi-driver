# Welcome to the Scality S3 CSI Driver Documentation

The Scality S3 Container Storage Interface (CSI) Driver allows Kubernetes applications to access Scality S3 objects through a file system interface.
This driver is a fork of the [Mountpoint for Amazon S3 CSI Driver](https://github.com/awslabs/mountpoint-s3-csi-driver) and has been adapted for use with Scality S3-compatible storage solutions.

Scality CSI driver presents an S3 bucket as a storage volume accessible by containers in Kubernetes clusters using [Mountpoint for Amazon S3](https://github.com/awslabs/mountpoint-s3).
It implements the [CSI specification](https://github.com/container-storage-interface/spec/blob/master/spec.md) for container orchestrators to manage storage volumes.

## Key Features

- **Static Provisioning**: Easily integrate existing S3 buckets as persistent storage in Kubernetes
- **Familiar File Access**: Access S3 objects as files and directories, simplifying application integration
- **Customizable Mounts**: Fine-tune volume mounts with a variety of options for performance and behavior
- **Scality Integration**: Optimized for Scality S3 storage solutions like [Scality RING](https://www.scality.com/ring/).

## Getting Started

The **[Quick Start Guide](quick-start.md)** provides step-by-step instructions for deploying the driver and mounting S3 buckets in Kubernetes clusters.

## Explore the Documentation

This documentation provides comprehensive information to install, configure, use, and troubleshoot the Scality S3 CSI Driver.

### Installation & Setup

<!-- TODO: Create installation.md file -->
<!-- - **[Installation](installation.md)** - Prerequisites and installation instructions -->
- **[Quick Start Guide](quick-start.md)** - Step-by-step deployment guide

### Configuration & Usage

<!-- TODO: Create configuration/index.md file -->
<!-- - **[Configuration](configuration/index.md)** - Driver and volume configuration -->
<!-- TODO: Create how-to/static-provisioning.md file -->
<!-- - **[How-To Guides](how-to/static-provisioning.md)** - Practical implementation examples -->

### Understanding the Driver

<!-- TODO: Create concepts/filesystem-semantics.md file -->
<!-- - **[Concepts](concepts/filesystem-semantics.md)** - Underlying principles and limitations -->
<!-- TODO: Create reference/access-modes.md file -->
<!-- - **[Reference](reference/access-modes.md)** - Detailed feature and option reference -->

### Support & Troubleshooting

<!-- TODO: Create troubleshooting.md file -->
<!-- - **[Troubleshooting](troubleshooting.md)** - Common issue resolution -->
- **[GitHub Issues](https://github.com/scality/mountpoint-s3-csi-driver/issues)** - Bug reports and feature requests

## Container Images

Container images for the Scality S3 CSI Driver are hosted on GHCR:

| Driver Version | Image URL                                                                 |
|----------------|---------------------------------------------------------------------------|
| v0.2.0 (latest)| `ghcr.io/scality/mountpoint-s3-csi-driver:0.2.0`                        |
| v0.1.0         | `ghcr.io/scality/mountpoint-s3-csi-driver:0.1.0`                        |

*Note: Please check the [releases page](https://github.com/scality/mountpoint-s3-csi-driver/releases) for the latest available versions.*

## Support and Community

For issues or questions:

<!-- TODO: Create troubleshooting.md file -->
<!-- 1. Check the [Troubleshooting Guide](troubleshooting.md) -->
1. Search existing [GitHub Issues](https://github.com/scality/mountpoint-s3-csi-driver/issues)
2. Open a new [GitHub Issue](https://github.com/scality/mountpoint-s3-csi-driver/issues) if the problem is not already addressed

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](https://github.com/scality/mountpoint-s3-csi-driver/blob/main/LICENSE) file for details.
It incorporates code from the original Mountpoint for Amazon S3 CSI Driver, also licensed under Apache 2.0.
