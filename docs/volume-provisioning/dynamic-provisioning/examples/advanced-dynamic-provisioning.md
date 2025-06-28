# Advanced Dynamic Provisioning

This example demonstrates advanced dynamic provisioning configurations including different regions, custom mount options, and various deployment scenarios.

## Features

- **Multi-region support**: StorageClasses for different AWS regions
- **Custom mount options**: Performance optimization with caching and custom settings
- **Deployment scenarios**: Different use cases from development to production
- **Performance tuning**: Optimized configurations for different workloads

## Multi-Region StorageClasses

```yaml
# Development environment - US East 1
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-dev
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-east-1
volumeBindingMode: Immediate
reclaimPolicy: Delete
mountOptions:
  - allow-delete
  - cache /tmp/s3-dev-cache
  - metadata-ttl 60  # Short TTL for development
---
# Production environment - US West 2
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-prod
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-west-2
volumeBindingMode: WaitForFirstConsumer  # Better pod placement
reclaimPolicy: Retain  # Preserve data in production
mountOptions:
  - allow-delete
  - cache /tmp/s3-prod-cache
  - metadata-ttl 300  # Longer TTL for performance
  - max-cache-size 1024  # 1GB cache
---
# Europe environment - EU West 1
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-eu
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: eu-west-1
volumeBindingMode: Immediate
reclaimPolicy: Delete
mountOptions:
  - allow-delete
  - cache /tmp/s3-eu-cache
  - metadata-ttl 180
```

## High-Performance Workload

```yaml
# StorageClass optimized for high-performance workloads
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-high-performance
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-west-2
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
mountOptions:
  - allow-delete
  - allow-overwrite
  - cache /tmp/s3-high-perf-cache
  - metadata-ttl 600  # 10 minutes
  - max-cache-size 2048  # 2GB cache
  - part-size 16  # 16MB parts for better performance
---
# PVC for high-performance workload
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: high-perf-storage
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi-high-performance
  resources:
    requests:
      storage: 500Gi
---
# Deployment using high-performance storage
apiVersion: apps/v1
kind: Deployment
metadata:
  name: data-processing-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: data-processing
  template:
    metadata:
      labels:
        app: data-processing
    spec:
      containers:
        - name: processor
          image: ubuntu
          command: ["/bin/sh"]
          args: ["-c", "while true; do date > /data/processing-$(hostname)-$(date +%s).log; sleep 10; done"]
          volumeMounts:
            - name: shared-storage
              mountPath: /data
          resources:
            requests:
              memory: "1Gi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "1000m"
      volumes:
        - name: shared-storage
          persistentVolumeClaim:
            claimName: high-perf-storage
```

## Database Backup Solution

```yaml
# StorageClass for database backups with retention
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-backup
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-east-1
volumeBindingMode: Immediate
reclaimPolicy: Retain  # Keep backups even if PVC is deleted
mountOptions:
  - allow-delete
  - allow-overwrite
  - cache /tmp/s3-backup-cache
  - metadata-ttl 3600  # 1 hour TTL for backup files
---
# PVC for database backups
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: database-backup-storage
  labels:
    purpose: backup
    retention: long-term
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi-backup
  resources:
    requests:
      storage: 1Ti  # Large storage for backups
---
# CronJob for automated database backups
apiVersion: batch/v1
kind: CronJob
metadata:
  name: database-backup-job
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: backup
              image: postgres:13
              command: ["/bin/sh"]
              args:
                - -c
                - |
                  BACKUP_FILE="/backup/postgres-backup-$(date +%Y%m%d-%H%M%S).sql"
                  echo "Creating backup: $BACKUP_FILE"
                  pg_dump -h database-service -U postgres -d myapp > "$BACKUP_FILE"
                  echo "Backup completed: $BACKUP_FILE"
                  ls -la /backup/
              env:
                - name: PGPASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: postgres-secret
                      key: password
              volumeMounts:
                - name: backup-storage
                  mountPath: /backup
          restartPolicy: OnFailure
          volumes:
            - name: backup-storage
              persistentVolumeClaim:
                claimName: database-backup-storage
```

## Development vs Production Configuration

```yaml
# Development StorageClass - fast iteration, no data persistence
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-dev-fast
  labels:
    environment: development
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-east-1
volumeBindingMode: Immediate
reclaimPolicy: Delete  # Clean up automatically
allowVolumeExpansion: true
mountOptions:
  - allow-delete
  - allow-overwrite
  - metadata-ttl 30  # Fast updates for development
  - cache /tmp/s3-dev-cache
---
# Production StorageClass - data safety, performance
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-production
  labels:
    environment: production
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-west-2
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Retain  # Preserve data
allowVolumeExpansion: true
mountOptions:
  - allow-delete
  - cache /tmp/s3-prod-cache
  - metadata-ttl 300
  - max-cache-size 1024
```

## Deployment with Node Affinity

```yaml
# PVC with specific requirements
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: geo-distributed-storage
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi-prod
  resources:
    requests:
      storage: 200Gi
---
# Deployment with node affinity for optimal performance
apiVersion: apps/v1
kind: Deployment
metadata:
  name: geo-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: geo-app
  template:
    metadata:
      labels:
        app: geo-app
    spec:
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              preference:
                matchExpressions:
                  - key: topology.kubernetes.io/zone
                    operator: In
                    values: ["us-west-2a", "us-west-2b"]  # Prefer same region as S3
      containers:
        - name: app
          image: nginx
          volumeMounts:
            - name: content
              mountPath: /usr/share/nginx/html
          ports:
            - containerPort: 80
      volumes:
        - name: content
          persistentVolumeClaim:
            claimName: geo-distributed-storage
```

## Monitoring and Observability

```yaml
# StorageClass with additional labels for monitoring
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-monitored
  labels:
    monitoring: enabled
    cost-center: engineering
    environment: production
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-west-2
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
mountOptions:
  - allow-delete
  - cache /tmp/s3-monitored-cache
  - metadata-ttl 300
---
# PVC with monitoring labels
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: monitored-app-storage
  labels:
    app: web-application
    tier: frontend
    monitoring: enabled
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: s3-csi-monitored
  resources:
    requests:
      storage: 100Gi
```

## Best Practices Configuration

```yaml
# Production-ready StorageClass following best practices
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: s3-csi-best-practices
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
  labels:
    purpose: production
    backup-required: "true"
provisioner: s3.csi.scality.com
parameters:
  bucketNaming: dedicated
  s3Region: us-west-2
volumeBindingMode: WaitForFirstConsumer  # Better pod placement
reclaimPolicy: Retain  # Preserve data
allowVolumeExpansion: true  # Allow future growth
mountOptions:
  - allow-delete
  - cache /var/cache/s3-csi  # Persistent cache location
  - metadata-ttl 300
  - max-cache-size 512
  - uid=1000  # Non-root user
  - gid=1000
  - allow-other
```

## Cleanup Scripts

```bash
#!/bin/bash
# cleanup-dev-resources.sh
# Script to clean up development resources

echo "Cleaning up development S3 CSI resources..."

# Delete development PVCs
kubectl delete pvc -l environment=development

# Delete development StorageClasses
kubectl delete storageclass -l environment=development

# Wait for cleanup
sleep 30

echo "Development cleanup completed!"
```

## Notes

- **Volume Binding Modes**: Use `WaitForFirstConsumer` for production to ensure optimal pod placement
- **Reclaim Policies**: Use `Retain` for production data, `Delete` for development
- **Cache Configuration**: Adjust cache size and TTL based on workload patterns  
- **Regional Placement**: Choose regions close to your compute resources for better performance
- **Security**: Use appropriate UID/GID settings for non-root containers
- **Monitoring**: Add labels and annotations for cost tracking and monitoring