# Codecov Integration Guide

## Overview

This document outlines the direct Codecov integration implemented in ticket S3CSI-10, replacing the AWS-specific test coverage tools.

## Changes Made

1. **Removed AWS-specific components**:
   - Deleted `.testcoverage.yml` file
   - Removed go-test-coverage installation
   - Updated the Makefile `cover` target

2. **Updated CI workflow**:
   - Added direct upload to Codecov in the unit tests job
   - Configured with the correct repository slug (`scality/mountpoint-s3-csi-driver`)
   - Set to use GitHub Actions token for authentication

3. **Simplified coverage file handling**:
   - Using `coverage.out` file directly
   - Using standard Go coverage output format

## How It Works

The new integration works as follows:

1. The `test-unit` target generates a standard Go coverage file (`coverage.out`)
2. The GitHub Action `codecov/codecov-action@v5` uploads this file directly to Codecov
3. Codecov processes the coverage data and updates the repository dashboard

## Validating the Integration

To verify the Codecov integration is working correctly:

1. **Check the GitHub Actions workflow logs**:
   - Look for successful upload messages from the Codecov action
   - Verify that no errors are reported during the upload step

2. **Verify coverage in Codecov dashboard**:
   - Visit the [Codecov dashboard](https://codecov.io/gh/scality/mountpoint-s3-csi-driver) after a workflow run
   - Confirm that coverage data is being updated
   - Check that the coverage report matches expectations

3. **PR Checks**:
   - Verify that Codecov is adding comments to pull requests
   - Confirm that coverage changes are being reported accurately

## Troubleshooting

If there are issues with the Codecov integration:

1. **Check Codecov token**:
   - Ensure the `CODECOV_TOKEN` secret is correctly set in GitHub repository settings
   - Verify the token has the necessary permissions

2. **Verify coverage files**:
   - Download and inspect the coverage files from workflow artifacts
   - Check if the format is compatible with Codecov

3. **Test locally**:
   - Run `make test-unit` locally and examine the generated `coverage.out` file
   - Validate the file with `go tool cover -func=coverage.out`

## References

- [Codecov GitHub Action Documentation](https://github.com/codecov/codecov-action)
- [Go Code Coverage Documentation](https://go.dev/blog/cover)
- [Codecov Integration Guide](https://docs.codecov.io/docs) 