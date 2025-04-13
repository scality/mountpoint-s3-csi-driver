# Driver Implementation

This directory contains the core implementation of the S3 CSI Driver.

## Overview

The S3 CSI Driver implements the [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec) specification, which defines a standard interface for container orchestration systems (like Kubernetes) to interact with storage providers.

The driver is divided into three main services as per the CSI specification:

1. **Identity Service** - Provides information about the driver's identity and capabilities
2. **Controller Service** - Handles volume lifecycle management (minimal implementation in this driver)
3. **Node Service** - Handles mounting/unmounting of volumes on nodes

## Files in This Directory

### controller.go

Contains the Controller Service implementation. In the S3 CSI Driver, most controller methods return `codes.Unimplemented` as the driver focuses on **static provisioning** only.

#### Controller Service Details

The Controller Service is responsible for volume lifecycle management operations including:

- Creating volumes (`CreateVolume`)
- Deleting volumes (`DeleteVolume`)
- Publishing volumes to nodes (`ControllerPublishVolume`)
- Unpublishing volumes from nodes (`ControllerUnpublishVolume`)
- Expanding volumes (`ControllerExpandVolume`) 
- Creating and managing snapshots (`CreateSnapshot`, `DeleteSnapshot`, etc.)

##### Why Controller Operations Return Unimplemented

The S3 CSI Driver is designed to mount pre-existing S3 buckets to Kubernetes pods. It does not implement dynamic provisioning, which would involve creating and deleting S3 buckets on demand. 

The reasons for this design choice include:

1. **Administrative Control**: S3 bucket creation/deletion is typically managed by administrators rather than automated by Kubernetes
2. **Bucket Naming**: S3 buckets have globally unique naming requirements that are better handled manually
3. **Policy Configuration**: Bucket policies, lifecycle rules, and permissions are usually carefully controlled
4. **Cost Management**: Direct control over bucket creation helps manage costs and prevent unintended resource usage

Although most controller methods return `Unimplemented`, the controller does minimally implement:

- `ControllerGetCapabilities`: Returns minimal capabilities to satisfy the CSI specification
- `ValidateVolumeCapabilities`: Provides basic validation

This minimal implementation affects the [sanity tests](../../tests/sanity/README.md), where many controller tests are skipped with the following flags in the Makefile:

```
go test -v ./tests/sanity/... -ginkgo.skip="ControllerGetCapabilities" -ginkgo.skip="ValidateVolumeCapabilities"
```

### identity.go

Contains the Identity Service implementation. This service provides information about the driver itself and its capabilities.

The Identity Service implements:

- `GetPluginInfo`: Returns the name and version of the driver
- `GetPluginCapabilities`: Reports the capabilities of the driver (e.g., whether it can create volumes)
- `Probe`: Indicates whether the driver is healthy and ready to serve requests

This is a critical component as it's the first point of contact when the container orchestrator interacts with the driver.

### driver.go

Contains the core Driver structure and initialization. This file:

1. Defines the main `Driver` structure that holds references to all services
2. Implements startup and shutdown logic
3. Sets up the gRPC server for CSI communication
4. Initializes the credential providers and mounters

The driver connects to the Kubernetes API server to facilitate credential management and pod watching capabilities.

### server.go

Contains utility functions for setting up the server component of the driver, including:

- Parsing endpoints for the gRPC server
- Handling Unix domain socket communication

## Subpackages

This directory contains the following subpackages:

- [node](node/README.md) - Contains the Node Service implementation for mounting S3 buckets to pods
- [version](version/README.md) - Contains version information handling for the driver

## Static Provisioning Usage

Since the controller does not implement dynamic provisioning, users must:

1. Create S3 buckets manually outside of Kubernetes
2. Create PersistentVolumes (PVs) with `storageClassName: ""` (empty string)
3. Create PersistentVolumeClaims (PVCs) with corresponding volume references

For usage examples, see the [static provisioning examples](../../examples/kubernetes/static_provisioning/) directory.
