# Upgrade Tests

This directory contains upgrade test infrastructure for the Scality CSI Driver for S3.

## Overview

The upgrade tests validate that existing workloads continue to function after upgrading from v1.2.0 to the latest development version.

**Note**: S3 bucket operations are performed using the AWS SDK v2 for Go, which connects directly to the S3 endpoint using the configured `S3_ENDPOINT_URL` environment variable.

## Test Types

### Static Provisioning Test

Tests pre-provisioned PV/PVC/Pod resources through upgrade:

- Creates S3 bucket and Kubernetes resources
- Writes test data before upgrade
- Verifies data persistence and new write capability after upgrade

### Manifest Files

- `manifests/static-pv.yaml` - PersistentVolume for static test
- `manifests/static-pvc.yaml` - PersistentVolumeClaim for static test  
- `manifests/static-pod.yaml` - Pod for static test

## Mage Targets

### Static Provisioning

- `mage setupStaticProvisioning` - Create bucket, PV, PVC, Pod, write test data
- `mage verifyStaticProvisioning` - Verify data persistence and new writes after upgrade
- `mage cleanupStaticProvisioning` - Remove all static test resources

## Usage

```bash
# Load credentials and set S3 endpoint
source tests/e2e/scripts/load-credentials.sh
export HOST_IP=<ip of the host>
export S3_ENDPOINT_URL=http://${HOST_IP}:8000

# if using kind cluster
export KIND_CLUSTER_NAME=<name of the kind cluster>

# Install v1.2.0
SCALITY_CSI_VERSION=1.2.0 mage install

# Setup test
mage setupStaticProvisioning

# Upgrade to local version
mage up

# Verify upgrade
mage verifyStaticProvisioning

# Cleanup
mage cleanupStaticProvisioning

# Clean up of driver
mage down
```

## Prerequisites

- Kubernetes cluster (Kind or Minikube)
- S3-compatible storage running on port 8000
- Credentials in `tests/e2e/integration_config.json`
