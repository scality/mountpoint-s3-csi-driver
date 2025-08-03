# System Overview

The Scality CSI Driver for S3 enables Kubernetes applications to use Scality RING S3 buckets as persistent volumes through the [Container Storage Interface (CSI) specification](https://github.com/container-storage-interface/spec/blob/master/spec.md).

<div align="center">

```mermaid
graph TB

    %% External entities
    K8s[Kubernetes Control Plane]
    S3[RING S3 Storage Endpoint]

    %% CSI Components
    subgraph CSI["Scality CSI Driver"]
        Controller["Controller Service: Reports capabilities (minimal)"]
        Node["Node Service: Mounts and unmounts volumes"]
    end

    %% Host Integration
    Host["Host OS with systemd and FUSE filesystem"]

    %% Application
    App[Application Pods]

    %% Connections
    K8s <-->|CSI Protocol| Controller
    K8s <-->|CSI Protocol| Node
    Node -->|Creates S3 FUSE services| Host
    Host <-->|S3 API| S3
    App -->|File I/O| Host

```

</div>

## Core Components

| Component | Responsibility | Details |
|-----------|----------------|---------|
| **CSI Driver Controller Service** | Volume Management | • Reports supported capabilities to Kubernetes (minimal functionality)<br>• Implements CSI controller interface for compatibility<br> |
| **CSI Driver Node Service** | RING S3 FUSE Operations | • Receives mount requests from kubelet via gRPC<br>• Creates systemd transient services for each volume mount<br>• Configures mount-s3 with appropriate credentials and cache settings<br>• Verifies mount point existence<br>• Handles unmount operations during pod termination |
| **Host Integration** | RING S3 Access Layer | • Runs mount-s3 processes as systemd transient services<br>• Provides POSIX-compliant filesystem interface through FUSE kernel module<br>• Maintains per-volume isolation with separate processes and credentials<br>• Translates file operations to S3 API calls with optional caching |

## S3 Volume Setup Flow

| Step | Action | Description |
|------|--------|-------------|
| 1 | **Volume Request** | Kubernetes requests a volume mount through the CSI protocol |
| 2 | **Mount Creation** | The CSI Driver Node Service creates a systemd service to run mount-s3 process |
| 3 | **S3 Connection** | mount-s3 process authenticates and establishes connection to S3 endpoint |
| 4 | **File Access** | Applications read/write files normally, which are translated to S3 operations |
