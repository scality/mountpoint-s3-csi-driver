# System Overview

The Scality CSI Driver for S3 enables Kubernetes applications to use Scality RING S3 buckets as persistent volumes through the [Container Storage Interface (CSI) specification](https://github.com/container-storage-interface/spec/blob/master/spec.md).

<div align="center">

```mermaid
graph TB

    %% External entities
    APIServer[Kubernetes API Server]
    S3[RING S3 Storage Endpoint]

    %% CSI Components
    subgraph CSI["Scality CSI Driver"]
        subgraph Controller["Controller Deployment"]
            CSIController["CSI Controller Service"]
            CSIProvisioner["CSI Provisioner Sidecar"]
        end
        Node["Node Service: Mounts and unmounts volumes"]
    end

    %% Host Integration
    Host["Host OS with systemd and FUSE filesystem"]

    %% Application
    App[Application Pods]

    %% Dynamic Provisioning Flow
    APIServer <-->|"Watch PVC/StorageClass, Create PV (Dynamic Provisioning)"| CSIProvisioner
    CSIProvisioner -->|"CreateVolume/DeleteVolume RPC"| CSIController
    CSIController -->|"Create/Delete S3 Buckets"| S3

    %% Node Operations
    APIServer <-->|CSI Protocol (Dynamic and Static Provisioning)| Node
    Node -->|Creates S3 FUSE services| Host
    Host <-->|S3 API| S3
    App -->|File I/O| Host

```

</div>

## Core Components

| Component | Responsibility | Details |
|-----------|----------------|---------|
| **CSI Driver Controller Service** | Dynamic Volume Provisioning | • Handles CreateVolume/DeleteVolume RPCs for dynamic provisioning<br>• Creates and deletes S3 buckets automatically based on StorageClass parameters<br>• Manages credential templating and secret resolution for multi-tenancy<br>• Enabled by default in Helm deployment |
| **CSI Driver Node Service** | RING S3 FUSE Operations | • Receives mount requests from kubelet via gRPC<br>• Creates systemd transient services for each volume mount<br>• Configures mount-s3 with appropriate credentials and cache settings<br>• Verifies mount point existence<br>• Handles unmount operations during pod termination |
| **Host Integration** | RING S3 Access Layer | • Runs mount-s3 processes as systemd transient services<br>• Provides POSIX-compliant filesystem interface through FUSE kernel module<br>• Maintains per-volume isolation with separate processes and credentials<br>• Translates file operations to S3 API calls with optional caching |

## S3 Volume Setup Flow

### Dynamic Provisioning

| Step | Action | Description |
|------|--------|-------------|
| 1 | **StorageClass Creation** | Administrator creates StorageClass referencing CSI provisioner, parameters, and secret configurations |
| 2 | **PVC Creation** | User creates PersistentVolumeClaim referencing the StorageClass |
| 3 | **Volume Provisioning** | CSI Provisioner watches PVC, resolves template variables, and calls CreateVolume RPC |
| 4 | **Bucket Creation** | CSI Controller Service creates S3 bucket based on StorageClass parameters |
| 5 | **PV Creation** | CSI Provisioner Sidecar creates PersistentVolume object after successful bucket creation |
| 6 | **Volume Mounting** | Kubernetes schedules pod, kubelet calls NodePublishVolume via CSI protocol |
| 7 | **Mount Service** | CSI Node Service creates systemd service to run mount-s3 process |
| 8 | **S3 Connection** | mount-s3 process authenticates and establishes connection to S3 bucket |
| 9 | **File Access** | Applications read/write files normally, which are translated to S3 API operations |

### Static Provisioning

| Step | Action | Description |
|------|--------|-------------|
| 1 | **Manual Bucket Setup** | Administrator pre-creates S3 bucket and PersistentVolume object |
| 2 | **PVC Binding** | User creates PVC that binds to existing PV |
| 3 | **Volume Mounting** | Kubernetes schedules pod, kubelet calls NodePublishVolume via CSI protocol |
| 4 | **Mount Service** | CSI Node Service creates systemd service to run mount-s3 process |
| 5 | **S3 Connection** | mount-s3 process authenticates and establishes connection to S3 bucket |
| 6 | **File Access** | Applications read/write files normally, which are translated to S3 API operations |
