# Uninstallation

To uninstall the driver:

```bash
helm uninstall scality-s3-csi --namespace kube-system
```

Delete the S3 credentials secret:

```bash
kubectl delete secret s3-secret --namespace kube-system
```
