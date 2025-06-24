# Release Notes

## [1.0.0](https://github.com/scality/mountpoint-s3-csi-driver/releases/tag/1.0.0)

June 13, 2025

### What's New

- General Availability (GA) release of the Scality S3 CSI Driver.
- Production‑ready [Helm chart](https://scality.github.io/mountpoint-s3-csi-driver/driver-deployment/installation-guide/) for production deployment.
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
