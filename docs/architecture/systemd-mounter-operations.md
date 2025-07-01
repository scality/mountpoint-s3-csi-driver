# Systemd Mounter Operations Guide

## Overview

This guide explains the operational behavior of the Systemd Mounter from an end-user perspective, covering pod lifecycle, mount persistence, and recovery scenarios. Understanding these operations helps troubleshoot issues and predict system behavior during various restart scenarios.

## How Pods Access Host Systemd Mounter

### Pod-to-Host Communication Flow

```mermaid
graph TB
    subgraph "Pod Container"
        A[Application Pod]
        B[Volume Mount Request]
    end
    
    subgraph "Node (Host System)"
        C[kubelet]
        D[CSI Node Plugin Container]
        E[SystemdMounter]
        F[Host systemd]
        G[mount-s3 Service]
    end
    
    subgraph "Mount Target"
        H[Target Directory]
        I[FUSE Mount Point]
    end
    
    A --> B
    B --> C
    C --> D
    D --> E
    E --> F
    F --> G
    G --> I
    I --> H
    
    style A fill:#e3f2fd
    style F fill:#fff3e0
    style I fill:#e8f5e8
```

### Key Communication Mechanisms

1. **CSI Interface**: Pod requests volumes through Kubernetes CSI (Container Storage Interface)
2. **Host Privileges**: CSI Node Plugin runs as privileged container with host access
3. **systemd Integration**: Plugin communicates with host systemd via D-Bus socket
4. **Mount Propagation**: Host mounts are visible in container through mount propagation

### Access Path Details

| Component | Access Method | Privileges Required |
|-----------|---------------|-------------------|
| **Application Pod** | Standard Kubernetes volume mount | User-level |
| **CSI Node Plugin** | Privileged container with host namespace access | Privileged, host PID/IPC |
| **Host systemd** | D-Bus socket at `/run/systemd/private` | Root access via privilege escalation |
| **mount-s3 Binary** | Direct execution on host filesystem | Root privileges through systemd |

## Pod Startup Workflow

### Initial Pod Creation

```mermaid
sequenceDiagram
    participant U as User/Kubectl
    participant K as Kubernetes API
    participant S as Scheduler
    participant KL as kubelet
    participant CSI as CSI Node Plugin
    participant SM as SystemdMounter
    participant SD as systemd
    
    Note over U, SD: Pod Creation & Volume Mounting
    
    U->>K: Create Pod with PVC
    K->>S: Schedule Pod to Node
    S->>KL: Pod assigned to node
    
    Note over KL: Volume attachment phase
    KL->>CSI: NodePublishVolume()
    CSI->>SM: Mount(bucket, target, creds, args)
    
    Note over SM: Pre-mount validation
    SM->>SM: Check if target already mounted
    
    alt Target not mounted
        SM->>SD: Create mount-s3 service
        SD->>SD: Start mount-s3 FUSE daemon
        SD-->>SM: Service active
        SM-->>CSI: Mount successful
    else Target already mounted
        Note over SM: Skip mount, return success
        SM-->>CSI: Already mounted
    end
    
    CSI-->>KL: Volume ready
    KL->>KL: Start Pod containers
    
    Note over KL: Pod is now Running
```

### Mount Point Detection Logic

The SystemdMounter performs these checks before creating a new mount:

```mermaid
flowchart TD
    A[Mount Request] --> B[Check target path exists]
    B --> C{Path exists?}
    
    C -->|No| D[Create target directory]
    C -->|Yes| E[Check if mount point]
    
    D --> E
    E --> F{Is mount point?}
    
    F -->|No| G[Proceed with new mount]
    F -->|Yes| H[Parse /proc/mounts]
    
    H --> I{Device = "mountpoint-s3"?}
    I -->|Yes| J[Skip mount - already active]
    I -->|No| K[Not our mount - proceed]
    
    G --> L[Create systemd service]
    K --> L
    J --> M[Return success]
    L --> N[Start mount-s3 daemon]
    N --> M
    
    style J fill:#e8f5e8
    style M fill:#e8f5e8
    style L fill:#fff3e0
```

## CSI Driver Restart Scenarios

### CSI Driver Restart with `requiresRepublish: true`

The CSI driver is configured with `requiresRepublish: true`, which means kubelet will call `NodePublishVolume` again for all existing volumes when the CSI driver restarts.

```mermaid
sequenceDiagram
    participant KL as kubelet
    participant CSI as CSI Node Plugin
    participant SM as SystemdMounter
    participant SD as systemd
    participant MP as Existing mount-s3 process
    
    Note over CSI: CSI Driver Restarts
    CSI->>CSI: Process restart/crash recovery
    
    Note over KL: kubelet detects CSI restart
    KL->>CSI: NodePublishVolume() for existing volumes
    
    loop For each existing volume
        CSI->>SM: Mount(bucket, target, creds, args)
        SM->>SM: Check if target is mount point
        
        alt Mount still active
            SM->>SM: Parse /proc/mounts
            SM->>SM: Verify device="mountpoint-s3"
            Note over SM: Mount is healthy, skip creation
            SM-->>CSI: Already mounted
        else Mount is stale/corrupted
            SM->>SD: Create new mount-s3 service
            SD->>SD: Start new mount process
            Note over MP: Old process may still be running
            SD-->>SM: New service active
            SM-->>CSI: Re-mount successful
        end
    end
    
    CSI-->>KL: All volumes republished
    
    Note over KL, MP: Pods continue running with persistent mounts
```

### Mount Persistence During CSI Restart

| Scenario | Behavior | Outcome |
|----------|----------|---------|
| **Active systemd mount** | Mount persists independently of CSI | ✅ No disruption |
| **Corrupted mount** | CSI detects and recreates mount | ✅ Automatic recovery |
| **Orphaned systemd service** | New service created, old may remain | ⚠️ Potential resource leak |
| **Credential refresh** | Credentials updated during republish | ✅ Seamless credential rotation |

## Pod Restart Scenarios

### Pod Restart with Persistent Volume

```mermaid
sequenceDiagram
    participant P1 as Pod-1 (Original)
    participant KL as kubelet
    participant CSI as CSI Node Plugin
    participant SM as SystemdMounter
    participant SD as systemd
    participant MS as mount-s3 Service
    participant P2 as Pod-2 (Replacement)
    
    Note over P1, MS: Pod is running with mounted volume
    
    P1->>P1: Pod terminates/crashes
    KL->>CSI: NodeUnpublishVolume()
    CSI->>SM: Unmount(target)
    SM->>SD: Stop mount-s3 service
    SD->>MS: Terminate FUSE daemon
    MS->>MS: Clean unmount, data persisted in S3
    
    Note over KL: Create replacement pod
    KL->>CSI: NodePublishVolume() for same PVC
    CSI->>SM: Mount(same bucket, same target)
    SM->>SD: Create new mount-s3 service
    SD->>MS: Start new FUSE daemon
    MS->>MS: Mount S3 bucket with persisted data
    
    Note over P2: Pod starts with existing data accessible
```

### Data Persistence Guarantees

```mermaid
graph LR
    subgraph "Pod Lifecycle"
        A[Pod-1 Writes Data]
        B[Pod-1 Terminates]
        C[Pod-2 Starts]
        D[Pod-2 Reads Data]
    end
    
    subgraph "Mount Lifecycle"
        E[Mount Created]
        F[FUSE Operations]
        G[Clean Unmount]
        H[New Mount]
    end
    
    subgraph "S3 Storage"
        I[Data Written to S3]
        J[Data Persisted]
        K[Data Available]
    end
    
    A --> E
    E --> F
    F --> I
    B --> G
    G --> J
    C --> H
    H --> K
    K --> D
    
    style I fill:#e8f5e8
    style J fill:#e8f5e8
    style K fill:#e8f5e8
```

## System Recovery Scenarios

### Node Restart Recovery

```mermaid
flowchart TD
    A[Node Restart] --> B[systemd services stopped]
    B --> C[kubelet starts]
    C --> D[CSI driver starts]
    D --> E[Detect existing PVCs]
    E --> F[Call NodePublishVolume for each PVC]
    
    F --> G{Target directory exists?}
    G -->|Yes| H[Check if mounted]
    G -->|No| I[Create target directory]
    
    H --> J{Valid mount?}
    J -->|No| K[Create new mount]
    J -->|Yes| L[Skip - already mounted]
    
    I --> K
    K --> M[Pod can access volume]
    L --> M
    
    style A fill:#ffebee
    style M fill:#e8f5e8
```

### Orphaned Service Cleanup

When CSI restarts, orphaned systemd services may remain:

```mermaid
graph TD
    A[CSI Restart] --> B[Check existing mounts]
    B --> C{Mount at target?}
    
    C -->|Yes| D[Verify mount health]
    C -->|No| E[Create new service]
    
    D --> F{Healthy mount?}
    F -->|Yes| G[Reuse existing mount]
    F -->|No| H[Create new service]
    
    E --> I[New mount-s3 service]
    H --> I
    G --> J[Mount ready]
    I --> K[Old service may remain]
    I --> J
    
    style K fill:#fff3e0
    style J fill:#e8f5e8
```

### Service Naming and Isolation

Each mount creates a unique systemd service:

```bash
# Service naming pattern
mount-s3-{VERSION}-{UUID}.service

# Example services
mount-s3-v1.12.0-a1b2c3d4-e5f6-7890-abcd-ef1234567890.service
mount-s3-v1.12.0-b2c3d4e5-f6g7-8901-bcde-f21234567890.service
```

## Operational Monitoring

### Health Check Commands

```bash
# Check systemd mount services
systemctl list-units 'mount-s3-*'

# Verify mount points
mount | grep mountpoint-s3

# Check CSI driver logs
kubectl logs -n kube-system -l app=scality-s3-csi-node

# Monitor mount activity
journalctl -f -u 'mount-s3-*'
```

### Common Operational Patterns

| Operation | Expected Behavior | Verification |
|-----------|------------------|--------------|
| **Pod Start** | New systemd service created | `systemctl list-units mount-s3-*` |
| **Pod Restart** | Old service stopped, new service created | Check service timestamps |
| **CSI Restart** | Existing mounts detected and reused | No new services for existing mounts |
| **Node Restart** | All mounts recreated during startup | All PVCs remounted successfully |

## Troubleshooting Mount Persistence

### Mount State Verification

```mermaid
flowchart TD
    A[Pod Restart Issue] --> B[Check target path]
    B --> C{Directory exists?}
    
    C -->|No| D[Target path cleanup issue]
    C -->|Yes| E[Check mount status]
    
    E --> F{Mount active?}
    F -->|No| G[Mount creation failed]
    F -->|Yes| H[Check service status]
    
    H --> I{Service running?}
    I -->|No| J[Service failed/stopped]
    I -->|Yes| K[Check pod access]
    
    G --> L[Review CSI logs]
    J --> M[Review systemd logs]
    K --> N[Check credentials/permissions]
    
    style D fill:#ffebee
    style G fill:#ffebee
    style J fill:#ffebee
```

### Recovery Actions

| Issue | Diagnosis | Resolution |
|-------|-----------|------------|
| **Stale mount** | Mount point exists but service stopped | Manual unmount: `umount <target>` |
| **Orphaned service** | Service running but no mount point | Stop service: `systemctl stop mount-s3-*` |
| **Permission denied** | Credentials expired/invalid | Update credentials, restart pod |
| **Mount collision** | Multiple services for same target | Clean up duplicate services |

## Best Practices

### For Cluster Administrators

1. **Monitor systemd services** regularly for orphaned mount services
2. **Set up alerts** for failed mount-s3 services
3. **Implement log retention** for systemd journal entries
4. **Plan for credential rotation** with zero-downtime updates

### For Application Developers

1. **Design for mount availability** - handle temporary mount unavailability
2. **Implement proper error handling** for I/O operations
3. **Use readiness probes** to verify volume accessibility
4. **Plan for data consistency** during pod restarts

### For Operations Teams

1. **Monitor CSI driver health** and restart behavior
2. **Track mount service lifecycle** and cleanup
3. **Implement automated recovery** for common failure scenarios
4. **Maintain runbook procedures** for manual recovery

## Summary

The Systemd Mounter provides robust mount persistence through:

- **Independent mount lifecycle** from CSI driver process
- **Automatic mount detection** and reuse during restarts
- **Graceful handling** of pod and CSI driver restarts
- **Persistent data access** across application lifecycle changes

Understanding these operational patterns helps ensure reliable S3 storage integration in production Kubernetes environments.