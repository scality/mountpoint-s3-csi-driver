# Pod Security and FSGroup

This page explains how Kubernetes pod security contexts — specifically `fsGroup` — interact with the Scality CSI Driver for S3 and its pod mounter architecture.

## Background: Kubernetes fsGroup

When a pod's `securityContext` specifies an `fsGroup`, Kubernetes changes the group ownership of all volumes mounted into that pod to match the specified GID.
This includes CSI volumes, emptyDir volumes, and others.

```yaml
spec:
  securityContext:
    runAsUser: 1001
    runAsGroup: 1001
    fsGroup: 1001       # Kubernetes sets GID on all mounted volumes
  containers:
    - name: app
      # ...
```

This is commonly used to ensure that non-root processes can read and write to shared volumes.

## How the CSI Driver Handles fsGroup

The Scality CSI Driver for S3 uses a [pod mounter architecture](../architecture/pod-mounter-architecture.md) where dedicated **Mountpoint Pods** run the mount-s3 (FUSE) process
on behalf of workload pods. The communication between the CSI node driver and the Mountpoint Pod happens over a Unix socket inside a shared emptyDir volume.

```text
[Workload Pod]                    [Mountpoint Pod]
  securityContext:                  securityContext:
    fsGroup: <any>                    fsGroup: 1000  (set by CSI driver)
        |                                 |
        |                            /comm/mount.sock
        |                                 |
   NodePublishVolume ──────────> mount-s3 (FUSE process)
        |                                 |
   bind mount  <───────────────  S3 FUSE mount
```

The driver automatically configures the Mountpoint Pod's security context:

| Field | Value | Description |
|-------|-------|-------------|
| `PodSecurityContext.FSGroup` | `1000` | Ensures the communication emptyDir has correct group ownership |
| `SecurityContext.RunAsUser` | `1000` | mount-s3 runs as non-root user |
| `SecurityContext.RunAsNonRoot` | `true` | Enforces non-root execution |

This is handled internally by the CSI driver — no user configuration is needed.

## Using fsGroup in Workload Pods

Workload pods **can freely use `fsGroup`** in their security context. The workload pod's `fsGroup` does not affect the Mountpoint Pod's internal communication.
The CSI driver manages the Mountpoint Pod's security context independently.

### Static Provisioning Example

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv
spec:
  capacity:
    storage: 1200Gi
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  claimRef:
    namespace: default
    name: s3-pvc
  mountOptions:
    - uid=1001
    - gid=1001
    - allow-other
  csi:
    driver: s3.csi.scality.com
    volumeHandle: s3-csi-fsgroup-example
    volumeAttributes:
      bucketName: my-bucket
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-pvc
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: ""
  resources:
    requests:
      storage: 1200Gi
  volumeName: s3-pv
---
apiVersion: v1
kind: Pod
metadata:
  name: s3-app
spec:
  securityContext:
    runAsUser: 1001
    runAsGroup: 1001
    fsGroup: 1001
  containers:
    - name: app
      image: ubuntu
      command: ["/bin/sh", "-c", "echo hello > /data/test.txt; tail -f /dev/null"]
      volumeMounts:
        - name: s3-volume
          mountPath: /data
  volumes:
    - name: s3-volume
      persistentVolumeClaim:
        claimName: s3-pvc
```

### Dynamic Provisioning Example

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-sc
provisioner: s3.csi.scality.com
reclaimPolicy: Delete
mountOptions:
  - uid=1001
  - gid=1001
  - allow-other
parameters:
  csi.storage.k8s.io/provisioner-secret-name: s3-provisioner-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: s3-node-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-pvc
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-sc
  resources:
    requests:
      storage: 1200Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: s3-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: s3-app
  template:
    metadata:
      labels:
        app: s3-app
    spec:
      securityContext:
        runAsUser: 1001
        runAsGroup: 1001
        fsGroup: 1001
      containers:
        - name: app
          image: ubuntu
          command: ["/bin/sh", "-c", "echo hello > /data/test.txt; tail -f /dev/null"]
          volumeMounts:
            - name: s3-volume
              mountPath: /data
      volumes:
        - name: s3-volume
          persistentVolumeClaim:
            claimName: s3-pvc
```

## Relationship Between fsGroup and Mount Options

The `fsGroup` in a pod's security context and the `uid`/`gid` mount options serve different purposes:

| Setting | Scope | Purpose |
|---------|-------|---------|
| `uid=<ID>` (mount option) | S3 FUSE mount | Sets the UID that mount-s3 reports for all files and directories in the mounted S3 bucket |
| `gid=<ID>` (mount option) | S3 FUSE mount | Sets the GID that mount-s3 reports for all files and directories in the mounted S3 bucket |
| `fsGroup` (pod security context) | Kubernetes volumes | Kubernetes sets the group ownership of volume mounts inside the pod to this GID |
| `runAsUser` (pod security context) | Container process | The UID the container process runs as |
| `runAsGroup` (pod security context) | Container process | The primary GID of the container process |

For S3 volumes, the `uid` and `gid` mount options control what ownership mount-s3 presents for S3 objects.
The `allow-other` mount option is required so that processes running as a different UID/GID (the workload pod's `runAsUser`/`runAsGroup`) can access the mount.

!!! note "fsGroup takes precedence over --gid"
    When a workload pod specifies `fsGroup` in its security context, the CSI driver
    **overrides** any `--gid` value set in PV or StorageClass mount options.
    The pod-level security intent (`fsGroup`) always wins. The driver also
    automatically sets `--allow-other`, `--dir-mode=770`, and `--file-mode=660` if
    not already present. The `--uid` mount option is not affected.

A common configuration pattern is to align these values:

```yaml
# Mount options on PV or StorageClass
mountOptions:
  - uid=1001
  - gid=1001
  - allow-other

# Pod security context
securityContext:
  runAsUser: 1001
  runAsGroup: 1001
  fsGroup: 1001        # Safe to use — does not affect mounter pod
```

## Volume Sharing and fsGroup

When multiple workload pods share the same S3 volume, the CSI driver considers `fsGroup` as part of the matching criteria for sharing a Mountpoint Pod.
Pods with different `fsGroup` values will get separate Mountpoint Pods, even if they reference the same PersistentVolume.

For details on volume sharing behavior, see [Pod Mounter Architecture - Volume Sharing](../architecture/pod-mounter-architecture.md#volume-sharing).
