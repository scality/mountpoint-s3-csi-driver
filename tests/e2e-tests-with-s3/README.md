# E2E Tests with CloudServer

This directory contains end-to-end tests for the S3 CSI Driver using CloudServer (an S3-compatible server) for testing.

## Overview

These tests verify that the S3 CSI Driver can properly interact with an S3-compatible server in a Kubernetes environment. The test suite uses Ginkgo for test organization and reporting.

## Test Coverage

The test suite includes:

- Basic S3 operations (connection, bucket creation/deletion)
- CSI driver integration with CloudServer
- Volume mounting and data persistence

## Running Tests

To run the tests:

```bash
make e2e-with-cloudserver
```

To run tests and generate a code coverage report:

```bash
make e2e-with-codecov
```

The coverage report will be generated at `tests/e2e-coverage.html`.

## Test Structure

- `e2e_suite_test.go` - Main test suite setup and teardown
- `s3_test.go` - Tests for S3 interaction and CSI driver functionality

## Adding New Tests

To add new tests:

1. Create a new test file with the `_test.go` suffix
2. Import the Ginkgo and Gomega packages
3. Add your tests using the Ginkgo DSL (`Describe`, `Context`, `It`, etc.)
4. Run the tests to verify they work

## CloudServer Configuration

[Add information about how CloudServer is configured for these tests] 