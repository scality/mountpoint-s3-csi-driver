# Static Provisioning Example

This example shows how to make a static provisioned Mountpoint for S3 persistent volume (PV) mounted inside container.

## Examples in this folder

- `static_provisioning.yaml` - spawning a pod which creates a file with name as the current date/time
- `non_root.yaml` - same as above, but the pod is spawned as non-root (uid `1000`, gid `2000`)
- `multiple_buckets_one_pod.yaml` - same as above, with multiple buckets mounted in one pod. Note: when mounting
  multiple buckets in the same pod, the `volumeHandle` must be unique as specified in the
  [CSI documentation](https://kubernetes.io/docs/concepts/storage/volumes/#csi).
- `multiple_pods_one_pv.yaml` - same as above, with multiple pods mounting the same persistent volume, in Deployment kind,
  that can be used to scale pods, using HPA or other method to create more replicas
- `caching.yaml` - shows how to configure mountpoint to use a cache directory. See the
  [Mountpoint documentation](https://github.com/awslabs/mountpoint-s3/blob/main/doc/CONFIGURATION.md#caching-configuration)
  for more details on caching options. Please thumbs up [#11](https://github.com/awslabs/mountpoint-s3-csi-driver/issues/141)
  or add details about your use case if you want improvements in this area.
- `kms_sse.yaml` - demonstrates using SSE-KMS encryption with a customer supplied key id. See the
  [Mountpoint documentation](https://github.com/awslabs/mountpoint-s3/blob/main/doc/CONFIGURATION.md#data-encryption)
  for more details.
- `aws_max_attempts.yaml` - configure the number of retries for requests to S3. This option is passed to Mountpoint
  as the `AWS_MAX_ATTEMPTS` environment variable. See the
  [Mountpoint configuration documentation](https://github.com/awslabs/mountpoint-s3/blob/main/doc/CONFIGURATION.md#other-s3-bucket-configuration)
  for more details.
- `secret_authentication.yaml` - demonstrates using a Kubernetes Secret to provide access credentials (access key and
  secret key) at Volume level for authenticating with S3. This is particularly useful when the user wants to set their
  own credentials which are different than the driver level credentials.

## Authentication

The CSI driver only supports static IAM credentials, which can be provided in two ways:

1. **Driver-level authentication** (default): Credentials are configured at the driver level and used for all PVs
   - Set using environment variables in the driver deployment
   - Configured via Helm values or directly in the deployment YAML

2. **Secret-level authentication**: Credentials are provided per-volume using Kubernetes Secrets
   - See `secret_authentication.yaml` for an example
   - Use `authenticationSource: secret` in volumeAttributes
   - Reference a secret with `nodePublishSecretRef`

## AWS Endpoint URL Configuration

For security and consistency reasons, if `--endpoint-url` is specified in the `mountOptions` of a PersistentVolume,
it will be **ignored** by the driver. This is enforced in both systemd and pod mounters to prevent potential security
risks like endpoint redirection attacks.

To configure a custom endpoint URL for S3 requests, you must set it at the driver level using one of the following methods:

### Using Helm

```yaml
# values.yaml for Helm chart
node:
  s3EndpointUrl: "https://s3.example.com:8000"
```

## Configure

### Edit [Persistent Volume](https://github.com/scality/mountpoint-s3-csi-driver/blob/main/examples/kubernetes/static_provisioning/static_provisioning.yaml)

!!! note
    This example assumes your S3 bucket has already been created. If you need to create a bucket, follow the
    [S3 documentation](https://docs.aws.amazon.com/AmazonS3/latest/userguide/creating-bucket.html).

- Bucket name (required): `PersistentVolume -> csi -> volumeAttributes -> bucketName`
- Bucket region (if bucket and cluster are in different regions): `PersistentVolume -> csi -> mountOptions`
- [Mountpoint configurations](https://github.com/awslabs/mountpoint-s3/blob/main/doc/CONFIGURATION.md) can be added
  in the `mountOptions` of the Persistent Volume spec.

See [Static Provisioning](https://github.com/scality/mountpoint-s3-csi-driver/blob/main/docs/CONFIGURATION.md#static-provisioning)
configuration page for more details.

## Deploy

```bash
kubectl apply -f examples/kubernetes/static_provisioning/static_provisioning.yaml
```

## Check the pod is running

```bash
kubectl get pod s3-app
```

## [Optional] Check fc-app created a file in s3

```bash
$ aws s3 ls <bucket_name>
> 2023-09-18 17:36:17         26 Mon Sep 18 17:36:14 UTC 2023.txt
```

## Cleanup

```bash
kubectl delete -f examples/kubernetes/static_provisioning/static_provisioning.yaml
```
