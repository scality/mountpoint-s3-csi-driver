# Installation Guide

This guide covers the steps to install the Scality S3 CSI Driver in a Kubernetes cluster.

## Prerequisites

Before installing the driver, ensure the environment meets all the requirements outlined in the **[Prerequisites](prerequisites.md)** guide.

## Installation with Helm

The recommended method for installing the Scality S3 CSI Driver is using Helm.

1. **Add Scality Helm Repository**:
   If not already added, add the Scality Helm repository:

   ```bash
   helm repo add scality https://scality.github.io/mountpoint-s3-csi-driver
   helm repo update
   ```

2. **Create S3 Credentials Secret**:
   Before installing the driver, create a Kubernetes Secret containing the S3 credentials:

   ```bash
   # Set S3 configuration
   export S3_ENDPOINT_URL="https://your-s3-endpoint.example.com"
   export AWS_ACCESS_KEY_ID="YOUR_ACCESS_KEY_ID"
   export AWS_SECRET_ACCESS_KEY="YOUR_SECRET_ACCESS_KEY"
   # export AWS_SESSION_TOKEN="YOUR_SESSION_TOKEN"  # Optional, uncomment if needed
   export SECRET_NAME="s3-credentials"
   ```

   Create the secret:

   ```bash
   kubectl create secret generic ${SECRET_NAME} \
     --from-literal=access_key_id="${AWS_ACCESS_KEY_ID}" \
     --from-literal=secret_access_key="${AWS_SECRET_ACCESS_KEY}" \
     --namespace kube-system
   ```

   If a session token is needed, add it:

   ```bash
   kubectl patch secret ${SECRET_NAME} -n kube-system --type='json' -p='[{"op": "add", "path": "/data/session_token", "value": "'$(echo -n "${AWS_SESSION_TOKEN}" | base64)'"}]'
   ```

   !!! tip "More Secure Alternative"
       For better security, create credential files locally (these won't appear in shell history):
       ```bash
       echo -n "${AWS_ACCESS_KEY_ID}" > /tmp/access_key_id
       echo -n "${AWS_SECRET_ACCESS_KEY}" > /tmp/secret_access_key
       kubectl create secret generic ${SECRET_NAME} \
         --from-file=access_key_id=/tmp/access_key_id \
         --from-file=secret_access_key=/tmp/secret_access_key \
         --namespace kube-system
       rm /tmp/access_key_id /tmp/secret_access_key  # Clean up
       ```

3. **Create a Custom Values File**:
   Create a `values.yaml` file that references the secret created above:

   ```yaml
   # my-scality-s3-values.yaml
   node:
     # REQUIRED: Specify the Scality S3 endpoint URL
     s3EndpointUrl: "https://your-s3-endpoint.example.com"

     # Optional: Specify the default AWS region for S3 requests
     # This can be overridden per-volume via PV mountOptions.
     s3Region: "us-east-1"

     # Optional: Customize the path where kubelet stores its data.
     # Default is /var/lib/kubelet. Change if the cluster uses a different path.
     # kubeletPath: /var/lib/kubelet

   s3CredentialSecret:
     # Reference the secret created above
     name: "s3-credentials"  # Must match the secret name created above
   ```

   - Replace `https://your-s3-endpoint.example.com` with the actual S3 endpoint.
   - Ensure the `s3CredentialSecret.name` matches the secret name created above.
   - Review other options in the default [chart values](https://github.com/scality/mountpoint-s3-csi-driver/blob/main/charts/scality-mountpoint-s3-csi-driver/values.yaml) and customize as needed.

   !!! important "S3 Endpoint URL is Required"
       The `node.s3EndpointUrl`  and `s3CredentialSecret.name` parameter is **mandatory**. The Helm installation will fail if it's not provided.

4. **Install the Helm Chart**:
   Deploy the driver using Helm, by default into the `kube-system` namespace. If installing the driver in a different namespace, the credentials secret should be created in that namespace.

   ```bash
   helm install scality-s3-csi scality/mountpoint-s3-csi-driver \
     -f my-scality-s3-values.yaml \
     --namespace kube-system
   ```

5. **Verify the Installation**:
   Check that the driver pods are running correctly:

   ```bash
   kubectl get pods -n kube-system -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
   ```

   There should be one `s3-csi-node-*` pod per eligible worker node in the cluster, and they should all be in the `Running` state.

   Verify the CSIDriver object is created:

   ```bash
   kubectl get csidriver s3.csi.scality.com
   ```


## Security Considerations

<!-- markdownlint-disable MD046 -->
!!! warning "Security Best Practices"
    For production deployments, follow these security guidelines:

    - **Manual Secret Creation**: Always create S3 credential secrets manually before installing the chart. Do not store credentials in Helm values or use in-line secrets.
    - **Credential Management**: Create secrets from files instead of command line arguments to avoid exposing credentials in shell history and process lists.
    - **Namespace Isolation**: Use dedicated namespaces with appropriate RBAC policies instead of the default `kube-system` namespace.
    - **Cleanup**: Always clean up temporary credential files after creating secrets.
<!-- markdownlint-enable MD046 -->
