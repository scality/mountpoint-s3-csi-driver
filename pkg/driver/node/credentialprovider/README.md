# Credential Provider Package

This package handles authentication credential management for the S3 CSI Driver.

## Overview

The credential provider package is responsible for obtaining and managing AWS credentials that allow the driver to access S3 buckets. It supports multiple authentication sources and credential types.

## Key Components

### Provider

The main `Provider` struct is responsible for:
- Determining the appropriate credential source based on configuration
- Retrieving credentials from the selected source
- Formatting credentials for use by Mountpoint for S3
- Cleaning up credentials after use

### Authentication Sources

The package supports multiple authentication sources:

1. **Driver** - Uses credentials configured for the driver itself
   - IAM roles for service accounts (IRSA)
   - Kubernetes secrets
   - Node instance profiles

2. **Pod** - Uses credentials from the pod's service account
   - Particularly useful for multi-tenant setups where different pods need different permissions

### Context Structures

- `ProvideContext` - Contains context for providing credentials (volume ID, pod ID, etc.)
- `CleanupContext` - Contains context for cleaning up credentials after unmounting

## Workflow

1. When a volume is mounted, `NodePublishVolume` calls the credential provider with a `ProvideContext`
2. The provider determines the authentication source from the volume context
3. Based on the source, it retrieves appropriate credentials
4. It formats these credentials for Mountpoint for S3 (environment variables or files)
5. When the volume is unmounted, `NodeUnpublishVolume` calls the provider with a `CleanupContext`
6. The provider cleans up any credentials files or resources

## AWS Region Handling

The package includes functionality for determining the AWS region:

- From volume context
- From mount options
- From IMDS (instance metadata service)

## Implementation Details
TODO: Change code to use credentials provider package
TODO: Update readme to show what type of credentials are used
TODO: Delete other roles based code while maintaining access token
### Driver-Level Credentials

For driver-level credentials, the provider:
1. Checks for credentials from IRSA attached to the driver's service account
2. Falls back to credentials from Kubernetes secrets
3. Finally falls back to instance profile credentials

### Pod-Level Credentials

For pod-level credentials, the provider:
1. Retrieves the pod's service account tokens
2. Uses the pod's service account for authentication
3. Applies appropriate IAM role mappings

## Security Considerations

The credential provider implements several security measures:
- Temporary file-based credentials with restricted permissions
- Credential rotation for long-lived mounts
- Proper cleanup of credentials after unmounting
- Isolation between credentials for different volumes

## See Also

- [Node Service](../README.md) - The main node service that uses the credential provider
- [Mounter Package](../mounter/README.md) - Uses the credentials provided by this package
