# Upgrade Tests

This directory contains upgrade test infrastructure for the Scality CSI Driver for S3.

## Overview

The upgrade tests validate that existing workloads continue to function after upgrading from v1.2.0 to the latest development version.

**Note**: S3 bucket operations are performed using the AWS SDK v2 for Go, which connects directly to the S3 endpoint using the configured `S3_ENDPOINT_URL` environment variable.

## Test Types

### Static Provisioning Test

Tests pre-provisioned PV/PVC/Pod resources through upgrade:

- Creates S3 bucket manually and Kubernetes resources
- Writes test data before upgrade
- Verifies data persistence and new write capability after upgrade

### Dynamic Provisioning Test

Tests dynamically provisioned volumes through upgrade:

- Creates StorageClass and PVC (triggers automatic bucket/PV creation)
- Writes test data before upgrade
- Verifies data persistence and new write capability after upgrade
- Tests full CSI controller functionality

### Manifest Files

**Static Provisioning:**

- `manifests/static-pv.yaml` - PersistentVolume for static test
- `manifests/static-pvc.yaml` - PersistentVolumeClaim for static test  
- `manifests/static-pod.yaml` - Pod for static test

**Dynamic Provisioning:**

- `manifests/dynamic-storageclass.yaml` - StorageClass for dynamic test
- `manifests/dynamic-pvc.yaml` - PersistentVolumeClaim for dynamic test
- `manifests/dynamic-pod.yaml` - Pod for dynamic test

## Mage Targets

### Static Provisioning

- `mage setupStaticProvisioning` - Create bucket, PV, PVC, Pod, write test data
- `mage verifyStaticProvisioning` - Verify data persistence and new writes after upgrade
- `mage cleanupStaticProvisioning` - Remove all static test resources

### Dynamic Provisioning

- `mage setupDynamicProvisioning` - Create StorageClass, PVC, Pod, write test data
- `mage verifyDynamicProvisioning` - Verify data persistence and new writes after upgrade
- `mage cleanupDynamicProvisioning` - Remove all dynamic test resources

### Combined Tests (CI)

- `mage setupUpgradeTests` - Setup both static and dynamic tests
- `mage verifyUpgradeTests` - Verify both static and dynamic tests
- `mage cleanupUpgradeTests` - Cleanup both static and dynamic tests

## Usage

### Static Provisioning Only

```bash
# Load credentials and set S3 endpoint
source tests/e2e/scripts/load-credentials.sh
export HOST_IP=<ip of the host>
export S3_ENDPOINT_URL=http://${HOST_IP}:8000

# If using KIND cluster, set cluster name
export KIND_CLUSTER_NAME=<kind-cluster-name>

# Install v1.2.0
SCALITY_CSI_VERSION=1.2.0 mage install

# Setup static test
mage setupStaticProvisioning

# Upgrade to local version
mage up

# Verify upgrade
mage verifyStaticProvisioning

# Cleanup
mage cleanupStaticProvisioning
```

### Dynamic Provisioning Only

```bash
# Load credentials and set S3 endpoint
source tests/e2e/scripts/load-credentials.sh
export HOST_IP=<ip of the host>
export S3_ENDPOINT_URL=http://${HOST_IP}:8000

# If using KIND cluster, set cluster name
export KIND_CLUSTER_NAME=<kind-cluster-name>

# Install v1.2.0
SCALITY_CSI_VERSION=1.2.0 mage install

# Setup dynamic test
mage setupDynamicProvisioning

# Upgrade to local version
mage up

# Verify upgrade
mage verifyDynamicProvisioning

# Cleanup
mage cleanupDynamicProvisioning
```

### Both Tests (Full Suite)

```bash
# Load credentials and set S3 endpoint
source tests/e2e/scripts/load-credentials.sh
export HOST_IP=<ip of the host>
export S3_ENDPOINT_URL=http://${HOST_IP}:8000

# If using KIND cluster, set cluster name
export KIND_CLUSTER_NAME=<kind-cluster-name>

# Install v1.2.0
SCALITY_CSI_VERSION=1.2.0 mage install

# Setup all tests
mage setupUpgradeTests

# Upgrade to local version
mage up

# Verify all tests
mage verifyUpgradeTests

# Cleanup all tests
mage cleanupUpgradeTests

# Clean up driver
mage down
```

## Prerequisites

- Kubernetes cluster (Kind or Minikube)
- S3-compatible storage running on port 8000
- Credentials in `tests/e2e/integration_config.json`
