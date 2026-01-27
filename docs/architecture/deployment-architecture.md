# Deployment Architecture

This document illustrates the deployment topology of the Scality CSI Driver for S3, showing how components are distributed across a Kubernetes cluster.
The architecture differs between static and dynamic provisioning modes.

<div align="center">

```mermaid
graph TB
    subgraph cluster["Kubernetes Cluster"]

        subgraph controlplane["Kubernetes Control Plane"]
            APIServer["Kubernetes API Server"]
            CRD[(MountpointS3PodAttachment CRD)]
        end

        subgraph node1["Kubernetes Node 1"]
            K1[Kubelet]
            subgraph ds1["CSI Driver Pod (DaemonSet)"]
                N1[CSI Driver Node Service]
                R1[CSI Driver Registrar Sidecar]
                L1[CSI Driver Liveness Probe]
            end
            MP1["Mountpoint Pods (ns: mount-s3)"]
            A1[Application Pods]
        end

        subgraph node2["Kubernetes Node 2"]
            subgraph controller["CSI Controller Deployment"]
                subgraph controllerPod["Controller Pod (1 replica)"]
                    CSIController["CSI Controller Service"]
                    PodReconciler["Pod Reconciler"]
                    CSIProvisioner["CSI Provisioner Sidecar"]
                end
            end
            K2[Kubelet]
            subgraph ds2["CSI Driver Pod (DaemonSet)"]
                N2[CSI Driver Node Service]
                R2[CSI Driver Registrar Sidecar]
                L2[CSI Driver Liveness Probe]
            end
            MP2["Mountpoint Pods (ns: mount-s3)"]
            A2[Application Pods]
        end

        subgraph node3["Kubernetes Node N..."]
            K3[Kubelet]
            subgraph ds3["CSI Driver Pod (DaemonSet)"]
                N3[CSI Driver Node Service]
                R3[CSI Driver Registrar Sidecar]
                L3[CSI Driver Liveness Probe]
            end
            MP3["Mountpoint Pods (ns: mount-s3)"]
            A3[Application Pods]
        end

    end

    S3Storage[S3 Storage Endpoint]

    %% Controller operations (Dynamic Provisioning)
    APIServer <-->|"Watch PVC/StorageClass, Create PV"| CSIProvisioner
    CSIProvisioner -->|"CreateVolume/DeleteVolume RPC"| CSIController
    CSIController -->|"Bucket Create/Delete via S3 API"| S3Storage

    %% Pod Reconciler operations
    PodReconciler -->|"Watch workload Pods, Create CRD"| CRD
    PodReconciler -->|"Creates"| MP2

    %% CSI Driver Registration
    R1 -->|Register via /registration/ entry| K1
    R2 -->|Register via /registration/ entry| K2
    R3 -->|Register via /registration/ entry| K3

    %% Health monitoring
    L1 -->|Monitor Unix socket /csi/csi.sock| N1
    L2 -->|Monitor Unix socket /csi/csi.sock| N2
    L3 -->|Monitor Unix socket /csi/csi.sock| N3

    %% Node operations
    K1 -->|Volume requests via gRPC| N1
    K2 -->|Volume requests via gRPC| N2
    K3 -->|Volume requests via gRPC| N3

    N1 -->|Wait for CRD assignment| CRD
    N2 -->|Wait for CRD assignment| CRD
    N3 -->|Wait for CRD assignment| CRD

    N1 -->|Bind mount to app| A1
    N2 -->|Bind mount to app| A2
    N3 -->|Bind mount to app| A3

    %% Application access via bind mounts
    A1 -->|File I/O via bind mount| MP1
    A2 -->|File I/O via bind mount| MP2
    A3 -->|File I/O via bind mount| MP3

    %% S3 connections
    MP1 -->|S3 API| S3Storage
    MP2 -->|S3 API| S3Storage
    MP3 -->|S3 API| S3Storage

    %% Styling for clarity without colors
    classDef optional stroke-dasharray: 5 5
```

</div>

## Deployment Components

### Controller Components

| Component | Type | Purpose | Details |
|-----------|------|---------|---------|
| **CSI Controller Service** | Main Container | Volume lifecycle management | Binary: `scality-s3-csi-driver` with `CSI_CONTROLLER_ONLY=true`. Handles CreateVolume/DeleteVolume RPCs for dynamic provisioning. Creates and deletes S3 buckets based on StorageClass parameters. Manages provisioner and node-publish secrets from StorageClass. Single replica Deployment (not DaemonSet). |
| **Pod Reconciler** | Main Container | Mountpoint Pod lifecycle | Binary: `scality-csi-controller`. Watches workload Pods (not CRDs). When a workload needs an S3 volume, creates Mountpoint Pod first, then creates MountpointS3PodAttachment CRD with assignment. Manages pod placement, resource allocation, and cleanup. Handles volume sharing by reusing Mountpoint Pods for matching workloads. |
| **CSI Provisioner Sidecar** | Sidecar Container | Kubernetes integration | Standard `csi-provisioner` from Kubernetes. Watches for PVCs that need dynamic provisioning. Reads StorageClass parameters and templates. Resolves template variables (`${pvc.name}`, `${pvc.namespace}`, `${pv.name}`, etc.). Calls CSI Controller's CreateVolume/DeleteVolume. Creates PV objects after successful bucket creation. |

### Node Components

| Component | Type | Purpose | Details |
|-----------|------|---------|---------|
| **CSI Driver Node Service** | Main Container | Core CSI functionality | Binary: `scality-s3-csi-driver`. Creates gRPC server on `/csi/csi.sock` Unix socket file. Exposes HTTP `/healthz` endpoint for Kubernetes liveness probe. Handles volume mount requests by waiting for MountpointS3PodAttachment CRD (created by Pod Reconciler). Sends mount options to Mountpoint Pod via Unix socket. Creates bind mounts from source directory to container target paths. Handles unmount by removing bind mounts. |
| **CSI Driver Registrar** | Sidecar | Kubelet registration | Creates registration entry in `/registration/` directory watched by kubelet. Registration entry announces CSI driver name `s3.csi.scality.com` and Unix socket location. Maintains registration while driver is deployed on node. Uses standard Kubernetes CSI node-driver-registrar sidecar. |
| **CSI Driver Liveness Probe** | Sidecar | CSI socket health logging | Checks CSI Driver Node Service via `/csi/csi.sock` Unix socket file. Logs health status to container logs for troubleshooting. Does NOT trigger pod restarts (logging only). |

### Mountpoint Pods

| Scope | Component | Purpose | Details |
|-------|-----------|---------|---------|
| **Per Volume (shared)** | Mountpoint Pod | S3 filesystem mounting | Dedicated pod running `mount-s3` FUSE process. Created by Pod Reconciler in the `mount-s3` namespace (configurable via `mountpointPod.namespace` Helm value). Mounts S3 bucket to source directory at `/var/lib/kubelet/plugins/s3.csi.scality.com/mnt/<pod-name>`. Can serve multiple workload pods with matching configurations. Provides POSIX-compliant filesystem interface through FUSE. Handles S3 API communication, caching, and file system semantics. |

### Custom Resource Definition

| Resource | Scope | Purpose | Details |
|----------|-------|---------|---------|
| **MountpointS3PodAttachment** | Cluster-scoped | Volume attachment tracking | Tracks which workload pods are attached to which Mountpoint Pods. Contains node name, PV name, volume ID, mount options, and fsGroup. Enables volume sharing across workloads with matching configurations. Short name: `s3pa`. Created by Pod Reconciler, Node Service waits for assignment. |

## Key Deployment Characteristics

### Resource Distribution

| Resource Scope | What Gets Deployed | Deployment Method | When Required |
|----------------|-------------------|-------------------|---------------|
| **Cluster-wide** | One CSI Controller pod | Deployment (1 replica) | Always (contains Pod Reconciler) |
| **Cluster-wide** | MountpointS3PodAttachment CRD | CustomResourceDefinition | Always |
| **Per Kubernetes Node** | One CSI Driver pod | DaemonSet | Always |
| **Per Volume (shared)** | One Mountpoint Pod | Created by Pod Reconciler | Per unique volume/node/options combination |

### Communication Paths

| Path | From | To | Protocol | Purpose | Provisioning Mode |
|------|------|----|----------|---------|-------------------|
| **PVC Monitoring** | CSI Provisioner Sidecar | Kubernetes API | HTTPS | Watch PVC/StorageClass events | Dynamic only |
| **Volume Provisioning** | CSI Provisioner Sidecar | CSI Controller Service | gRPC on Unix socket `/csi/csi.sock` | CreateVolume/DeleteVolume | Dynamic only |
| **Bucket Operations** | CSI Controller Service | S3 endpoint | HTTPS | Create/delete S3 buckets | Dynamic only |
| **Pod Watch** | Pod Reconciler | Kubernetes API | HTTPS | Watch workload Pods with S3 volumes | Both |
| **Mountpoint Pod Management** | Pod Reconciler | Kubernetes API | HTTPS | Create/delete Mountpoint Pods | Both |
| **CSI Driver Registration** | CSI Driver Registrar | Kubelet | Unix socket `/registration/` | Register driver per Kubernetes node | Both |
| **Volume Operations** | Kubelet | CSI Driver Node Service | gRPC on Unix socket | Mount/unmount requests | Both |
| **CRD Wait** | CSI Driver Node Service | Kubernetes API | HTTPS | Wait for MountpointS3PodAttachment assignment | Both |
| **Health Monitoring** | CSI Driver Liveness Probe | CSI Driver Node Service | gRPC on Unix socket `/csi/csi.sock` | Health status checks | Both |
| **File I/O** | Application pods | Mountpoint Pods | Bind mount + FUSE | File system operations | Both |
| **Storage Access** | Mountpoint Pods | S3 endpoint | HTTPS | S3 API calls | Both |

### Host Mounts Required

| Host Path | Purpose | Used By |
|-----------|---------|---------|
| `/var/lib/kubelet/plugins/s3.csi.scality.com/` | CSI driver socket and mount source directories | CSI Driver Node Service (creates gRPC socket and source mounts), kubelet (volume operations), CSI Driver Registrar (driver registration), CSI Driver Liveness Probe (health checks) |
| `/var/lib/kubelet/plugins/s3.csi.scality.com/mnt/<mp-pod-name>/` | Source mount directory for S3 bucket | Mountpoint Pod (FUSE mount point), CSI Driver Node Service (creates bind mounts from here) |
| `/var/lib/kubelet/pods/<pod-id>/volumes/kubernetes.io~csi/<volume-id>/mount/` | Target mount point for application access | kubelet (creates mount point directory), CSI Driver Node Service (creates bind mount to here), Application pods (file I/O) |

### Scaling Behavior

| Resource | Scaling Behavior | Mechanism | Notes |
|----------|------------------|-----------|-------|
| **CSI Controller** | Single instance | Deployment with 1 replica | One controller needed cluster-wide |
| **Kubernetes Nodes** | Automatic deployment to new nodes | DaemonSet controller | One CSI node pod per Kubernetes node |
| **Mountpoint Pods** | One per unique volume/node/options | Created by Pod Reconciler | Multiple workloads can share one Mountpoint Pod |

## Static vs Dynamic Provisioning

### Static Provisioning

- Controller deployment still required (for Pod Reconciler)
- DaemonSet for node pods
- Administrator pre-creates S3 buckets
- PersistentVolumes reference existing buckets

### Dynamic Provisioning

- Controller deployment required (CSI Controller Service + Pod Reconciler)
- Controller creates/deletes S3 buckets automatically
- StorageClass defines bucket creation parameters
- Supports credential templating for multi-tenancy

### Credential Flow Differences

| Aspect | Static Provisioning | Dynamic Provisioning |
|--------|--------------------|-----------------------|
| **Bucket Creation** | Manual by admin | Automatic by CSI Controller |
| **Credential Sources** | Driver-level (global), PV-level (nodePublishSecretRef) | Driver-level (global), StorageClass provisioner secrets, StorageClass node-publish secrets, Template-based secrets |
| **Secret Resolution** | At mount time by CSI Node | Provisioner secrets at CreateVolume, Node-publish secrets at mount time |
| **Multi-tenancy** | Per-PV secrets | Per-StorageClass or per-PVC templated secrets |
