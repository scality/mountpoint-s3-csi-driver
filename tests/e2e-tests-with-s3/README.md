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

This will generate a JUnit XML test report at `tests/e2e-tests-with-s3/e2e-with-cloudserver-results.xml` that can be used for test analytics in Codecov or other CI systems.

## Test Results Analytics

The tests produce a JUnit XML report that's compatible with Codecov's test analytics feature. When running in the CI environment, the test results are automatically uploaded to Codecov via the GitHub Action configured in the workflow.

If you're running the tests locally and want to see the results in Codecov, you'll need to upload them manually using the Codecov CLI or web interface.

## Test Structure

- `e2e_suite_test.go` - Main test suite setup and teardown
- `s3_test.go` - Tests for S3 interaction and CSI driver functionality
- `secrets_test.go` - Tests for Kubernetes secret management 
- `environment_test.go` - Tests for environment verification

## Adding New Tests

To add new tests:

1. Create a new test file with the `_test.go` suffix
2. Import the Ginkgo and Gomega packages
3. Add your tests using the Ginkgo DSL (`Describe`, `Context`, `It`, etc.)
4. Run the tests to verify they work

## CloudServer Configuration

The E2E tests use CloudServer, an open-source S3-compatible server, for testing S3 interactions without requiring actual AWS credentials.

The `scripts/setup_cloudserver.sh` script handles:
- Starting CloudServer in Docker
- Configuring test credentials
- Creating a test bucket

Default credentials:
- Access Key: `accessKey1`
- Secret Key: `verySecretKey1`
- Endpoint: `http://localhost:8000` 