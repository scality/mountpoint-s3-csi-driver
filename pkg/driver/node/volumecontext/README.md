# Volume Context Package

This package defines constants for the volume context attributes used in the S3 CSI Driver.

## Overview

The volume context package provides a centralized location for the keys used in CSI volume context maps. These keys are used to pass information between the Kubernetes CSI subsystem and the driver.

## Key Constants

The package defines the following constants:
TODO: remove STSRegion, CSIServiceAccountName, CSIServiceAccountTokens as we do not need that but after updating credetials package
- `BucketName` - Key for the S3 bucket name
- `AuthenticationSource` - Key for the authentication source to use
- `STSRegion` - Key for the STS region to use for authentication
- `CSIPodUID` - Key for the pod UID (used in pod-level authentication)
- `CSIPodNamespace` - Key for the pod namespace
- `CSIServiceAccountName` - Key for the pod's service account name
- `CSIServiceAccountTokens` - Key for the pod's service account tokens

## Usage

These constants are used when:

1. Extracting information from a volume context in the node server:
   ```go
   bucket, ok := volumeCtx[volumecontext.BucketName]
   ```

2. Setting values in a PersistentVolume's `volumeAttributes` field (in YAML):
   ```yaml
   volumeAttributes:
     bucketName: my-s3-bucket
     authenticationSource: pod
   ```

## See Also

- [Node Service](../README.md) - The main node service that uses these volume context attributes
- [Credential Provider](../credentialprovider/README.md) - Uses these attributes for authentication 
