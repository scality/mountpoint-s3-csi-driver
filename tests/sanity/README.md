# CSI Sanity Tests

This directory contains sanity tests for the S3 CSI Driver. These tests use the [CSI Sanity Testing framework](https://github.com/kubernetes-csi/csi-test) to verify that the driver implementation complies with the CSI specification.

## Overview

The CSI Sanity Testing framework runs a standardized set of tests against a CSI driver to ensure it correctly implements the Container Storage Interface (CSI) specification. These tests verify that the driver properly handles all required CSI calls and behavior.

## Why Some Tests Are Skipped

In the output of the sanity tests, you might see results like this:

```
Ran 10 of 72 Specs in 0.026 seconds
SUCCESS! -- 10 Passed | 0 Failed | 0 Pending | 62 Skipped
```

The S3 CSI Driver is designed for **static provisioning** only, meaning it doesn't implement the controller functionality for dynamic provisioning. As a result, most of the controller-related tests are skipped.

The following test cases are explicitly skipped (as configured in the Makefile):
- `ControllerGetCapabilities`
- `ValidateVolumeCapabilities`

## Why Static Provisioning Only?

The S3 CSI Driver focuses on mounting existing S3 buckets to Kubernetes pods. This design decision is intentional because:

1. S3 buckets are typically pre-created by administrators with specific naming, permissions, and lifecycle policies
2. Bucket creation is generally considered an administrative task rather than something to be dynamically handled
3. The static provisioning approach aligns better with how object storage is typically used in practice

## Running the Tests

To run the sanity tests, execute:

```bash
make test
```

This will run both regular driver tests and the sanity tests with appropriate skipping of controller-related test cases.

## Test Implementation

The sanity tests in `sanity_test.go` set up a minimal S3 CSI driver instance with a fake mounter and run the standardized CSI test suite against it. The test configuration includes:

- Setting up a Unix domain socket for communication
- Creating temporary mount and staging paths
- Running a minimal driver implementation with a fake mounter
- Configuring the test volume size and other parameters

The tests focus primarily on the node service functionality, which is the core of the static provisioning capabilities of this driver. 
