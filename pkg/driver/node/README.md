# Node Service Implementation

This directory contains the Node Service implementation for the S3 CSI Driver.

## Overview

In the Container Storage Interface (CSI) specification, the Node Service is responsible for the mount/unmount operations on a specific node. The key operations include:

- `NodePublishVolume`: Mounts the volume to a specified path on the node
- `NodeUnpublishVolume`: Unmounts the volume from the node
- `NodeGetCapabilities`: Reports the capabilities of the node service
- `NodeGetInfo`: Provides information about the node

The Node Service runs as a DaemonSet in Kubernetes, ensuring that it's available on every node where volumes need to be mounted.

## Implementation in S3 CSI Driver

In this driver, the Node Service (implemented in `node.go` and supporting files) is the primary focus of the driver's functionality. Since the S3 CSI Driver uses **static provisioning**, the Node Service is responsible for mounting pre-existing S3 buckets to pods.

### Key Components in the Node Service

The Node Service implementation includes several important components:

1. **S3NodeServer** (`node.go`): The main implementation of the CSI Node Service interface
2. **Mounter Interface** ([mounter/](mounter/README.md)): Abstracts the mounting functionality with two implementations:
   - `SystemdMounter`: Uses systemd units to manage Mountpoint for S3 processes with in the CSI driver pod (TODO: verify this)
   - `PodMounter`: Manages Mountpoint for S3 in pods (for Kubernetes ≥ 1.24)
3. **Credential Provider** ([credentialprovider/](credentialprovider/README.md)): Handles authentication to S3
4. **Volume Context** ([volumecontext/](volumecontext/README.md)): Manages the context attributes for volumes
5. **Target Path** ([targetpath/](targetpath/README.md)): Parses and validates CSI target paths
6. **Environment Provider** ([envprovider/](envprovider/README.md)): Manages environment variables for S3 credentials

### Node Service Capabilities

The Node Service in this driver offers these key capabilities:

- Mounting S3 buckets as filesystems in pods
- Support for access modes:
  - `MULTI_NODE_MULTI_WRITER` (ReadWriteMany)
  - `MULTI_NODE_READER_ONLY` (ReadOnlyMany)
- Mount options to customize S3 access behavior
- Multiple authentication methods

### Implementation Details

The `NodePublishVolume` method, which is the core of the Node Service:

1. Extracts the S3 bucket name and other parameters from the volume context
2. Validates the target path and volume capabilities
3. Constructs the appropriate mount options
4. Invokes the appropriate mounter to connect to S3

The `NodeUnpublishVolume` method handles unmounting:

1. Checks if the target path is actually mounted
2. Invokes the mounter to unmount the S3 bucket
3. Cleans up any credentials

## Static Provisioning Workflow

When using static provisioning with this driver:

1. A user creates a PersistentVolume (PV) with details of an existing S3 bucket
2. A user creates a PersistentVolumeClaim (PVC) that references the PV
3. A pod uses the PVC in its volume definition
4. When the pod is scheduled to a node:
   - Kubernetes calls the driver's `NodePublishVolume` method
   - The Node Service mounts the S3 bucket at the specified path
   - The pod can access the S3 bucket as a regular filesystem

## Subpackages

- [mounter/](mounter/README.md) - Handles the mounting of S3 buckets
- [credentialprovider/](credentialprovider/README.md) - Manages S3 authentication credentials
- [volumecontext/](volumecontext/README.md) - Defines volume context attributes
- [targetpath/](targetpath/README.md) - Parses and validates CSI target paths
- [envprovider/](envprovider/README.md) - Manages environment variables for credentials

## See Also

- [Driver Implementation](../README.md) - Main driver service implementations
- [Sanity Tests](../../../tests/sanity/README.md) - Information about CSI conformance testing
