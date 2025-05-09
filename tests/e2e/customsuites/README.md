# Custom Test Suites for S3 CSI Driver

This package provides test suites specific to the Scality S3 CSI driver. It extends the standard Kubernetes storage test framework with tests that validate Scality-specific functionality.

## Test Suites

### Mount Options Test Suite

The mount options test suite (`mountoptions.go`) verifies that the S3 CSI driver correctly handles volume mount options related to permissions, user/group IDs, and access controls when mounting S3 buckets in Kubernetes pods. It includes tests for:

- Access to volumes when mounted with non-root user/group IDs
- Proper enforcement of permissions when mount options are absent
- File and directory ownership when mounting with specific uid/gid

### File Permissions Test Suite

The file permissions test suite (`filepermissions.go`) validates file permission behavior for the S3 CSI driver, focusing on the file-mode mount option and ensuring correct, consistent enforcement of file metadata semantics across a range of scenarios.

**File Permission Tests:**

- Default file permissions (0644) when no mount options are specified
- Custom file permissions via the `file-mode` mount option
- Permission inheritance in subdirectories
- Remount behavior with updated mount options
- Multi-pod access with different permissions
- Permission preservation during standard file operations
- FSGroup and security context ownership verification

**Contrarian & Edge-Case Behavior Tests:**

- `chmod` operations fail (EPERM/ENOTSUP) and permissions remain unchanged
- `chown` operations fail post-creation (immutability enforced)
- Umask has no effect; driver-enforced permissions always apply
- Symlink creation is blocked (EPERM)
- Truncation of existing files is blocked
- Extended attributes (xattr) are unsupported (EPERM/ENOTSUP)
- `access()` syscall consistency with `stat()`
- Edge-case filename support (Unicode, long names, special chars)
- Pod umask + fsGroup conflict handling

### Directory Permissions Test Suite

The directory permissions test suite (`directorypermissions.go`) validates directory permission behavior for the S3 CSI driver, focusing on the `dir-mode` mount option. It complements the file permissions tests by ensuring correct application, consistency, and immutability of directory permissions under various scenarios.

**Directory Permission Tests:**

- Default directory permissions (`0755`) when no mount options are specified
- Custom directory permissions via the `dir-mode` mount option
- Distinct directory permissions for multiple volumes in the same pod
- Directory permission updates after PV mount option changes (remount behavior)
- Recursive application of directory permissions to nested subdirectories
- Directory permission preservation during file operations (e.g., copy, recursive copy)
- Multi-pod mounts showing different permissions based on mount timing
- FSGroup and security context ownership verification

**Contrarian & Edge-Case Behavior Tests:**

- `chmod` on directories succeeds but has no effect; permissions remain unchanged (noop)
- `chown` operations fail (EPERM/ENOTSUP); ownership is immutable post-mount
- Umask has no effect; driver-enforced directory permissions always apply
- `mkdir -m` explicit mode bits are ignored; `dir-mode` is always enforced
- Directory `mv`/rename operations fail cleanly (ENOTSUP or equivalent)
- `access()` syscall reports consistent permissions matching `stat()`
- Edge-case directory name support (Unicode, long names, special characters)
- Pod umask + security context conflict handling (dir-mode remains authoritative)

This suite ensures that directory permissions are correctly applied and maintained across various usage scenarios, providing proper access control for directory structures within S3 volumes.

### Credentials Test Suite

The credentials test suite (`credentials.go`) validates authentication and authorization behavior for the S3 CSI driver, covering both successful mounts (default and secret‐mounted credentials) and expected failure modes.

**Credentials Tests:**

- Default driver credentials Usage: Mount with the built-in access key/secret → write an object → verify owner ID matches the driver’s canonical ID (Bart)  
- Secret-mounted volume credentials: Provide access key/secret via a Kubernetes Secret (`authenticationSource=secret`) → mount → write an object → verify owner ID matches the Secret’s canonical ID (Lisa)  

**Error & Negative Tests:**

- Invalid access key: Mount with a non-existent key → expect “access key Id does not exist” error  
- Unauthorized credentials: Mount with valid key but no bucket permissions → expect “Access Denied Error: Failed to create mount process” failure  

### Multi-Volume Test Suite

The multi-volume test suite (`multivolume.go`) validates scenarios involving multiple volumes and pods to ensure the S3 CSI driver properly handles concurrent access and volume isolation. It includes tests for:

- Multiple pods accessing the same volume simultaneously
- A single pod accessing multiple volumes concurrently
- Data persistence across pod recreations with the same volume

This suite verifies the core functionality needed for both stateless and stateful workloads in Kubernetes when using S3 CSI volumes.

### Cache Test Suite

The cache test suite (`cache.go`) provides smoke tests to validate the caching functionality of the Mountpoint S3 client when deployed through the CSI driver. It includes tests for:

- Basic read/write operations with local caching enabled
- Persistence of cached data even after removal from the underlying S3 bucket
- Cache behavior with different user contexts (root and non-root)
- Cache sharing between containers in the same pod

Note that comprehensive caching functionality tests are part of the upstream [Mountpoint S3 project](https://github.com/awslabs/mountpoint-s3), while these tests focus specifically on validating CSI driver integration with caching features.

### Performance Test Suite

The performance test suite (`performance.go`) measures the I/O throughput and performance characteristics of the S3 CSI driver using the FIO (Flexible I/O Tester) benchmarking tool. This suite:

- Spawns multiple pods (N=3) on the same node accessing a shared volume
- Runs a series of FIO benchmarks to test different I/O patterns:
  - Sequential reads: Testing continuous read throughput from S3 objects
  - Sequential writes: Evaluating write performance for streaming data to S3
  - Random reads: Measuring performance when accessing S3 data in a non-sequential pattern

#### Benchmark Configuration Details

The FIO benchmarks are configured with these parameters:

- **Common Settings**:
  - Block size: 256KB for all tests
  - Runtime: 30 seconds (time-based)
  - I/O engine: sync

- **Sequential Read Test**:
  - File size: 10GB
  - Operation: Sequential read

- **Sequential Write Test**:
  - File size: 100GB
  - Operation: Sequential write 
  - fsync_on_close=1 (ensures data is committed to storage)
  - create_on_open=1 (creates the file when opened)
  - unlink=1 (removes the file after testing)

- **Random Read Test**:
  - File size: 10GB
  - Operation: Random read

#### Test Methodology

- Each pod creates and operates on its own test file (e.g., `/mnt/volume1/seq_read_0`) to prevent contention
- Tests run concurrently across all pods to measure performance under multi-client load
- The minimum throughput (MiB/s) observed across all pods is recorded as the baseline metric
- Results are saved to a JSON file in the `test-results/` directory for further analysis

This test suite is particularly valuable for:

- Establishing performance baselines for the S3 CSI driver
- Validating that multiple pods can concurrently access the same S3 volume with acceptable throughput
- Detecting performance regressions in driver updates
- Comparing performance across different S3 storage configurations

**Note:** Performance tests are disabled by default and can be enabled by using the `--performance` flag when running the E2E tests.

### Utilities

The `util.go` file contains utility functions that support all test suites:

- Helpers for file operations (read/write/verify)
- Pod configuration utilities
- Volume resource creation with custom mount options

## Adding New Test Suites

When adding new test suites to this package, follow these guidelines:

1. Create a new file named after the feature being tested (e.g., `multivolume.go`)
2. Implement the storage framework's `TestSuite` interface
3. Create an initializer function named `InitXXXTestSuite()`
4. Register the new test suite in `tests/e2e/e2e_test.go`
5. Add documentation for your test suite in this README

## Running Tests

Tests in this package are automatically executed as part of the [E2E test suite](../e2e_test.go) when running:

```sh
go test -v ./tests/e2e/...
```

For performance tests, use the `--performance` flag:

```sh
go test -v --performance ./tests/e2e/...
```

See the [main project documentation](../README.md) for details on setting up the test environment with proper credentials and S3 endpoint configuration.
