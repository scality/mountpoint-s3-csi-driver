# Volume Provisioning

The Scality S3 CSI driver supports two methods of volume provisioning to meet different use cases and operational requirements.

## Provisioning Methods

### Static Provisioning
Pre-provision S3 buckets manually and create PersistentVolumes that reference existing buckets.

**Key characteristics:**
- **Manual bucket creation** required before use
- **Full control** over bucket configuration and naming
- **Flexible** - can use any existing S3 bucket
- **Direct bucket mapping** - one PV maps to one pre-existing bucket

**Best for:**
- Existing S3 infrastructure integration
- Custom bucket configurations
- Environments with strict bucket naming requirements
- Migration from existing storage setups

📖 **[Learn more about Static Provisioning →](static-provisioning/overview.md)**

### Dynamic Provisioning
Automatically create and manage S3 buckets through StorageClass configurations.

**Key characteristics:**
- **Automatic bucket creation** when PVCs are created
- **StorageClass-driven** configuration and management
- **Two modes**: Dedicated buckets or shared buckets with prefixes
- **Lifecycle management** - automatic cleanup when volumes are deleted

**Best for:**
- Cloud-native applications
- Standardized deployment workflows
- Development and testing environments
- Multi-tenant scenarios

📖 **[Learn more about Dynamic Provisioning →](dynamic-provisioning/overview.md)**

## Comparison

| Feature | Static Provisioning | Dynamic Provisioning |
|---------|--------------------|--------------------|
| **Bucket Creation** | Manual | Automatic |
| **Setup Complexity** | Higher | Lower |
| **StorageClass** | Empty string (`""`) | CSI driver name |
| **Flexibility** | High | Structured |
| **Bucket Naming** | Complete control | CSI-managed |
| **IAM Permissions** | Read/Write | + Bucket management |
| **Use Cases** | Existing infrastructure, custom configs | New applications, standardized workflows |

## Quick Start

### For New Applications (Recommended: Dynamic)

1. Create a StorageClass:
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-east-1
volumeBindingMode: Immediate
reclaimPolicy: Delete
```

2. Create a PVC:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-app-storage
spec:
  accessModes: [ReadWriteMany]
  storageClassName: s3-csi
  resources:
    requests:
      storage: 100Gi
```

### For Existing S3 Infrastructure (Static)

1. Create a PersistentVolume:
```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: existing-s3-pv
spec:
  capacity:
    storage: 1200Gi
  accessModes: [ReadWriteMany]
  storageClassName: ""
  csi:
    driver: s3.csi.scality.com
    volumeHandle: existing-bucket-pv
    volumeAttributes:
      bucketName: my-existing-bucket
```

2. Create a PVC that references the PV:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: existing-s3-pvc
spec:
  accessModes: [ReadWriteMany]
  storageClassName: ""
  volumeName: existing-s3-pv
  resources:
    requests:
      storage: 1200Gi
```

## Volume Features

Both provisioning methods support the same volume features:

- **Multi-node access**: ReadWriteMany and ReadOnlyMany access modes
- **Mount options**: Extensive configuration through mountpoint-s3 options
- **Caching**: Local disk caching for improved performance
- **Security**: Credential management and access controls
- **Regional flexibility**: Support for different S3 regions

## Examples and Tutorials

### Static Provisioning Examples
- [Basic Static Provisioning](static-provisioning/examples/basic-static-provisioning.md)
- [Multiple Buckets](static-provisioning/examples/multiple-buckets.md)
- [Secret Authentication](static-provisioning/examples/secret-authentication.md)
- [Advanced Local Caching](static-provisioning/examples/advanced-local-caching.md)

### Dynamic Provisioning Examples  
- [Basic Dynamic Provisioning](dynamic-provisioning/examples/basic-dynamic-provisioning.md)
- [Shared Bucket Provisioning](dynamic-provisioning/examples/shared-bucket-provisioning.md)
- [Advanced Dynamic Provisioning](dynamic-provisioning/examples/advanced-dynamic-provisioning.md)

## Migration Guide

### From Static to Dynamic
1. Create equivalent StorageClass configurations
2. Update application deployments to use dynamic PVCs
3. Migrate data if needed
4. Clean up static PVs and PVCs

### From Dynamic to Static
1. Identify existing buckets created by dynamic provisioning
2. Create manual PVs that reference these buckets
3. Update PVCs to reference the manual PVs
4. Remove StorageClass configurations

## Best Practices

### Choose Dynamic When:
- Building new cloud-native applications
- Need standardized storage configurations  
- Want automated lifecycle management
- Working in development/testing environments

### Choose Static When:
- Integrating with existing S3 infrastructure
- Need custom bucket configurations
- Require specific bucket naming schemes
- Working with legacy applications

### General Recommendations:
- Use labels and annotations for organization
- Configure appropriate mount options for your workload
- Set up monitoring and alerting for storage usage
- Plan for backup and disaster recovery scenarios

## Troubleshooting

Common issues and solutions are documented in each provisioning method's section:
- [Static Provisioning Troubleshooting](static-provisioning/overview.md#troubleshooting)
- [Dynamic Provisioning Troubleshooting](dynamic-provisioning/overview.md#troubleshooting)

## Next Steps

1. **Understand your requirements**: Determine whether static or dynamic provisioning fits your use case
2. **Review examples**: Study the relevant examples for your chosen approach
3. **Plan your implementation**: Design your StorageClass/PV configurations  
4. **Test thoroughly**: Validate configurations in a development environment
5. **Deploy incrementally**: Roll out storage configurations gradually in production