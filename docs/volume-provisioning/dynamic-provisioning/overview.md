# Dynamic Provisioning

!!! note "Automatic Bucket Creation"
    With dynamic provisioning, S3 buckets are automatically created and deleted by the CSI driver based on StorageClass parameters.

Dynamic provisioning allows the Scality S3 CSI driver to automatically create S3 buckets or bucket prefixes when a PersistentVolumeClaim (PVC) is created,
eliminating the need to manually pre-create buckets.

## Key Features

- **Automatic bucket creation**: Buckets are created automatically when PVCs are created
- **Automatic cleanup**: Buckets and their contents are deleted when PVCs are deleted (with Delete reclaim policy)
- **Two provisioning modes**: Dedicated buckets per volume or shared buckets with unique prefixes
- **StorageClass-based configuration**: All provisioning behavior is configured through StorageClass parameters

## StorageClass Parameters

Dynamic provisioning is configured through StorageClass parameters:

| Parameter | Description | Default | Required | Valid Values |
|-----------|-------------|---------|----------|--------------|
| `bucketNaming` | Bucket creation strategy | `dedicated` | No | `dedicated`, `shared` |
| `s3Region` | AWS S3 region for bucket creation | `us-east-1` | No | Valid AWS region |
| `bucketPrefix` | Shared bucket name (for shared mode) | - | Yes (for shared mode) | Valid S3 bucket name |
| `mountOptions` | Default mount options for volumes | - | No | Valid mountpoint-s3 options |

## Bucket Naming Strategies

### Dedicated Buckets (`bucketNaming: dedicated`)

- **Default strategy**: Each PVC gets its own dedicated S3 bucket
- **Bucket naming**: `s3-csi-{volume-id}` (sanitized and truncated if needed)
- **Isolation**: Complete isolation between volumes
- **Use case**: When you need complete isolation or have permissions to create many buckets

### Shared Buckets (`bucketNaming: shared`)

- **Prefix-based**: Multiple PVCs share a single pre-existing bucket with unique prefixes
- **Prefix naming**: `volumes/{volume-id}/`
- **Bucket requirement**: The shared bucket must be created manually before use
- **Use case**: When bucket creation permissions are limited or you want to consolidate storage

## Volume Lifecycle

### Volume Creation
1. PVC is created with a StorageClass that uses the S3 CSI driver
2. Driver reads StorageClass parameters
3. **Dedicated mode**: Creates a new S3 bucket with sanitized name
4. **Shared mode**: Validates that the shared bucket exists and generates a unique prefix
5. PV is created with appropriate volume attributes
6. Volume is ready for mounting

### Volume Deletion
1. PVC is deleted
2. **Dedicated mode**: All objects in the bucket are deleted, then the bucket itself is deleted
3. **Shared mode**: Objects under the volume's prefix are cleaned up (bucket remains)
4. PV is deleted

## Security Considerations

- **IAM Permissions**: The CSI driver needs appropriate S3 permissions for bucket operations
- **Bucket Naming**: Only buckets with the `s3-csi-` prefix are managed by the driver
- **Safety Checks**: The driver validates bucket ownership before deletion operations

## Required IAM Permissions

For dynamic provisioning, the CSI driver requires these additional S3 permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:CreateBucket",
                "s3:DeleteBucket",
                "s3:GetBucketLocation",
                "s3:HeadBucket",
                "s3:ListBucket",
                "s3:DeleteObject",
                "s3:ListObjects",
                "s3:ListObjectsV2"
            ],
            "Resource": [
                "arn:aws:s3:::s3-csi-*",
                "arn:aws:s3:::s3-csi-*/*"
            ]
        }
    ]
}
```

## Comparison with Static Provisioning

| Aspect | Static Provisioning | Dynamic Provisioning |
|--------|-------------------|---------------------|
| **Bucket Creation** | Manual | Automatic |
| **StorageClass** | Empty string (`""`) | CSI driver name |
| **Bucket Management** | External | CSI driver |
| **Flexibility** | High (any bucket) | Structured (CSI-managed) |
| **Setup Complexity** | Higher | Lower |
| **IAM Requirements** | Read/Write only | + Bucket management |

## Getting Started

1. **Configure StorageClass**: Create a StorageClass with desired parameters
2. **Create PVC**: Reference the StorageClass in your PVC
3. **Use in Pods**: Mount the PVC in your application pods

See the [examples section](examples/) for detailed YAML configurations.

## Troubleshooting

### Common Issues

#### PVC stays in Pending state
- Check StorageClass parameters are valid
- Verify IAM permissions for bucket creation
- Check driver logs for S3 connection errors

#### Bucket creation fails
- Ensure S3 region is accessible
- Verify AWS credentials have bucket creation permissions
- Check for bucket naming conflicts

#### Volume mounting fails
- Verify bucket was created successfully
- Check mount options compatibility
- Review pod security context settings

#### For more troubleshooting help
- Check driver logs: `kubectl logs -n kube-system -l app=s3-csi-driver`
- Review S3 service status and permissions
- Validate StorageClass parameter syntax
