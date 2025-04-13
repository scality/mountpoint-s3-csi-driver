# S3 CSI Driver Architecture

This document provides an overview of the architecture and design of the Mountpoint for Amazon S3 CSI Driver.

## Overview

The S3 CSI Driver implements the [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec) to allow Kubernetes to mount S3 buckets onto pods using [Mountpoint for Amazon S3](https://github.com/awslabs/mountpoint-s3).

The driver follows a **static provisioning** model, where existing S3 buckets are mounted to pods rather than dynamically creating buckets on demand.

## Key Components

The driver consists of several key components:

### 1. CSI Driver Implementation (`pkg/driver`)

The core CSI implementation is divided into three services as per the CSI specification:

- **[Identity Service](../pkg/driver/README.md#identitygo)**: Provides information about the driver's identity and capabilities
- **[Controller Service](../pkg/driver/README.md#controllergo)**: Minimal implementation that returns `Unimplemented` for most operations
- **[Node Service](../pkg/driver/node/README.md)**: Handles the mounting/unmounting of S3 buckets to nodes

For more details about the driver implementation, see the [driver README](../pkg/driver/README.md).

### 2. Mounting System

The driver supports two methods for mounting S3 buckets:

- **SystemdMounter**: Uses systemd units to manage Mountpoint for S3 processes
- **PodMounter**: Manages Mountpoint for S3 in pods (for Kubernetes ≥ 1.24)

### 3. Authentication System

The driver offers multiple methods for authenticating to S3:

- Driver-level credentials via IAM roles for service accounts (IRSA)
- Driver-level credentials via Kubernetes secrets
- Driver-level credentials via node instance profiles
- Pod-level credentials for multi-tenant setups

## Design Decisions

### Static Provisioning Only

The S3 CSI Driver is designed for static provisioning only, meaning it doesn't implement dynamic provisioning of S3 buckets. This design decision was made for several reasons:

1. S3 buckets are typically pre-created by administrators with specific naming, permissions, and lifecycle policies
2. Bucket creation is generally considered an administrative task
3. The static provisioning approach aligns better with how object storage is typically used in practice

### Skipping Controller Tests

Due to the static provisioning design, most controller-related sanity tests are skipped as described in the [Sanity Tests documentation](../tests/sanity/README.md).

## Deployment Components

When deployed, the S3 CSI Driver consists of:

- A DaemonSet running the node service on each node
- (Optional) A controller deployment for identity service and minimal controller implementation
- Service accounts and RBAC permissions
- CSI registration mechanism via CSIDriver object

## Usage Flow

1. Administrator creates an S3 bucket
2. Administrator creates a PersistentVolume (PV) pointing to the S3 bucket
3. User creates a PersistentVolumeClaim (PVC) referencing the PV
4. User deploys a pod using the PVC
5. When the pod is scheduled, the CSI driver:
   - Uses the appropriate credentials to authenticate to S3
   - Mounts the S3 bucket at the specified path using Mountpoint for S3
   - Makes the S3 bucket accessible as a filesystem to the pod

## Component Documentation

For more detailed information about specific components:

- [Driver Implementation](../pkg/driver/README.md) - Main driver service implementations
- [Node Service](../pkg/driver/node/README.md) - Details about the node service implementation
- [Version Package](../pkg/driver/version/README.md) - Version information handling
- [Sanity Tests](../tests/sanity/README.md) - Information about CSI conformance testing

## Configuration

For details on configuring the driver, see:

- [Configuration Guide](CONFIGURATION.md) - Complete configuration options
- [Logging Guide](LOGGING.md) - Information about driver logging
- [Installation Guide](install.md) - Installation instructions
