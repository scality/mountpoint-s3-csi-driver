# Scality S3 CSI Driver E2E Tests

This directory contains end-to-end (E2E) tests for the Scality S3 CSI Driver. These tests verify that the driver functions correctly in a Kubernetes environment, interacting properly with Scality S3 storage.

## Directory Structure

```
tests/e2e-tests/
├── pkg/               # Common packages
│   ├── s3client/      # S3 client implementation
│   └── testutil/      # Test utilities (planned)
├── testsuites/        # Test suite implementations (planned)
├── e2e_test.go        # Main test file
├── testdriver.go      # Driver implementation (planned)
└── scripts/           # Test execution scripts (planned)
```

## Prerequisites

- Kubernetes cluster (local or remote)
- kubectl installed
- Scality S3 server or compatible S3 storage
- Go 1.24+ installed

## Running Tests

The tests can be executed using the following command:

```bash
go test -v ./... \
  -kubectl-path=/path/to/kubectl \
  -kubeconfig=/path/to/kubeconfig \
  -s3-endpoint-url=http://localhost:8000 \
  -access-key-id=accessKey1 \
  -secret-access-key=verySecretKey1 \
  -bucket-prefix=e2e-test
```

### Required Parameters

- `kubectl-path`: Path to the kubectl binary
- `kubeconfig`: Path to the kubeconfig file
- `s3-endpoint-url`: S3 endpoint URL for Scality S3 server
- `access-key-id`: S3 access key ID
- `secret-access-key`: S3 secret access key

### Optional Parameters

- `bucket-prefix`: Prefix for temporary buckets (default: e2e-test)
- `performance`: Enable performance tests (default: false)
- `-ginkgo.focus`: Focus on specific test cases

## Development

To add new tests, follow these steps:

1. Create a new test suite in the `testsuites` directory
2. Implement the required interfaces for the test suite
3. Register the test suite in `e2e_test.go`
4. Run the tests to verify functionality

## CI Integration

These tests are integrated with CI through GitHub Actions. The workflow configuration is located in `.github/workflows/ci-and-e2e-tests.yaml`. 