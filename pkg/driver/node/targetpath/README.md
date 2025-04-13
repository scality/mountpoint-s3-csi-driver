# Target Path Package

This package handles parsing and validation of CSI target paths.

## Overview

The target path package provides utilities for extracting information from CSI target paths. In Kubernetes, CSI target paths follow a specific format that includes information about the pod and volume.

## Key Components

### TargetPath Structure

The main structure is `TargetPath`, which contains:
- `PodID` - The ID of the pod to which the volume is mounted
- `VolumeID` - The ID of the volume being mounted

### Functions

The package provides these key functions:

- `Parse(target string) (TargetPath, error)` - Parses a target path string into a `TargetPath` structure
- `TargetPath.String()` - Returns a string representation of the target path

## Path Format

Target paths in Kubernetes CSI follow this format:
```
/var/lib/kubelet/pods/<pod-uid>/volumes/kubernetes.io~csi/<volume-id>/mount
```

Where:
- `/var/lib/kubelet/` - The base kubelet path (configurable)
- `<pod-uid>` - The UID of the pod
- `kubernetes.io~csi` - CSI plugin identifier
- `<volume-id>` - The ID of the CSI volume
- `mount` - The mount directory

## Error Handling

The package defines:
- `ErrInvalidTargetPath` - Returned when a path doesn't match the expected format

## Usage Example

```go
targetPath, err := targetpath.Parse("/var/lib/kubelet/pods/abc123/volumes/kubernetes.io~csi/vol-xyz/mount")
if err != nil {
    // Handle error
}
fmt.Printf("Pod ID: %s, Volume ID: %s\n", targetPath.PodID, targetPath.VolumeID)
```

## See Also

- [Node Service](../README.md) - The main node service that uses target path parsing
