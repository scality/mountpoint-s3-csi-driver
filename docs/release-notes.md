# Release Notes

## [2.2.0](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/2.2.0)

March 31, 2026

### What's New

- **Custom CA Certificate Support for TLS**: Added support for injecting custom CA certificates
  into mounter pods, enabling TLS connections to S3 endpoints with self-signed or internally-signed
  certificates. Configure via `tls.caCertConfigMap` (ConfigMap name) and optionally `tls.caCertData`
  (PEM content via `--set-file`) so Helm creates the ConfigMap in both the controller and mounter
  pod namespaces automatically.
  See [TLS Configuration](driver-deployment/tls-configuration.md) for details.

- **Node startup taint watcher**: Added automatic taint removal to prevent the
  race condition where workload pods are scheduled before the CSI driver registers
  with kubelet. Cluster admins can pre-taint nodes with
  `s3.csi.scality.com/agent-not-ready:NoExecute`; the driver automatically removes
  the taint once it has registered, ensuring pods never see "driver not found" errors
  during node startup, reboot, or autoscaling events.
  See [Node Startup Taint](driver-deployment/node-startup-taint.md) for details.

- **Version-aware mounter pod management during rolling upgrades**: During a rolling
  upgrade of the CSI driver, new workloads will no longer reuse mounter pods created by a
  previous driver version. The controller now creates a fresh mounter pod running the same
  version as the current driver, ensuring consistent behavior throughout the upgrade window.
  Existing workloads continue running undisturbed on their current mounter pods until they
  are naturally rescheduled.

  **Why this matters:** Without version affinity, a newly-started driver node could reuse a
  mounter pod still running a previous driver version. If the two versions differ in
  communication protocol, mount options handling, or expected binary behavior, workloads
  could experience subtle and hard-to-diagnose mount failures during the upgrade window.
  Version-aware management eliminates this class of issue entirely.

### Bug Fixes

- **Concurrent volume mount race condition**: Fixed a race condition where multiple pods
  sharing the same volume could fail to mount when starting at the same time on the same
  node. The mount operation is now properly serialized so that only one pod sets up the
  S3 connection while others wait and reuse it.

  **Before this fix:** When two or more pods referencing the same PVC started concurrently
  on the same node, one pod could fail with a `"connection refused"` error. The pod would
  eventually start after Kubernetes retried the mount, but with a delay of up to 2 minutes.

  **After this fix:** All pods start without errors or delays. The first pod sets up the
  S3 mount; subsequent pods reuse it automatically.

  **Affected scenarios:** Deployments with `replicas > 1`, Jobs with `parallelism > 1`,
  pod restarts while another pod using the same volume is still mounting, and rolling
  updates of Deployments with shared volumes.

- **Duplicate S3PA creation during concurrent mounts**: Fixed a race condition where the
  controller could create duplicate MountpointS3PodAttachment resources when multiple pods
  mounting the same volume were reconciled concurrently. An expectations system now tracks
  pending S3PA creations, preventing the controller from creating a duplicate while a
  recently-created S3PA is not yet visible in the informer cache.

- **Prefix Parsing with Equals Signs**: Fixed mount option parsing that incorrectly truncated prefix
  values containing equals signs. For example, `prefix=env=prod/` was parsed as `--prefix=env` instead
  of `--prefix=env=prod/`. The parser now splits on whichever separator (space or `=`) appears first,
  preserving the full value.

### Breaking Changes

None.

## [2.1.1](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/2.1.1)

March 5, 2026

### Bug Fixes

- **Mounter Pod FSGroup**: Fixed volume mount failure when workload pods specify `fsGroup` in their
  `securityContext`. The mounter pod's communication socket (`/comm/mount.sock`) timed out because
  the emptyDir volume lacked proper group ownership. The fix adds `FSGroup` to the mounter pod's
  `PodSecurityContext`, ensuring the communication directory is writable by the non-root mount-s3
  process regardless of the workload pod's `fsGroup` configuration.

### Breaking Changes

None.

## [2.1.0](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/2.1.0)

January 27, 2026

### What's New

- **UUID Secret Key Support** ([#280](https://github.com/scality/mountpoint-s3-csi-driver/issues/280)): Added support for UUID format
  in secret key validation, enabling compatibility with S3 providers that use UUID-formatted secret keys.
- **Documentation Updates**: Updated architecture documentation to reflect v2 pod mounter strategy, including new Pod Mounter
  Architecture and CRD Reference pages, updated provisioning flows, and improved reference documentation.

### Bug Fixes

- **DeleteVolume Credentials**: Fixed DeleteVolume to use provisioner secrets from the request instead of always using
  driver-level credentials. This resolves bucket deletion failures when volumes were created with different account credentials.

### Breaking Changes

None.

## [2.0.1](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/2.0.1)

October 17, 2025

### Bug Fixes

- **Credential Validation**: Fixed validation logic that was incorrectly rejecting IAM standard 20-character access keys when using node-publish-secret in dynamic provisioning.
- **Enhanced Documentation**: Updated documentation to reflect correct usage for node pubblish creds for dynamic provisioning.
  Documentation updates for are available in
      - [Dynamic provisioning overview](./volume-provisioning/dynamic-provisioning/overview.md)
      - [Dynamic provisioning storage class reference and usage examples](./volume-provisioning/dynamic-provisioning/storageclass-reference-and-usage-examples.md)
      - [Dynamic provisioning credentials management](architecture/ring-s3-credentials-management/dynamic-provisioning-credentials-management.md)

## [2.0.0](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/2.0.0)

September 30, 2025

### What's New

- **Pod Mounter Strategy**: New default mounting approach using dedicated mounter pods for improved isolation and resource management.
- **MountpointS3PodAttachment CRD**: Introduced Custom Resource Definition to track volume attachments and enable volume sharing across multiple pods.
- **Scality RING 9.5.1+ Support**: Tested and validated compatibility with Scality RING version 9.5.1 and newer.
- **Enhanced Resource Management**: Automatic resource request/limit calculation for mounter pods based on cache size and mount options.
- **Controller Improvements**: Enhanced controller service with CRD reconciliation for managing pod attachments.

### Breaking Changes

- Default mounter strategy changed from systemd to pod-based mounter. Legacy systemd mounter is still available and will use pod-based mounter once the mount is requested again(pod restarts).
- Requires installation of MountpointS3PodAttachment CRD via kustomize or manual application. Helm v3 does not support automatic installation of CRDs on upgrades.

### Upgrade Notes

- **Required Upgrade Path**: Must upgrade to v1.2.0 before upgrading to v2.0. Direct upgrades from versions earlier than v1.2.0 are not supported.
- **CRD Installation Required**: The MountpointS3PodAttachment CRD must be installed manually before upgrading. See the [Upgrade Guide](driver-deployment/upgrade-guide.md) for detailed instructions.
- **Version Specification**: Explicitly specify `--version 1.2.0` or `--version 2.0.0` in Helm commands to control the upgrade path.

For complete upgrade instructions, see the [Upgrade Guide](driver-deployment/upgrade-guide.md).

## [1.2.0](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/1.2.0)

August 21, 2025

### What's New

- **Dynamic Provisioning**: Added support for automatic S3 bucket creation during PersistentVolumeClaim provisioning referencing a storage class.
- **Controller Service**: Controller service is now deployed by default (previously experimental) to support dynamic provisioning.
- **Helm Chart Updates**:
      - Added global `s3.region` and `s3.endpointUrl` values that apply to both node and controller services.
      - Legacy `node.s3EndpointUrl` and `node.s3Region` values are still supported and take precedence for backward compatibility.
      - Added new `controller` section in Helm chart `values.yaml` with improved credential provider and RBAC permissions.
      - Added configurable `images.provisioner` section in `values.yaml` for controller sidecar image configuration.

## [1.1.1](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/1.1.1)

August 5, 2025

### What's New

- **Product Name Update**: Renamed from "Scality S3 CSI Driver" to "Scality CSI Driver for S3" per AWS legal requirements.
- **Enhanced Documentation**: Added comprehensive architecture documentation including:
  - System overview and deployment architecture diagrams
  - Static provisioning flow documentation with detailed workflows
  - S3 credentials management architecture and best practices

### Bug Fixes

- **SystemD D-Bus Reliability**: Fixed D-Bus connection recreation for improved SystemD mount reliability and error handling.
- **Documentation Improvements**: Remove base64 mention from credentials in kubernetes secrets as the CSI driver does not use it.

## [1.1.0](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/1.1.0)

June 26, 2025

### What's New

- Enabled `force-path-style` at the driver level to support fully qualified domain names (FQDNs) for RING S3 endpoints.

## [1.0.1](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/1.0.1)

August 5, 2025

### What's Changed

- Documentation update: Renamed from "Scality S3 CSI Driver" to "Scality CSI Driver for S3" to comply with AWS copyright policies.
- No functional changes to the service or driver behavior.

## [1.0.0](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/1.0.0)

June 13, 2025

### What's New

- General Availability (GA) release of the Scality S3 CSI Driver.
- Production‑ready [Helm chart](https://github.com/scality/mountpoint-s3-csi-driver/tree/main/charts/scality-mountpoint-s3-csi-driver) for production deployment.
- Static provisioning allows seamless integration of existing S3 buckets as Kubernetes PersistentVolumes.
- Flexible credential strategies: driver‑level credentials and per‑volume credentials managed with Kubernetes Secrets.
- Optimized for Scality RING with advanced configuration options.
- Documentation site: <https://scality.github.io/mountpoint-s3-csi-driver/>.

### Current Limitations

- Static provisioning only; dynamic provisioning is planned for a future release.
- Optimized for sequential writes, random writes and file modifications follow S3 semantics.
- Single‑writer semantics per object ensure data consistency.
- Symbolic and hard links are not supported due to S3 limitations.
- Empty directories require at least one object in order to persist in S3.
- SELinux enforcing mode on the Kubernetes host is not yet supported.
