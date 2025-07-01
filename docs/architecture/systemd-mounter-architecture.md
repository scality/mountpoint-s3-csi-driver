# Systemd Mounter Architecture

## Overview

The Systemd Mounter is one of two mounting strategies available in the Scality S3 CSI Driver (the other being Pod Mounter). It provides a lightweight, systemd-based approach to mounting S3 buckets as FUSE filesystems using the `mount-s3` binary from AWS Mountpoint for Amazon S3.

The Systemd Mounter leverages the host system's systemd daemon to manage mount-s3 processes as transient systemd units. This approach provides several advantages:

- **Process Management**: Systemd handles process lifecycle, restart policies, and cleanup
- **Resource Isolation**: Each mount runs as a separate systemd unit with configurable resource limits
- **Logging Integration**: Output is captured through pseudoterminals and integrated with systemd logging
- **Service Dependencies**: Proper ordering and dependency management through systemd

## Architecture Components

```mermaid
graph TB
    subgraph "CSI Node Plugin"
        A[CSI NodeServer] --> B[SystemdMounter]
        B --> C[SystemdSupervisor]
        B --> D[CredentialProvider]
        B --> E[Mount Interface]
    end
    
    subgraph "Systemd Integration"
        C --> F[SystemdOsConnection]
        F --> G[D-Bus Interface]
        C --> H[Pts Manager]
    end
    
    subgraph "Host System"
        G --> I[systemd Daemon]
        I --> J[mount-s3 Service Units]
        H --> K[Pseudoterminals]
        J --> L[S3 FUSE Mount]
    end
    
    subgraph "External Dependencies"
        D --> M[AWS Credentials]
        L --> N[S3 Bucket]
    end
    
    style A fill:#e1f5fe
    style B fill:#f3e5f5
    style I fill:#fff3e0
    style L fill:#e8f5e8
```

### Core Components

| Component | Purpose | Location |
|-----------|---------|----------|
| **SystemdMounter** | Main interface implementing the Mounter contract | `pkg/driver/node/mounter/systemd_mounter.go` |
| **SystemdSupervisor** | Manages systemd service lifecycle and monitoring | `pkg/system/systemd.go` |
| **SystemdOsConnection** | Low-level D-Bus communication with systemd | `pkg/system/systemd.go` |
| **Pts** | Pseudoterminal management for output capture | `pkg/system/pts.go` |
| **ServiceRunner** | Interface for running systemd services | `pkg/driver/node/mounter/mounter.go` |

## Static Provisioning Workflow

The Systemd Mounter only supports static provisioning, where S3 buckets are pre-created and manually configured as PersistentVolumes.

```mermaid
sequenceDiagram
    participant K as Kubelet
    participant CSI as CSI NodeServer
    participant SM as SystemdMounter
    participant SS as SystemdSupervisor
    participant SD as systemd Daemon
    participant MS as mount-s3 Service
    participant S3 as S3 Bucket
    
    Note over K, S3: Static Provisioning Mount Request
    
    K->>CSI: NodePublishVolume()
    CSI->>SM: Mount(bucket, target, credentials, args)
    
    Note over SM: Validate parameters & check existing mount
    SM->>SM: verifyMountPointStatx(target)
    SM->>SM: IsMountPoint(target)
    
    Note over SM: Setup credentials & environment
    SM->>SM: credProvider.Provide()
    SM->>SM: enforceCSIDriverMountArgPolicy()
    
    Note over SM: Create systemd service configuration
    SM->>SS: StartService(ExecConfig)
    SS->>SS: Create pseudoterminal
    SS->>SS: Build D-Bus properties
    
    Note over SS, SD: D-Bus Communication
    SS->>SD: StartTransientUnit(service_name, properties)
    SD->>MS: Create & start mount-s3 service
    
    Note over MS, S3: FUSE Mount Process
    MS->>S3: Connect & authenticate
    MS->>MS: Mount FUSE filesystem
    
    Note over SS: Monitor service state
    SD-->>SS: Service state updates (D-Bus signals)
    SS->>SS: Check ActiveState == "active"
    
    SS-->>SM: Service started successfully
    SM-->>CSI: Mount completed
    CSI-->>K: Success
```

## Mount Process Deep Dive

### 1. Pre-Mount Validation

```mermaid
flowchart TD
    A[Mount Request] --> B{Bucket name empty?}
    B -->|Yes| C[Return Error]
    B -->|No| D{Target path empty?}
    D -->|Yes| C
    D -->|No| E[Create timeout context]
    
    E --> F[Set credential paths]
    F --> G[Check target path status]
    
    G --> H{Path exists?}
    H -->|No| I[Create directory]
    H -->|Yes| J{Corrupted mount?}
    
    J -->|Yes| K[Attempt unmount]
    J -->|No| L[Check if already mounted]
    
    I --> L
    K --> L
    L --> M{Already mounted?}
    M -->|Yes| N[Return success]
    M -->|No| O[Proceed with mount]
    
    style A fill:#e3f2fd
    style C fill:#ffebee
    style N fill:#e8f5e8
    style O fill:#fff3e0
```

### 2. Credential Management

The SystemdMounter handles AWS credentials through multiple sources:

- **Environment Variables**: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, etc.
- **Instance Profile**: EC2 instance roles
- **Shared Credentials File**: AWS credentials file
- **Kubernetes Secrets**: Secret-based authentication

```mermaid
graph LR
    subgraph "Credential Sources"
        A[Environment Variables]
        B[Instance Profile]
        C[Shared Credentials]
        D[Kubernetes Secrets]
    end
    
    subgraph "Processing"
        E[CredentialProvider]
        F[Environment Merger]
    end
    
    subgraph "Output"
        G[Service Environment]
        H[Credential Files]
    end
    
    A --> E
    B --> E
    C --> E
    D --> E
    
    E --> F
    F --> G
    F --> H
    
    style E fill:#f3e5f5
    style G fill:#e8f5e8
    style H fill:#e8f5e8
```

### 3. Systemd Service Creation

```mermaid
graph TD
    A[ExecConfig] --> B[Create Pseudoterminal]
    B --> C[Build D-Bus Properties]
    
    C --> D[Service Properties]
    D --> E[Description: FUSE daemon]
    D --> F[Type: forking]
    D --> G[ExecPath: /usr/bin/mount-s3]
    D --> H[Args: mount options + bucket + target]
    D --> I[Environment: AWS credentials]
    D --> J[StandardOutput: tty]
    D --> K[TTYPath: /dev/pts/N]
    
    L[Service Name] --> M[mount-s3-VERSION-UUID.service]
    
    style A fill:#e3f2fd
    style D fill:#f3e5f5
    style M fill:#fff3e0
```

## Systemd Integration Details

### D-Bus Communication

The SystemdMounter communicates with systemd through D-Bus, specifically using the systemd Manager interface:

```mermaid
sequenceDiagram
    participant SM as SystemdMounter
    participant SC as SystemdOsConnection
    participant DB as D-Bus
    participant SD as systemd Manager
    
    Note over SM, SD: Service Lifecycle Management
    
    SM->>SC: StartTransientUnit()
    SC->>DB: Connect to unix:/run/systemd/private
    DB->>SD: org.freedesktop.systemd1.Manager.StartTransientUnit
    
    Note over SD: Create and start service
    SD-->>DB: UnitNew signal
    DB-->>SC: Unit created notification
    
    Note over SD: Service state changes
    SD-->>DB: PropertiesChanged signals
    DB-->>SC: ActiveState updates
    SC-->>SM: Service monitoring
    
    Note over SM: Mount completion
    SM->>SC: Service started successfully
```

### Signal Monitoring

```mermaid
graph TD
    A[D-Bus Signals] --> B{Signal Type}
    
    B -->|UnitNew| C[Register Service Watcher]
    B -->|UnitRemoved| D[Clean Up Watchers]
    B -->|PropertiesChanged| E[Update Service State]
    
    E --> F{Property Type}
    F -->|ActiveState| G[Check if 'active']
    F -->|ExecMainCode| H[Check exit code]
    F -->|ExecMainStatus| I[Check exit status]
    
    G --> J[Notify Mount Success]
    H --> K{Code == 0?}
    I --> L{Status != 0?}
    
    K -->|No| M[Continue Monitoring]
    K -->|Yes| M
    L -->|Yes| N[Report Error]
    L -->|No| O[Report Success]
    
    style A fill:#e3f2fd
    style J fill:#e8f5e8
    style N fill:#ffebee
    style O fill:#e8f5e8
```

## Error Handling and Cleanup

### Mount Failure Recovery

```mermaid
flowchart TD
    A[Mount Failure] --> B[Read service output]
    B --> C[Stop systemd service]
    C --> D[Clean up target directory]
    D --> E[Clean up credentials]
    E --> F[Return error with output]
    
    style A fill:#ffebee
    style F fill:#ffebee
```

### Unmount Process

```mermaid
sequenceDiagram
    participant CSI as CSI NodeServer
    participant SM as SystemdMounter
    participant SS as SystemdSupervisor
    participant SD as systemd Daemon
    
    CSI->>SM: Unmount(target, credentials)
    SM->>SS: RunOneshot(umount config)
    SS->>SD: StartTransientUnit(oneshot service)
    
    Note over SD: Execute /usr/bin/umount target
    SD-->>SS: Service completion
    SS->>SS: Stop and remove service
    SS-->>SM: Unmount result
    
    SM->>SM: Clean up credentials
    SM-->>CSI: Unmount completed
```

## Configuration and Environment

### Mount Arguments Policy

The SystemdMounter enforces security policies by filtering certain mount arguments:

```mermaid
graph TD
    A[Raw Mount Args] --> B[Policy Filter]
    
    B --> C{Allowed Arguments}
    C -->|Keep| D[--read-only]
    C -->|Keep| E[--allow-root]
    C -->|Keep| F[--cache]
    C -->|Keep| G[--user-agent-prefix]
    
    B --> H{Blocked Arguments}
    H -->|Remove| I[--endpoint-url]
    H -->|Remove| J[--profile]
    H -->|Remove| K[--cache-xz]
    H -->|Remove| L[--incremental-upload]
    
    D --> M[Filtered Args]
    E --> M
    F --> M
    G --> M
    
    style A fill:#e3f2fd
    style M fill:#e8f5e8
    style I fill:#ffebee
    style J fill:#ffebee
    style K fill:#ffebee
    style L fill:#ffebee
```

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `MOUNT_S3_PATH` | Path to mount-s3 binary | `/usr/bin/mount-s3` |
| `HOST_PLUGIN_DIR` | Host plugin directory | `/var/lib/kubelet/plugins/s3.csi.scality.com/` |
| `PTMX_PATH` | Pseudoterminal master path | `/dev/ptmx` |
| `AWS_*` | AWS credential variables | Various |

## Deployment Considerations

### Prerequisites

- systemd available on host system
- D-Bus socket accessible at `/run/systemd/private`
- `mount-s3` binary installed on host
- Appropriate permissions for D-Bus communication

### Resource Management

Each mount creates a separate systemd service unit, allowing for:

- Individual process monitoring
- Resource limits per mount
- Independent lifecycle management
- Proper cleanup on node shutdown

## Enabling Mermaid Support

The project already has Mermaid support configured in `mkdocs.yml`:

```yaml
plugins:
  - mermaid2

markdown_extensions:
  - pymdownx.superfences:
      custom_fences:
        - name: mermaid
          class: mermaid
          format: pymdownx.superfences.fence_div_format
```

No additional configuration is required to view the diagrams in this documentation.

## Related Components

- **Pod Mounter**: Alternative mounting strategy using Kubernetes pods
- **CSI Node Server**: Main CSI interface implementation
- **Credential Provider**: AWS credential management
- **Mount-s3**: AWS Mountpoint for Amazon S3 binary

## Troubleshooting

Common issues and their resolution:

| Issue | Cause | Solution |
|-------|-------|----------|
| Mount fails with D-Bus error | systemd not accessible | Check D-Bus socket permissions |
| Service creation fails | Invalid mount arguments | Review mount options and policies |
| Credentials not found | Credential provider misconfigured | Verify AWS credential setup |
| Mount hangs | Network connectivity issues | Check S3 endpoint accessibility |

For detailed troubleshooting, see the [Troubleshooting Guide](../troubleshooting.md).