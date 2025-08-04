# Release Notes

## [1.0.1](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/1.0.1)

August 4, 2025

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
