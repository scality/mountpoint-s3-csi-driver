# E2E Test Packages

This directory contains shared packages used by the E2E tests for the Scality S3 CSI Driver.

## Packages

- [s3client](./s3client): Provides a client for interacting with S3-compatible storage services
- [testutil](./testutil): Test utility functions for verification of Kubernetes resources and common operations

## Overview

These packages provide the core functionality needed by the E2E tests to interact with both the Kubernetes cluster and the S3 storage service. They are designed to be reusable across different test suites and scenarios. 