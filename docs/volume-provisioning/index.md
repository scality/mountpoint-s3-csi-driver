# Volume Provisioning Overview

The Scality CSI Driver for S3 supports two methods for creating and managing persistent volumes: **Static Provisioning** and **Dynamic Provisioning**.

## Provisioning Methods Comparison

| Aspect | Static Provisioning | Dynamic Provisioning |
|--------|--------------------|--------------------|
| **Bucket Creation** | Pre-created S3 bucket required | Bucket created automatically |
| **Kubernetes Resources** | Manual PV + PVC creation | Automatic PV creation from PVC |
| **Use Case** | Fixed buckets, shared storage | On-demand storage, isolated workloads |
| **Management Overhead** | Higher (manual PV management) | Lower (automated) |
| **Resource Lifecycle** | Bucket persists after PV deletion | Configurable via reclaim policy (bucket deletion only occurs if empty) |
| **Multi-tenancy** | Manual bucket isolation | Built-in isolation per PVC |
| **Credential Management** | Per-volume secrets or global | StorageClass-level configuration |

## Getting Started

<!-- markdownlint-disable MD046 -->
!!! tip "Quick Navigation"
    **Static Provisioning:**

    - [Overview & Examples](static-provisioning/overview.md) - Basic concepts and step-by-step examples
    - [Credentials Management](../architecture/ring-s3-credentials-management/static-provisioning-credentials-management.md) - Driver-level vs volume-level authentication

    **Dynamic Provisioning:**

    - [Overview & Examples](dynamic-provisioning/overview.md) - StorageClass setup and workflows  
    - [Credentials Management](../architecture/ring-s3-credentials-management/dynamic-provisioning-credentials-management.md) - Template-based and fixed authentication methods

    **Common Configuration:**

    - [Mount Options Reference](mount-options.md) - Customization options for both provisioning methods
    - [TLS Configuration](tls-configuration.md) - Configure custom CA certificates for HTTPS S3 endpoints
<!-- markdownlint-enable MD046 -->

### Quick Start: Static Provisioning

1. Create a PersistentVolume pointing to an existing bucket
2. Create a PersistentVolumeClaim that binds to the PV
3. Mount the PVC in the pod

### Quick Start: Dynamic Provisioning

1. Create a StorageClass with S3 provisioner configuration
2. Create a PersistentVolumeClaim referencing the StorageClass
3. Using the CSI Driver, Kubernetes automatically creates the bucket and PV
4. Mount the PVC in the pod
