# Administrator and User Guide

This guide defines roles and responsibilities for administrators managing the Scality CSI Driver for S3 and users consuming S3 storage in Kubernetes.

The driver supports both **Dynamic Provisioning** (automatic S3 bucket creation and deletion if empty) and **Static Provisioning** (manual bucket and PV creation).

## Administrator Responsibilities

### Common Tasks (Both Provisioning Types)

- Install and configure the driver (Helm, global settings, upgrades)
- Manage network connectivity to S3
- Set mount options and manage credentials
- Rotate credentials, implement least-privilege, monitor logs

### Dynamic Provisioning

- Create and manage StorageClasses with S3 parameters
- Configure credential secrets for bucket operations
- Set up credential templating for multi-tenancy
- Monitor automatic bucket creation and deletion

### Static Provisioning

- Manually create S3 buckets
- Create PersistentVolumes for existing buckets
- Manage per-volume credential secrets

## User Responsibilities

### Dynamic Provisioning

- Create PVCs referencing StorageClasses
- Mount volumes in pods (including inline pod creation with PVCs)
- Understand S3 consistency and error handling
- Follow naming conventions and data management best practices

### Static Provisioning

- Create PVCs referencing administrator-provided PVs
- Mount volumes in pods
- Understand S3 consistency and error handling
- Follow naming conventions and data management best practices

## Communication Workflows

### Dynamic Provisioning Workflow

1. **Administrator Setup**: Creates StorageClass with bucket parameters and credential configuration
2. **User Storage Request**: Creates PVC referencing StorageClass (bucket created automatically)
3. **User Application Deployment**: Deploys pods with PVC references (volume mounted automatically)

### Static Provisioning Workflow

1. **User Storage Request**: Requests storage (bucket, access, requirements)
2. **Administrator Provision**: Reviews, creates bucket/PV, provides PV name
3. **User Application Deployment**: Creates PVC and deploys application
