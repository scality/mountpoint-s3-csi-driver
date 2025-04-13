# End-to-End Tests for Scality S3 CSI Driver

This directory contains end-to-end tests for the Scality S3 CSI Driver. The tests are written in Go and use the Kubernetes E2E framework.

## Directory Structure

- `kubernetes/` - Contains tests using the standard Kubernetes storage E2E framework
- `scality/` - Contains Scality-specific tests for additional functionality

## Running Tests

These tests are intended to be run after the CSI driver has been installed in a Kubernetes cluster. The tests will validate that the CSI driver is functioning correctly.

To run the tests:

```bash
cd tests/e2e-scality/e2e-tests
go test -v ./...
```

## Test Categories

The tests cover:

1. **Kubernetes Storage Tests**: Standard tests for CSI driver functionality
   - Dynamic provisioning
   - Static provisioning
   - Mount options
   - Volume expansion
   - Pod restart/rescheduling

2. **Scality-specific Tests**: Tests specific to the Scality S3 implementation
   - Authentication
   - Bucket management
   - Object lifecycle
   - Performance

## Development

When adding new tests, follow these guidelines:

1. Use the Go test framework and Ginkgo for BDD-style tests
2. Place Kubernetes storage tests in the `kubernetes/` directory
3. Place Scality-specific tests in the `scality/` directory
4. Ensure tests clean up after themselves
5. Document test requirements and any special setup needs 