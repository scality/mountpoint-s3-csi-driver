# Dynamic Provisioning Architecture

This document details the complete lifecycle of dynamically provisioned volumes in the Scality CSI Driver for S3, from StorageClass creation to application file access and eventual cleanup.

<div align="center">

```mermaid
sequenceDiagram
    participant Admin as Kubernetes Administrator
    participant K8s as Kubernetes API Server
    participant Dev as Kubernetes Developer
    participant Provisioner as CSI Provisioner Sidecar
    participant Controller as CSI Controller Service
    participant S3 as RING S3 endpoint
    participant Reconciler as Pod Reconciler
    participant Kubelet as Kubelet (Node Agent)
    participant CSI as CSI Driver Node Service
    participant MPPod as Mountpoint Pod (ns: mount-s3)
    participant App as Application Pod

    %% StorageClass Setup Phase
    note over Admin,K8s: === StorageClass Setup Phase (One-time) ===
    Admin->>K8s: Create StorageClass with provisioner, parameters, secrets
    K8s->>K8s: Validate and store StorageClass

    %% Volume Provisioning Phase
    note over Dev,S3: === Dynamic Volume Provisioning Phase ===
    Dev->>K8s: Create PersistentVolumeClaim referencing StorageClass
    K8s->>K8s: Store PVC (status: Pending)
    Provisioner->>K8s: Watch for new PVCs
    K8s-->>Provisioner: PVC event notification
    Provisioner->>K8s: Read StorageClass parameters
    Provisioner->>K8s: Resolve template variables (${pvc.name}, ${pvc.namespace}) if specified
    Provisioner->>K8s: Fetch provisioner secrets if specified
    Provisioner->>Controller: CreateVolume RPC via Unix socket /csi/csi.sock
    Controller->>Controller: Validate request and parameters
    Controller->>Controller: Generate unique Volume ID (csi-s3-{uuid})
    Controller->>Controller: Resolve credentials (from secrets or driver-level)
    Controller->>S3: Create S3 bucket using Volume ID as bucket name
    S3-->>Controller: Bucket created successfully
    Controller-->>Provisioner: CreateVolume response with Volume ID
    Provisioner->>K8s: Create PersistentVolume object
    alt Volume Binding Mode: Immediate
        K8s->>K8s: Bind PV and PVC atomically
        K8s-->>Dev: PVC Bound (Reserved)
    else Volume Binding Mode: WaitForFirstConsumer
        K8s->>K8s: Keep PVC Pending until pod is scheduled
        K8s-->>Dev: PVC remains Pending (PV Available but unbound)
    end

    %% Pod Scheduling Phase
    note over Dev,Kubelet: === Pod Scheduling Phase ===
    Dev->>K8s: Create Pod with PVC
    K8s->>K8s: Schedule Pod to Node
    alt Volume Binding Mode: WaitForFirstConsumer
        K8s->>K8s: Bind PV and PVC now that pod is scheduled
        K8s-->>Dev: PVC now Bound (Reserved)
    end
    K8s->>Kubelet: Pod assignment

    %% Pod Reconciler detects workload
    note over Reconciler,MPPod: === Mountpoint Pod Provisioning ===
    Reconciler->>K8s: Watching all Pods in cluster
    K8s-->>Reconciler: Workload Pod event (scheduled, needs S3 volume)
    Reconciler->>Reconciler: Check for existing CRD with matching config
    Reconciler->>K8s: Create Mountpoint Pod on same node
    K8s->>MPPod: Schedule Mountpoint Pod
    Reconciler->>K8s: Create MountpointS3PodAttachment CRD with Pod assignment

    %% Volume Mount Phase
    note over Kubelet,MPPod: === Volume Mount Phase ===
    Kubelet->>CSI: NodePublishVolume RPC (triggered by pod start request)
    CSI->>CSI: Validate request and prepare credentials
    CSI->>K8s: Wait for MountpointS3PodAttachment CRD with assignment
    K8s-->>CSI: CRD found with Mountpoint Pod name
    CSI->>MPPod: Send mount options via Unix socket
    MPPod->>S3: Authenticate and connect to bucket
    S3-->>MPPod: Connection established
    MPPod->>MPPod: Create FUSE mount at source directory
    CSI->>CSI: Create bind mount from source to target
    CSI-->>Kubelet: Volume mounted and ready

    %% Application Access Phase
    note over Kubelet,App: === Application Access Phase ===
    Kubelet->>App: Start application container (volume accessible at mount path)
    App->>MPPod: File operations via bind mount
    MPPod->>S3: S3 API calls
    S3-->>MPPod: Data transfer
    MPPod-->>App: Folder (prefix) and File (object) data

    %% Cleanup Phase - Pod Deletion
    note over K8s,MPPod: === Cleanup Phase (Pod Deletion) ===
    Dev->>K8s: Delete Pod
    K8s->>Kubelet: Terminate Pod
    Kubelet->>App: Stop container
    Kubelet->>CSI: NodeUnpublishVolume RPC
    CSI->>CSI: Remove bind mount from target
    CSI-->>Kubelet: Volume unmounted
    note over Reconciler,MPPod: Mountpoint Pod cleanup (when no workloads remain)
    Reconciler->>Reconciler: Detect workload Pod terminated
    Reconciler->>K8s: Update CRD (remove workload attachment)
    Reconciler->>K8s: Delete Mountpoint Pod (if no remaining workloads)
    K8s->>MPPod: Terminate pod
    MPPod->>MPPod: Unmount FUSE filesystem
    Reconciler->>K8s: Delete CRD (if no remaining Mountpoint Pods)

    %% Cleanup Phase - Volume Deletion
    note over Dev,S3: === Cleanup Phase (Volume Deletion) ===
    note right of S3: Bucket deletion depends on PV reclaim policy
    Dev->>K8s: Delete PVC
    K8s->>K8s: Mark PVC for deletion
    K8s->>K8s: Check reclaim policy (Delete/Retain)
    alt Reclaim Policy: Delete
        Provisioner->>K8s: Watch for PVC deletion
        K8s-->>Provisioner: PVC deletion event
        Provisioner->>Controller: DeleteVolume RPC with Volume ID via Unix socket
        Controller->>S3: Delete S3 bucket using Volume ID as bucket name (only if empty)
        S3-->>Controller: Bucket deleted (if empty, otherwise retained for safety)
        Controller-->>Provisioner: DeleteVolume response (always success per CSI spec)
        Provisioner->>K8s: Delete PersistentVolume
    else Reclaim Policy: Retain
        K8s->>K8s: Release PV (status: Released)
        note right of S3: Bucket and data preserved
    end
```

</div>

## Phase Flow Summary

| Phase/Step | Description | Key Outcome |
|------------|-------------|-------------|
| **Phase 1: StorageClass Setup** | **Administrator defines provisioning parameters (one-time setup)** | **Template for dynamic volumes** |
| 1.1 | Create StorageClass with provisioner `s3.csi.scality.com`, parameters (mount options), and secret references | StorageClass stored |
| 1.2 | Define provisioner-secret and node-publish-secret names with template variables (${pvc.name}, ${pvc.namespace}) | Credential templates ready |
| **Phase 2: Volume Provisioning** | **Automatic S3 bucket creation on PVC request** | **S3 bucket and PV created** |
| 2.1 | Developer creates PVC referencing StorageClass, specifying capacity and access modes | PVC created (status: Pending) |
| 2.2 | CSI Provisioner Sidecar watches API Server, detects new PVC, resolves templates | Provisioning triggered |
| 2.3 | Provisioner calls CreateVolume RPC to CSI Controller Service with resolved parameters | Controller invoked |
| 2.4 | CSI Controller generates unique Volume ID (csi-s3-{uuid}) and creates S3 bucket with this ID as bucket name | S3 bucket created with consistent naming |
| 2.5a | Provisioner creates PV object. If volumeBindingMode: Immediate, PVC binds immediately | PV created, binding depends on mode |
| 2.5b | If volumeBindingMode: WaitForFirstConsumer, PVC remains Pending until pod scheduled | PV available, PVC waits for consumer |
| | | |
| **Phase 3: Pod Scheduling** | **Kubernetes finds where to run the pod** | **Pod assigned to node** |
| 3.1 | Create Pod with volumeMounts section referencing PVC and specifying container mount path | Pod object created |
| 3.2 | Scheduler evaluates nodes based on resources, topology constraints, and CSI driver availability | Pod scheduled to node |
| 3.3 | If volumeBindingMode: WaitForFirstConsumer, PVC and PV bind now that pod is scheduled | PVC finally bound (all modes) |
| | | |
| **Phase 4: Mountpoint Pod Provisioning** | **Pod Reconciler creates Mountpoint Pod and CRD** | **Mountpoint Pod ready** |
| 4.1 | Pod Reconciler (watching all Pods) detects workload Pod scheduled and needs S3 volume | Reconciliation triggered |
| 4.2 | Pod Reconciler checks for existing MountpointS3PodAttachment CRD with matching config | Reuse or create decision |
| 4.3 | Pod Reconciler creates Mountpoint Pod on the same node as workload | Mountpoint Pod scheduled |
| 4.4 | Pod Reconciler creates MountpointS3PodAttachment CRD with workload and Mountpoint Pod assignment | CRD created with assignment |
| | | |
| **Phase 5: Volume Mount (CSI)** | **Node Service coordinates mount operation** | **S3 mounted locally** |
| 5.1 | Kubelet triggers NodePublishVolume RPC when pod starts on the scheduled node | CSI mount request initiated |
| 5.2 | CSI Node Service waits for MountpointS3PodAttachment CRD to contain its Mountpoint Pod assignment | Assignment found |
| 5.3 | CSI Node Service sends mount options to Mountpoint Pod via Unix socket | Mountpoint Pod receives S3 config |
| 5.4 | Mountpoint Pod authenticates to S3 endpoint, creates FUSE mount at source directory | Source mount ready |
| 5.5 | CSI Node Service creates bind mount from source to container target path | Volume NOW accessible via bind mount |
| | | |
| **Phase 6: Application Access** | **Pod performs file operations on dynamically provisioned storage** | **S3 data accessible** |
| 6.1 | Container starts with volume accessible at specified mountPath | Container has S3 access |
| 6.2 | Application performs file operations, mount-s3 translates to S3 API calls | Data read/write operations |
| | | |
| **Phase 7: Cleanup (Pod)** | **Unmount volume when pod terminates** | **Mount cleaned up** |
| 7.1 | Pod deletion initiated, kubelet stops container gracefully | Container terminated |
| 7.2 | Kubelet calls NodeUnpublishVolume RPC, CSI driver removes bind mount from target path | Bind mount removed |
| 7.3 | Pod Reconciler detects workload Pod terminated, removes attachment from CRD | CRD updated |
| 7.4 | If no remaining workloads, Pod Reconciler deletes Mountpoint Pod and CRD | Mountpoint Pod terminated, FUSE unmounted |
| | | |
| **Phase 8: Cleanup (Volume)** | **Handle bucket lifecycle based on reclaim policy** | **Storage fate determined** |
| 8.1 | Developer deletes PVC, triggering cleanup based on StorageClass reclaim policy | PVC marked for deletion |
| 8.2a | If Delete: Provisioner calls DeleteVolume RPC with Volume ID, Controller attempts to delete S3 bucket (only if empty) | Storage removed if empty, otherwise retained |
| 8.2b | If Retain: PV released but bucket preserved, admin must manually clean up | Data preserved for recovery |

## Volume ID System Architecture

The CSI driver implements a unified identification system that ensures perfect consistency between Kubernetes resources and S3 storage:

### Volume ID Generation and Usage

```mermaid
graph LR
    A[CreateVolume RPC] --> B[generateVolumeID]
    B --> C["Volume ID: csi-s3-{uuid}"]
    C --> D[S3 Bucket Name]
    C --> E[CSI Volume ID]
    C --> F[PV Volume Context]

    G[DeleteVolume RPC] --> H[req.GetVolumeId]
    H --> I["Same Volume ID: csi-s3-{uuid}"]
    I --> J[Delete S3 Bucket by Name]
```

### Key Characteristics

- **Unique Generation**: Each volume gets a UUID-based identifier: `csi-s3-12345678-abcd-1234-abcd-123456789012`
- **Dual Purpose**: The same ID serves as both the CSI Volume ID (stored in PersistentVolume) and the S3 bucket name
- **Lifecycle Consistency**: Creation and deletion operations use identical identifiers, eliminating bucket/volume mapping ambiguity
- **Resource Mapping**:
  - Kubernetes PV Name: `pvc-{pvc-uuid}` (generated by external-provisioner)
  - CSI Volume ID: `csi-s3-{driver-uuid}` (generated by CSI driver)
  - S3 Bucket Name: `csi-s3-{driver-uuid}` (same as Volume ID)

This architecture ensures that:

1. There's never confusion about which S3 bucket corresponds to which Kubernetes volume
2. Deletion operations are reliable and precise
3. Manual debugging is simplified (Volume ID directly maps to bucket name)

## Volume Sharing

When multiple workload pods on the same node use the same PersistentVolume with matching mount options and fsGroup:

- Pod Reconciler adds the new workload to the existing MountpointS3PodAttachment CRD
- The same Mountpoint Pod serves all workloads
- Each workload gets its own bind mount to the shared source mount
- Resource efficiency is optimized by avoiding duplicate FUSE processes
- When any workload terminates, only its bind mount is removed
- The Mountpoint Pod remains until all workloads using it terminate

## Key Differences from Static Provisioning

| Aspect | Dynamic Provisioning | Static Provisioning |
|--------|---------------------|-------------------|
| **Bucket Creation** | Automatic via CSI Controller | Manual by administrator |
| **PV Creation** | Automatic by CSI Provisioner Sidecar | Manual by administrator |
| **Credential Flow** | Template-based, resolved at provision time | Fixed at PV creation |
| **Bucket Deletion** | Automatic if reclaim policy is Delete | Never deleted by CSI |
| **Multi-tenancy** | Supported via credential templates | Limited to per-PV secrets |
| **Complexity** | More moving parts, but automated | Simpler architecture, manual setup |

## Credential Resolution Details

For detailed information about how credentials are resolved during dynamic provisioning, including:

- Provisioner secret resolution for bucket operations
- Node-publish secret resolution for mount operations
- Template variable substitution
- Credential precedence and fallback

See [Dynamic Provisioning Credentials Management](./ring-s3-credentials-management/dynamic-provisioning-credentials-management.md)
