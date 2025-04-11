# Mounter Package

This package implements the mounting functionality for the S3 CSI Driver.

## Overview

The mounter package provides interfaces and implementations for mounting S3 buckets to the host filesystem. It abstracts the underlying mounting mechanisms and provides different strategies for different environments.

## Core Interface

The package defines the `Mounter` interface:

```go
type Mounter interface {
    Mount(ctx context.Context, bucketName string, target string, provideCtx credentialprovider.ProvideContext, args mountpoint.Args) error
    Unmount(ctx context.Context, target string, cleanupCtx credentialprovider.CleanupContext) error
    IsMountPoint(target string) (bool, error)
}
```

This interface allows the driver to:
- Mount S3 buckets to the specified target path
- Unmount previously mounted buckets
- Check if a path is a mount point

## Mounter Implementations

The package provides two primary implementations:

TODO: Add clarification that systemd mounter is within the CSI pod
### 1. SystemdMounter

The SystemdMounter uses systemd units to manage Mountpoint for S3 processes. It:

- Creates systemd unit files to run Mountpoint for S3
- Uses systemd to start and stop these units
- Provides proper lifecycle management for the mount processes
- Handles credentials securely

This mounter is used by default and is suitable for most environments.

### 2. PodMounter

The PodMounter manages Mountpoint for S3 in pods. It:

- Creates pods in a dedicated namespace to run Mountpoint for S3
- Manages the lifecycle of these pods
- Provides proper isolation between different mount instances
- Integrates with the Kubernetes pod model

This mounter is available for Kubernetes ≥ 1.24 and can be enabled by setting the environment variable `MOUNTER_KIND=pod`.

### Testing Support

The package also includes a `FakeMounter` implementation for testing purposes that simulates mounting behavior without actually performing any mounting operations.

## Configuration

The mounter selection is determined by the `MOUNTER_KIND` environment variable:
- If set to `pod`, the PodMounter is used
- Otherwise, the SystemdMounter is used

## Mount Arguments

Mounters accept mount arguments via the `mountpoint.Args` structure, which provides a standardized way to pass options to the Mountpoint for S3 binary, including:

- Cache configuration
- Performance settings
- Log level
- Region
- Custom endpoint
- Authentication options
- Access control options

## Relationship to Other Packages

The mounter package works closely with:
- `credentialprovider`: For obtaining S3 authentication credentials
- `mountpoint`: For handling Mountpoint-specific arguments
- `podmounter`: For implementing the pod-based mounting strategy

## See Also

- [Node Service](../README.md) - The main node service that uses these mounter implementations
- [Credential Provider](../credentialprovider/README.md) - Provides authentication credentials for S3 access
