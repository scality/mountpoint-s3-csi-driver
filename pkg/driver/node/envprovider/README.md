# Environment Provider Package

This package handles environment variable management for S3 credentials.

## Overview

The environment provider package provides a mechanism for creating and managing environment variables needed for Mountpoint for S3 authentication. It handles the creation of environment files that can be sourced by the mounting process.

## Key Features

- Creates credential files with appropriate permissions
- Manages AWS credential environment variables
- Handles custom endpoint configuration
- Supports multiple authentication methods

## Core Functions

The package provides these main functions:
TODO: Remove STS endpoint configuration
- `ProvideAWSCredentials` - Writes AWS credentials to environment files
- `ProvideAWSEndpoint` - Configures custom S3 endpoints
- `ProvideAWSStsEndpoint` - Configures custom STS endpoints

## Environment Variables

The package manages several AWS environment variables including:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_SESSION_TOKEN`
- `AWS_REGION`
- `AWS_S3_ENDPOINT`
- `AWS_STS_ENDPOINT`

## Security Considerations

- Environment files are created with restricted permissions
- Files are cleaned up after use
- Variables are isolated between different mounts

## Usage

This package is primarily used by the credential provider to prepare environment variables for the mounter process:

```go
err := envprovider.ProvideAWSCredentials(path, credentials)
if err != nil {
    // Handle error
}
```

## See Also

- [Credential Provider](../credentialprovider/README.md) - Uses this package to manage credentials
- [Mounter Package](../mounter/README.md) - Uses the environment files created by this package
