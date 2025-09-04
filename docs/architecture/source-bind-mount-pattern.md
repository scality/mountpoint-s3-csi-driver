# Source/Bind Mount Pattern

## Overview

The Scality CSI Driver v2 implements a source/bind mount pattern for efficient pod mount sharing. This architecture allows multiple containers to share the same S3 bucket mount while maintaining isolation and proper cleanup.

## Architecture

### Two-Step Mount Process

The pod mounter uses a two-step mounting process:

1. **Source Mount**: S3 bucket is mounted to a dedicated source directory
2. **Bind Mount**: Source directory is bind-mounted to the target container path

```
┌─────────────────────────────────────────────────────┐
│                    Mountpoint Pod                    │
│                                                      │
│  mount-s3 --bucket=mybucket /source/mp-<hash>       │
└──────────────────────┬──────────────────────────────┘
                       │
                       │ FUSE Mount
                       ▼
        ┌──────────────────────────────┐
        │      Source Directory        │
        │ /var/lib/kubelet/plugins/    │
        │ s3.csi.scality.com/mnt/      │
        │ mp-<hash>                    │
        └──────────┬───────────────────┘
                   │
      ┌────────────┼────────────┐
      │            │            │
  Bind Mount  Bind Mount   Bind Mount
      │            │            │
      ▼            ▼            ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│Container1│ │Container2│ │Container3│
│ /data    │ │ /mnt/s3  │ │ /bucket  │
└──────────┘ └──────────┘ └──────────┘
```

## Implementation Details

### Source Directory Structure

Source mounts are located at:
```
/var/lib/kubelet/plugins/s3.csi.scality.com/mnt/<mountpoint-pod-name>
```

Where `<mountpoint-pod-name>` is deterministically generated using:
```go
func MountpointPodNameFor(podUID string, volumeName string) string {
    return fmt.Sprintf("mp-%x", sha256.Sum224(fmt.Appendf(nil, "%s%s", podUID, volumeName)))
}
```

### Mount Function Flow

```go
func (pm *PodMounter) Mount(...) error {
    // 1. Determine source path
    source := filepath.Join(SourceMountDir(kubeletPath), mpPodName)
    
    // 2. Check if source is already mounted
    if !isSourceMounted {
        // Mount S3 bucket to source directory
        mountS3AtSource(source, bucketName, args)
    }
    
    // 3. Bind mount from source to target
    bindMountSyscall(source, target)
}
```

### Unmount Function Flow

```go
func (pm *PodMounter) Unmount(target string, ...) error {
    // Only unmount the bind mount
    unmountTarget(target)
    
    // Source remains mounted for other containers
    // PodUnmounter will clean up source when no longer needed
}
```

## Benefits

### 1. Efficient Resource Usage

- Single Mountpoint Pod serves multiple containers
- Reduced memory and CPU overhead
- Fewer S3 API calls and connections

### 2. Improved Performance

- Faster mount times for subsequent containers
- Shared cache benefits across containers
- Reduced startup latency

### 3. Simplified Management

- Clear separation between source and target mounts
- Easier debugging with centralized source mounts
- Predictable cleanup behavior

### 4. Better Isolation

- Each container gets its own bind mount
- Container-specific mount options can be applied
- Independent unmount operations

## Pod Sharing Criteria

Multiple workload pods can share a single Mountpoint Pod when they have:

- Same node location
- Same PersistentVolume
- Compatible mount options
- Same authentication source
- Same FSGroup (if specified)

## FSGroup Support

The implementation supports Kubernetes FSGroup for proper file permissions:

```go
// Extract FSGroup from VolumeCapability
fsGroup := capMount.GetVolumeMountGroup()

// Configure mount options for FSGroup
if fsGroup != "" {
    args.SetIfAbsent("--gid", fsGroup)
    args.SetIfAbsent("--allow-other", "")
    args.SetIfAbsent("--dir-mode", "0770")
    args.SetIfAbsent("--file-mode", "0660")
}
```

## Cleanup Process

### Bind Mount Cleanup

When a container is terminated:
1. CSI Driver receives `NodeUnpublishVolume` call
2. Only the bind mount to the container's target is removed
3. Source mount remains for other containers

### Source Mount Cleanup

The PodUnmounter component handles source cleanup:
1. Monitors for orphaned source mounts
2. Waits for all bind mount references to be removed
3. Unmounts the source when no longer in use
4. Removes the source directory

## Testing

Comprehensive tests verify the source/bind mount pattern:

- Source mount creation and reuse
- Multiple container sharing
- Proper unmount behavior
- FSGroup filtering
- Concurrent mount serialization

See `pkg/driver/node/mounter/pod_mount_sharing_test.go` for detailed test scenarios.

## Migration from v1

The source/bind mount pattern is transparent to users:
- No changes required to PersistentVolumes or PersistentVolumeClaims
- Existing workloads continue to function
- Performance improvements are automatic

## Troubleshooting

### Viewing Source Mounts

```bash
# List all source mounts
ls -la /var/lib/kubelet/plugins/s3.csi.scality.com/mnt/

# Check if a source is mounted
mount | grep s3.csi.scality.com/mnt

# Find bind mounts for a source
findmnt --source /var/lib/kubelet/plugins/s3.csi.scality.com/mnt/mp-<hash>
```

### Common Issues

1. **"Source directory not empty"**
   - Indicates improper cleanup
   - Check PodUnmounter logs
   - Manually unmount and remove if necessary

2. **"Bind mount failed"**
   - Verify source is properly mounted
   - Check target directory exists
   - Review filesystem permissions

3. **"Too many mounts"**
   - Check for mount leaks
   - Verify PodUnmounter is running
   - Review cleanup intervals