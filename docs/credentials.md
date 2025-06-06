# Credentials Management

This guide explains how to manage AWS credentials for the Scality S3 CSI Driver, covering both driver-level and volume-level authentication methods.

## Overview

The CSI driver supports two authentication approaches:

- **Driver-level credentials** - Global credentials shared across all volumes
- **Secret-level credentials** - Per-volume credentials using Kubernetes Secrets

## Authentication Methods

### Driver-Level Authentication

Driver-level authentication uses a single set of credentials for all volumes managed by the CSI driver instance. These credentials are configured globally during driver installation.

**When to use:**

- Single S3 endpoint with consistent access patterns
- Simplified credential management
- All applications use the same S3 service account

**Configuration:**

1. **Create the credentials secret:**

   ```bash
   kubectl create secret generic s3-credentials \
     --from-literal=access_key_id="YOUR_ACCESS_KEY_ID" \
     --from-literal=secret_access_key="YOUR_SECRET_ACCESS_KEY" \
     --namespace kube-system
   ```

2. **Configure in Helm values:**

   ```yaml
   s3CredentialSecret:
     name: "s3-credentials"
   ```

3. **PersistentVolume configuration:**

   ```yaml
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: my-s3-volume
   spec:
     csi:
       driver: s3.csi.scality.com
       volumeHandle: my-bucket
       volumeAttributes:
         bucketName: my-bucket
         # authenticationSource defaults to "driver" if omitted
   ```

### Secret-Level Authentication

Secret-level authentication allows different volumes to use different credentials, providing better isolation and security.

**When to use:**

- Multi-tenant environments
- Different applications need different S3 accounts
- Enhanced security isolation between workloads
- Credential rotation per application

**Configuration:**

1. **Create volume-specific credentials:**

   ```bash
   kubectl create secret generic app1-s3-credentials \
     --from-literal=access_key_id="APP1_ACCESS_KEY_ID" \
     --from-literal=secret_access_key="APP1_SECRET_ACCESS_KEY" \
     --namespace my-app-namespace
   ```

2. **PersistentVolume configuration:**

   ```yaml
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: app1-s3-volume
   spec:
     csi:
       driver: s3.csi.scality.com
       volumeHandle: app1-bucket
       volumeAttributes:
         bucketName: app1-bucket
         authenticationSource: "secret"
       nodePublishSecretRef:
         name: app1-s3-credentials
         namespace: my-app-namespace
   ```

## Secret Format Requirements

### Required Keys

Both authentication methods require secrets with these keys:

- **`access_key_id`** - S3 Access Key ID (1-16 alphanumeric characters)
- **`secret_access_key`** - S3 Secret Access Key (1-40 characters, [A-Za-z0-9/+=])

### Optional Keys

- **`session_token`** - For temporary credentials (STS tokens)
