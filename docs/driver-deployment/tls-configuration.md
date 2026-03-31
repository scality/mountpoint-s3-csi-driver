# TLS Configuration

## Overview

When your S3 endpoint uses TLS with certificates signed by a private or internal CA,
the CSI driver needs access to the CA certificate to validate the connection.
The Scality CSI Driver supports injecting custom CA certificates via Kubernetes ConfigMaps.

This is required when:

- Your RING S3 endpoint uses HTTPS with a self-signed or internally-signed certificate
- Your organization uses a private CA for internal services
- The S3 endpoint's certificate chain is not in the default system trust store

## Prerequisites

- A PEM-encoded CA certificate file (the root or intermediate CA that signed your S3 server certificate)
- The CSI driver Helm chart installed or ready to install

## Configuration (Helm-Managed)

The recommended approach uses `--set-file` to pass the CA certificate content directly to Helm.
Helm creates the ConfigMap in both required namespaces automatically.

### Step 1: Install or Upgrade with CA Certificate Data

```bash
helm upgrade --install scality-s3-csi \
  ./charts/scality-mountpoint-s3-csi-driver \
  --namespace kube-system \
  --set s3.endpointUrl=https://s3.example.com:443 \
  --set tls.caCertConfigMap=s3-ca-cert \
  --set-file tls.caCertData=/path/to/your/ca.crt
```

This single command:

- Creates a ConfigMap named `s3-ca-cert` in the controller namespace (`kube-system`)
- Creates the same ConfigMap in the mounter pod namespace (`mount-s3`)
- Configures the controller and mounter pods to use the CA certificate

!!! important "Key Name"
    The ConfigMap key is automatically set to `ca-bundle.crt`, which is the key the driver expects.

### Step 2: Verify

Check that the controller pod has the CA certificate mounted:

```bash
kubectl exec -n kube-system deploy/s3-csi-controller \
  -c s3-csi-controller -- ls /etc/ssl/custom-ca/
```

Expected output: `ca-bundle.crt`

Verify the ConfigMap exists in the mounter pod namespace:

```bash
kubectl get configmap s3-ca-cert -n mount-s3
```

### Certificate Rotation

To rotate the CA certificate, update the Helm release with the new certificate file:

```bash
helm upgrade scality-s3-csi \
  ./charts/scality-mountpoint-s3-csi-driver \
  --namespace kube-system \
  --reuse-values \
  --set-file tls.caCertData=/path/to/new/ca.crt
```

Helm updates the ConfigMap in both namespaces. Existing pods will pick up the change
on their next restart.

## Manual Mode

If you cannot pass the certificate data via Helm values (e.g., policy restrictions),
you can create the ConfigMaps manually. In this mode, set only `tls.caCertConfigMap`
without `tls.caCertData`.

!!! info "Why Two Namespaces?"
    The CA certificate ConfigMap must exist in **two** namespaces because the controller and
    mounter pods run in separate namespaces:

    1. **Controller namespace** (e.g., `kube-system`) — mounted by the `s3-csi-controller` for
       AWS SDK S3 API calls (bucket creation/deletion during dynamic provisioning).
    2. **Mounter pod namespace** (e.g., `mount-s3`) — mounted by mounter pod init containers
       that inject the CA into the `mount-s3` trust store.

### Step 1: Create the CA Certificate ConfigMap in the Controller Namespace

```bash
kubectl create configmap s3-ca-cert \
  --from-file=ca-bundle.crt=/path/to/your/ca.crt \
  -n kube-system
```

!!! important "Key Name"
    The ConfigMap key **must** be `ca-bundle.crt`. This is the key the driver expects.

### Step 2: Install or Upgrade the Helm Chart

```bash
helm upgrade --install scality-s3-csi \
  ./charts/scality-mountpoint-s3-csi-driver \
  --namespace kube-system \
  --set s3.endpointUrl=https://s3.example.com:443 \
  --set tls.caCertConfigMap=s3-ca-cert
```

### Step 3: Create the CA Certificate ConfigMap in the Mounter Namespace

After Helm creates the `mount-s3` namespace, create the same ConfigMap there:

```bash
kubectl create configmap s3-ca-cert \
  --from-file=ca-bundle.crt=/path/to/your/ca.crt \
  -n mount-s3
```

!!! warning "Namespace Ordering"
    Do **not** attempt to create the ConfigMap in the `mount-s3` namespace before the Helm install —
    the namespace does not exist yet. If a ConfigMap is missing from either namespace, the
    respective pod will be stuck in `ContainerCreating` with a `configmap not found` event.

### Switching from Manual to Helm-Managed Mode

If you previously created ConfigMaps manually and want to switch to Helm-managed mode,
delete the manually created ConfigMaps first — Helm cannot adopt resources it did not create:

```bash
kubectl delete configmap s3-ca-cert -n kube-system
kubectl delete configmap s3-ca-cert -n mount-s3
helm upgrade scality-s3-csi \
  ./charts/scality-mountpoint-s3-csi-driver \
  --namespace kube-system \
  --reuse-values \
  --set tls.caCertConfigMap=s3-ca-cert \
  --set-file tls.caCertData=/path/to/your/ca.crt
```

## How It Works

The TLS configuration operates at two levels:

### Controller Pod (Dynamic Provisioning)

The controller pod uses the CA certificate for S3 API calls (bucket creation/deletion)
during dynamic provisioning:

- The ConfigMap is mounted at `/etc/ssl/custom-ca/` in the `s3-csi-controller` container
- The `AWS_CA_BUNDLE` environment variable is set to `/etc/ssl/custom-ca/ca-bundle.crt`
- AWS SDK Go v2 reads this variable and uses the CA certificate for TLS validation

### Mounter Pods (Volume Mounting)

Mounter pods use `mount-s3` (which uses s2n-tls) to mount S3 buckets.
s2n-tls reads CA certificates from the system trust store (`/etc/ssl/certs/`),
so a simple volume mount is not sufficient. Instead:

1. An **initContainer** (`install-ca-cert`) runs before the main `mountpoint` container
2. The initContainer copies the system CA bundle from the Alpine image to a shared emptyDir volume
3. It appends the custom CA certificate from the ConfigMap to the combined bundle
4. The main container mounts the shared volume at `/etc/ssl/certs/` (read-only)
5. `mount-s3` reads the combined trust store and validates the S3 endpoint certificate

The initContainer runs as non-root and complies with the PodSecurity `restricted` policy
enforced on the mounter pod namespace.

## Helm Values Reference

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `tls.caCertConfigMap` | Name of the ConfigMap containing the CA certificate | `""` (disabled) |
| `tls.caCertData` | PEM-encoded CA certificate content (enables Helm-managed mode) | `""` |
| `tls.initImage.repository` | Image repository for the CA cert init container | `alpine` |
| `tls.initImage.tag` | Image tag for the CA cert init container | `3.21` |
| `tls.initImage.pullPolicy` | Pull policy for the init image | `IfNotPresent` |
| `tls.initResources.requests.cpu` | CPU request for the init container | `10m` |
| `tls.initResources.requests.memory` | Memory request for the init container | `16Mi` |
| `tls.initResources.limits.memory` | Memory limit for the init container | `64Mi` |

## Why ConfigMap Instead of Secret

CA certificates are public configuration data, not confidential information.
Using ConfigMaps instead of Secrets:

- Follows the Kubernetes convention of using ConfigMaps for non-sensitive configuration
- Avoids unnecessary RBAC complexity for managing Secrets
- Makes the certificates easier to inspect and manage

## Troubleshooting

### Pod Stuck in ContainerCreating

If a controller or mounter pod is stuck in `ContainerCreating` after enabling TLS, the CA
certificate ConfigMap is likely missing from that pod's namespace. Check the pod events:

```bash
kubectl describe pod <pod-name> -n <namespace>
```

Look for an event like: `configmap "s3-ca-cert" not found`.

To fix, either switch to Helm-managed mode (`--set-file tls.caCertData=...`) or create the
ConfigMap manually in the correct namespace:

```bash
# For controller pods (controller namespace, default: kube-system)
kubectl create configmap s3-ca-cert \
  --from-file=ca-bundle.crt=/path/to/your/ca.crt \
  -n kube-system

# For mounter pods (mounter pod namespace, default: mount-s3)
kubectl create configmap s3-ca-cert \
  --from-file=ca-bundle.crt=/path/to/your/ca.crt \
  -n mount-s3
```

### Certificate Not Found

If mounter pods fail with TLS errors, verify the ConfigMap exists in **both** namespaces:

1. Controller namespace (default: `kube-system`):

    ```bash
    kubectl get configmap s3-ca-cert -n kube-system
    ```

2. Mounter pod namespace (default: `mount-s3`):

    ```bash
    kubectl get configmap s3-ca-cert -n mount-s3
    ```

3. The ConfigMap has the correct key:

    ```bash
    kubectl get configmap s3-ca-cert -n mount-s3 -o jsonpath='{.data}' | head -c 100
    ```

### Certificate Chain Issues

If you see certificate verification errors despite having the CA cert configured:

- Ensure you are providing the **root CA** certificate, not the server certificate
- If using an intermediate CA, include the full chain in the `ca-bundle.crt` file
- Verify the certificate is in PEM format (starts with `-----BEGIN CERTIFICATE-----`)

### Init Container Failures

If the init container fails, check its logs:

```bash
kubectl logs <mounter-pod-name> -n mount-s3 -c install-ca-cert
```

Common issues:

- The init image must include a system CA bundle at `/etc/ssl/certs/ca-certificates.crt`
  (Alpine includes this by default via the `ca-certificates` package)
- The ConfigMap may not be mounted correctly
