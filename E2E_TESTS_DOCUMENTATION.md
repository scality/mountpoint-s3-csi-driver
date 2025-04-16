# S3 CSI Driver E2E Kubernetes Tests Documentation

This document provides comprehensive documentation for the end-to-end (E2E) tests for the S3 CSI Driver on Kubernetes. These tests verify that the driver works correctly with S3 buckets in a real Kubernetes environment.

## Table of Contents
- [Introduction](#introduction)
- [Test Environment Setup](#test-environment-setup)
- [Test Execution Guide](#test-execution-guide)
- [Test Suite Overview](#test-suite-overview)
- [Test Scripts Documentation](#test-scripts-documentation)
- [S3 Client Implementation](#s3-client-implementation)
- [Test Implementation Details](#test-implementation-details)
- [Performance Testing with FIO](#performance-testing-with-fio)
- [Troubleshooting](#troubleshooting)
- [Extending the Tests](#extending-the-tests)
- [CI Integration](#ci-integration)

## Introduction

The E2E test framework verifies that the S3 CSI Driver can correctly:
- Mount S3 buckets as volumes in Kubernetes pods
- Support both standard S3 and S3 Express storage classes
- Handle authentication and credential management
- Perform basic file operations on mounted volumes
- Configure various mount options
- Support caching mechanisms

The tests are built using the Kubernetes E2E test framework and Ginkgo testing library, providing integration with standard Kubernetes testing patterns while adding S3-specific test cases.

### Architecture Overview

The test framework consists of the following components:

1. **Test Driver** (`testdriver.go`) - Implements the Kubernetes storage test driver interface
2. **Test Suites** (`testsuites/`) - Contains the test implementations
3. **S3 Client** (`s3client/`) - Handles S3 bucket operations
4. **Scripts** (`scripts/`) - Manages the test environment
5. **Performance Testing** (`fio/`) - Contains FIO configurations for performance testing

## Test Environment Setup

### Prerequisites

To run these tests, you need:
- AWS CLI configured with appropriate credentials
- Go development environment (1.19+)
- kubectl
- Access to create AWS resources (S3 buckets, EC2 instances, etc.)

Required AWS permissions are listed in the [README.md](./README.md#prerequisites).

### Environment Variables

Important environment variables for test configuration:
```bash
export KOPS_STATE_FILE="s3://your-kops-state-store" # set KOPS_STATE_FILE to your bucket when running locally
export AWS_REGION=us-east-1
export TAG=<version-tag> # CSI Driver image tag to install
export IMAGE_NAME="s3-csi-driver" # repository is inferred from current AWS account and region
export SSH_KEY=/path/to/your/ssh/key.pub # optional
export K8S_VERSION="1.30.0" # optional, must be a full version
export CLUSTER_TYPE=kops # or eksctl
export ARCH=x86 # or arm64
export MOUNTER_KIND=systemd # type of mounter to use
```

### Resource Requirements

The tests require:
- Kubernetes cluster with at least 3 nodes
- Sufficient IAM permissions for S3 operations
- Sufficient quota for EC2 instances
- Network connectivity to AWS S3 endpoints

### Cluster Creation Options

The test framework supports two types of Kubernetes clusters:

1. **kops** - Kubernetes Operations, a tool to create production-ready Kubernetes clusters on AWS
   - Provides more flexibility for cluster configuration
   - Requires a state store in S3
   - Configuration file: `kops-patch.yaml` and related files

2. **eksctl** - Amazon EKS CLI, a tool for creating and managing clusters on Amazon EKS
   - Easier to use with Amazon EKS
   - Better integration with AWS services
   - Configuration file: `eksctl-patch.json` and related files

The cluster type is specified using the `CLUSTER_TYPE` environment variable.

## Test Execution Guide

All commands should be executed from the repository root as described in the [README.md](./README.md).

### Full Test Sequence

```bash
# 1. Install required tools
ACTION=install_tools tests/e2e-kubernetes/scripts/run.sh

# 2. Create a Kubernetes cluster
ACTION=create_cluster tests/e2e-kubernetes/scripts/run.sh

# 3. Update kubeconfig
ACTION=update_kubeconfig tests/e2e-kubernetes/scripts/run.sh

# 4. Install the S3 CSI driver
ACTION=install_driver tests/e2e-kubernetes/scripts/run.sh

# 5. Run tests
ACTION=run_tests tests/e2e-kubernetes/scripts/run.sh

# 6. Clean up
ACTION=uninstall_driver tests/e2e-kubernetes/scripts/run.sh
ACTION=delete_cluster tests/e2e-kubernetes/scripts/run.sh
```

### Local vs CI Test Execution

When running tests locally:
- You need to provide your own S3 bucket for KOPS_STATE_FILE
- You should use your own AWS credentials
- You can choose to run specific tests or suites

In CI:
- State bucket and credentials are provided by the CI environment
- All tests are run automatically
- Results are reported in the CI logs

### Test Command-Line Parameters

The test command supports several parameters:
```
--bucket-region: AWS region for creating test buckets
--commit-id: Commit ID used for naming test buckets
--bucket-prefix: Prefix for test bucket names
--performance: Run performance tests (boolean)
--imds-available: Whether instance metadata service is available (boolean)
```

## Test Suite Overview

### Active Test Suites

The test suites in `e2e_test.go` define which tests are run:

```go
var CSITestSuites = []func() framework.TestSuite{
    testsuites.InitVolumesTestSuite,
    custom_testsuites.InitS3CSIMultiVolumeTestSuite,
    custom_testsuites.InitS3MountOptionsTestSuite,
    custom_testsuites.InitS3CSICredentialsTestSuite,
    custom_testsuites.InitS3CSICacheTestSuite,
}
```

1. **Volume Tests** (`testsuites.InitVolumesTestSuite`)
   - Tests basic volume operations
   - Writes and reads data from mounted volumes
   - Verifies content integrity
   - **Key test**: Writing 53 bytes to index.html file, then reading and verifying content from another pod

2. **Multi-Volume Tests** (`custom_testsuites.InitS3CSIMultiVolumeTestSuite`)
   - Tests mounting multiple S3 buckets simultaneously
   - Verifies isolation between volumes
   - Checks that data written to one volume doesn't appear in another
   - Tests defined in `testsuites/multivolume.go`

3. **Mount Options Tests** (`custom_testsuites.InitS3MountOptionsTestSuite`)
   - Tests different mount options for S3 volumes
   - Verifies that mount options are correctly applied
   - Tests specific mount option behaviors
   - Tests defined in `testsuites/mountoptions.go`

4. **Credentials Tests** (`custom_testsuites.InitS3CSICredentialsTestSuite`)
   - Tests various credential configurations
   - Verifies authentication methods work correctly
   - Tests access using different credential types
   - Tests defined in `testsuites/credentials.go`

5. **Cache Tests** (`custom_testsuites.InitS3CSICacheTestSuite`)
   - Tests caching functionality
   - Verifies cache behavior and performance
   - Tests data persistence across pod restarts
   - Tests defined in `testsuites/cache.go`

6. **Performance Tests** (`custom_testsuites.InitS3CSIPerformanceTestSuite`) - only run with `--performance=true` flag
   - Benchmarks performance metrics
   - Uses FIO for storage performance testing
   - Collects and reports performance data
   - Tests defined in `testsuites/performance.go`

### Skipped Test Suites

Several standard Kubernetes storage test suites are skipped (commented out in `e2e_test.go`) because they test functionality that doesn't apply to S3 storage:

- `InitCapacityTestSuite` - S3 doesn't have traditional capacity limits
- `InitVolumeIOTestSuite` - Tries to open a file for writing multiple times, which is unsupported by Mountpoint
- `InitVolumeModeTestSuite` - Block mode not supported by S3, only succeeds in checking unused volume is not mounted
- `InitSubPathTestSuite` - Subpath mounting not applicable
- `InitProvisioningTestSuite` - Dynamic provisioning not supported (static only)
- `InitMultiVolumeTestSuite` - Replaced by S3-specific multi-volume test
- `InitVolumeExpandTestSuite` - Volume expansion not applicable to S3
- `InitDisruptiveTestSuite` - Disruptive tests not applicable
- `InitVolumeLimitsTestSuite` - Volume limits not applicable
- `InitTopologyTestSuite` - Topology not applicable
- `InitVolumeStressTestSuite` - Generic stress tests not applicable
- `InitFsGroupChangePolicyTestSuite` - FsGroup policies not applicable
- `InitSnapshottableTestSuite` - Snapshots not applicable
- `InitSnapshottableStressTestSuite` - Snapshot stress tests not applicable
- `InitVolumePerformanceTestSuite` - Replaced by S3-specific performance tests
- `InitReadWriteOncePodTestSuite` - ReadWriteOnce not applicable (S3 supports ReadWriteMany)

## Test Scripts Documentation

The `scripts` directory contains scripts that manage the test environment:

### run.sh

The main entry point for test execution. Supports various actions:

- `install_tools` - Installs required tools (kubectl, helm, kops, eksctl)
- `create_cluster` - Creates a Kubernetes cluster
- `update_kubeconfig` - Updates the kubeconfig file
- `install_driver` - Installs the S3 CSI driver
- `run_tests` - Runs the E2E tests
- `run_perf` - Runs performance tests
- `uninstall_driver` - Uninstalls the S3 CSI driver
- `delete_cluster` - Deletes the Kubernetes cluster
- `e2e_cleanup` - Cleans up resources created during tests

Implementation details:
- Sets up environment variables and directories
- Sources other scripts (kops.sh, eksctl.sh, helm.sh)
- Executes the requested action
- Handles errors and exit codes

### kops.sh

Functions for creating and managing kops clusters:
- `kops_install` - Installs kops
- `kops_create_cluster` - Creates a Kubernetes cluster using kops
- `kops_delete_cluster` - Deletes a kops cluster

Configuration is done via:
- Environment variables
- Patch files for customization
- Command-line arguments

### eksctl.sh

Functions for creating and managing EKS clusters:
- `eksctl_install` - Installs eksctl
- `eksctl_create_cluster` - Creates an EKS cluster
- `eksctl_delete_cluster` - Deletes an EKS cluster

Configuration is done via:
- Environment variables
- JSON patch files
- Command-line arguments

### helm.sh

Handles driver installation via Helm:
- `helm_install` - Installs Helm
- `helm_install_driver` - Installs the S3 CSI driver using Helm
- `helm_uninstall_driver` - Uninstalls the S3 CSI driver
- `driver_installed` - Checks if the driver is installed

## S3 Client Implementation

The `s3client` directory contains the S3 client implementation used by the tests:

### Client Structure

The S3 client is implemented in `s3client/client.go` and provides:
- Bucket creation and deletion
- Support for both standard S3 and S3 Express directory buckets
- Authentication configuration
- Error handling

### Authentication Methods

The client supports multiple authentication methods:
- IAM roles - Used when running in AWS with appropriate IAM roles
- Access keys - Used when explicit credentials are provided
- Instance metadata - Used when running on EC2 instances

Environment variables for authentication:
- `S3_ENDPOINT_URL` - Custom S3 endpoint
- `AWS_ACCESS_KEY_ID` - Access key ID
- `AWS_SECRET_ACCESS_KEY` - Secret access key

### Bucket Management

The client provides functions for:
- Creating standard S3 buckets
- Creating S3 Express directory buckets
- Deleting buckets and their contents
- Waiting for bucket availability

Bucket naming convention:
- Based on cluster name, commit ID, and random suffix
- Ensures uniqueness for parallel test runs

### Error Handling

The client implements error handling for:
- Connection issues
- Permission errors
- Bucket already exists
- Bucket not empty
- Service unavailable

## Test Implementation Details

### Test Driver

The `testdriver.go` file implements the Kubernetes storage test driver interface:

Key components:
- `s3Driver` struct - Implements the test driver interface
- `s3Volume` struct - Represents an S3 volume
- Test driver initialization and configuration
- Volume creation and mounting

Implementation details:
- Implements `framework.TestDriver` interface
- Implements `framework.PreprovisionedVolumeTestDriver` interface
- Implements `framework.PreprovisionedPVTestDriver` interface
- Skips unsupported test patterns

### Volume Creation Workflow

1. Test creates a bucket via S3 client (`CreateVolume` method)
2. Driver configures the bucket as a persistent volume (`GetPersistentVolumeSource` method)
3. Kubernetes framework creates PVs and PVCs
4. Test pods are created that mount these volumes
5. Tests perform operations on the mounted volumes
6. Resources are cleaned up after tests (`DeleteVolume` method)

### Test Utilities

The `testsuites/util.go` file provides helper functions:

- `genBinDataFromSeed` - Generates random data for tests
- `checkWriteToPath` - Verifies writing to a path
- `checkReadFromPath` - Verifies reading from a path
- `createVolumeResourceWithMountOptions` - Creates volume with specific mount options
- `createPod` - Creates test pods
- `createPodWithServiceAccount` - Creates pods with specific service accounts
- Other utility functions for test implementation

## Performance Testing with FIO

The `fio` directory contains FIO (Flexible I/O Tester) configurations for benchmarking:

### FIO Configuration

Configuration files define:
- Test duration
- I/O patterns (random, sequential)
- Block sizes
- Number of jobs
- Read/write mix

### Running Performance Tests

Performance tests are run with:
```bash
ACTION=run_perf tests/e2e-kubernetes/scripts/run.sh
```

Or directly via:
```bash
KUBECONFIG=${KUBECONFIG} go test -ginkgo.vv --bucket-region=${REGION} --commit-id=${TAG} --bucket-prefix=${CLUSTER_NAME} --performance=true --imds-available=true
```

### Metrics Collected

The performance tests collect and report:
- Read/write throughput (MB/s)
- IOPS (I/O operations per second)
- Latency statistics (min, avg, max, percentiles)
- CPU utilization during tests

Results are stored in JSON format for analysis.

## Troubleshooting

### Common Issues

1. **Cluster creation failure**
   - Check AWS permissions
   - Verify region quotas
   - Check network configuration
   - Examine kops/eksctl logs

2. **Driver installation failure**
   - Verify image tag and availability
   - Check Helm configuration and logs
   - Examine pod logs
   - Check for conflicting resources

3. **Test failures**
   - Check S3 bucket accessibility
   - Verify authentication configuration
   - Check network connectivity to S3
   - Examine test pod logs

4. **S3 Access Issues**
   - Verify IAM permissions
   - Check for bucket policy restrictions
   - Verify credentials are correctly passed
   - Check for endpoint configuration issues

### Log Collection

Important logs to collect:

```bash
# Check driver logs
kubectl logs -l app=s3-csi-node -n kube-system

# Check test pod logs
kubectl logs -n <test-namespace> <pod-name>

# Check kops logs (if using kops)
kops get cluster
kops validate cluster

# Check eksctl logs (if using eksctl)
eksctl get cluster
```

## Extending the Tests

### Adding New Test Cases

1. Identify the appropriate test suite for your test
2. Create a new test function using Ginkgo's `It` block
3. Implement the test logic using helper functions from `testsuites/util.go`
4. Add appropriate assertions and cleanup

Example:
```go
It("should write and read files with custom permissions", func() {
    // Test implementation
})
```

### Adding Tests for New S3 Implementations

1. Extend the S3 client to support the new implementation:
   - Add new authentication methods if needed
   - Add new bucket creation functions if needed
   - Handle implementation-specific errors

2. Add test cases that verify implementation-specific functionality:
   - Create a new test suite if necessary
   - Add tests for unique features
   - Test compatibility with existing functionality

3. Update configuration to support new implementation parameters:
   - Add environment variables for configuration
   - Update driver to use new parameters

### Best Practices

- Clean up resources after tests
- Use unique identifiers for test resources
- Implement proper error handling
- Add appropriate logging for debugging
- Follow existing test patterns
- Make tests independent of each other
- Avoid assumptions about the environment

## CI Integration

The E2E tests are run as part of the CI pipeline:

### CI Configuration

The tests are executed in CI using the workflow defined in `.github/workflows/ci-and-e2e-tests.yaml`:
1. Build the S3 CSI driver image
2. Create a test cluster
3. Install the driver with the built image
4. Run the E2E tests
5. Clean up resources

### Environment Setup in CI

CI environment uses:
- Predefined AWS credentials
- Automated cluster creation
- Parallel test execution
- Automatic cleanup

### Test Reports

Test results are reported in CI output:
- Test successes and failures
- Performance metrics (if applicable)
- Resource usage
- Test execution time

### Common CI Issues

- Timeouts during cluster creation
- Permission issues with AWS resources
- Race conditions in parallel tests
- Resource cleanup failures

For detailed information on specific topics, see the code comments and documentation in the respective directories. 