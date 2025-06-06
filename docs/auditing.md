# Auditing CSI Driver Usage

This guide explains how to monitor and audit Scality S3 CSI Driver usage through S3 access logs and user-agent analysis.

## Overview

The Scality S3 CSI Driver automatically includes detailed information in HTTP User-Agent headers for all S3 requests, making it easy to track and audit driver usage across your infrastructure.

## User-Agent Information

### Automatic User-Agent Prefix

The CSI driver automatically sets a user-agent prefix with the following format:

```sh
s3-csi-driver/{VERSION} credential-source#{AUTH_SOURCE} k8s/{K8S_VERSION}
```

**Components:**

- **`s3-csi-driver/{VERSION}`** - Driver name and version (e.g., `s3-csi-driver/1.0.0`)
- **`credential-source#{AUTH_SOURCE}`** - Authentication method used:
  - `credential-source#driver` - Driver-level credentials (static keys)
  - `credential-source#secret` - Volume-level credentials (Kubernetes secrets)
- **`k8s/{K8S_VERSION}`** - Kubernetes cluster version (e.g., `k8s/v1.29.6` or `k8s/v1.30.2-eks-db838b0`)

### Example User-Agent Headers

```sh
s3-csi-driver/0.6.0 credential-source#driver k8s/v1.29.6
s3-csi-driver/0.6.0 credential-source#secret k8s/v1.30.2-eks-db838b0
s3-csi-driver/1.1.0 credential-source#driver k8s/v1.28.5-gke.1234567
```
