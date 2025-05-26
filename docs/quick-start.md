# Quick Start Guide

This guide will walk you through a minimal installation of the Scality S3 CSI Driver and how to verify it.

## Prerequisites

- A Kubernetes cluster (v1.30.0+).
- Helm installed.
- `kubectl` configured to communicate with your cluster.
- An S3-compatible endpoint URL. This is a **required** parameter.
- Access to S3 credentials: an Access Key ID and Secret Access Key (and optionally a Session Token).

> **⚠️ Security Warning**: This guide demonstrates basic credential handling for testing purposes. Be aware of the following security considerations:
>
> - Environment variables expose credentials in shell history and process lists
> - Commands with credentials are visible to other users via `ps` commands
> - The `kube-system` namespace has elevated privileges
>
> **For Production Use**:
>
> - Create secrets from files instead of command line arguments
> - Use a dedicated namespace with appropriate RBAC policies
> - Consider using IAM roles or service accounts for credential management
> - Always clean up credentials after testing

## Installation

### Step 1: Add the Scality Helm repository

```bash
helm repo add scality https://scality.github.io/mountpoint-s3-csi-driver/charts/
```

```bash
helm repo update
```

### Step 2: Set your configuration variables

Replace these values with your actual S3 configuration:

```bash
# Required: Your S3-compatible endpoint URL
export S3_ENDPOINT_URL="http://s3.example.com:8000"

# Required: Your S3 credentials
export AWS_ACCESS_KEY_ID="YOUR_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="YOUR_SECRET_ACCESS_KEY"
# export AWS_SESSION_TOKEN="YOUR_SESSION_TOKEN"  # Optional, uncomment if needed

# Required: Your S3 bucket name for testing
export S3_BUCKET_NAME="my-test-bucket"

# Optional: Customize these if needed
export S3_REGION="us-east-1"
export SECRET_NAME="aws-secret"
export NAMESPACE="scality-s3-csi"
```

### Step 3: Create namespace (recommended)

Create a dedicated namespace for better security isolation:

```bash
kubectl create namespace ${NAMESPACE}
```

### Step 4: Create S3 credentials secret

Method 1: From environment variables (quick but less secure)

```bash
kubectl create secret generic ${SECRET_NAME} \
  --from-literal=key_id="${AWS_ACCESS_KEY_ID}" \
  --from-literal=access_key="${AWS_SECRET_ACCESS_KEY}" \
  --namespace=${NAMESPACE}
```

If you need a session token, add it:

```bash
kubectl patch secret ${SECRET_NAME} -n ${NAMESPACE} --type='json' -p='[{"op": "add", "path": "/data/session_token", "value": "'$(echo -n "${AWS_SESSION_TOKEN}" | base64)'"}]'
```

Method 2: From files (more secure alternative)

Create credential files locally (these won't appear in shell history):

```bash
echo -n "${AWS_ACCESS_KEY_ID}" > /tmp/key_id
echo -n "${AWS_SECRET_ACCESS_KEY}" > /tmp/access_key
```

Create secret from files:

```bash
kubectl create secret generic ${SECRET_NAME} \
  --from-file=key_id=/tmp/key_id \
  --from-file=access_key=/tmp/access_key \
  --namespace=${NAMESPACE}
```

Clean up temporary files:

```bash
rm /tmp/key_id /tmp/access_key
```

### Step 5: Install the CSI driver

```bash
helm install mountpoint-s3-csi-driver scality/scality-mountpoint-s3-csi-driver \
  --set node.s3EndpointUrl="${S3_ENDPOINT_URL}" \
  --set node.s3Region="${S3_REGION}" \
  --set awsAccessSecret.name="${SECRET_NAME}" \
  --namespace ${NAMESPACE}
```

### Step 6: Verify installation

Check if the CSI driver pods are running:

```bash
kubectl get pods -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver
```

Check CSI driver registration:

```bash
kubectl get csidriver s3.csi.scality.com
```

## Create and Test a Volume

### Step 1: Create PV and PVC

```bash
cat <<EOF | kubectl apply -f -
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: s3-pv-test
spec:
  accessModes:
    - ReadWriteMany
  capacity:
    storage: 1Gi
  csi:
    driver: s3.csi.scality.com
    volumeHandle: ${S3_BUCKET_NAME}
    volumeAttributes:
      region: "${S3_REGION}"
  storageClassName: ""
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: s3-pvc-test
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
  volumeName: s3-pv-test
  storageClassName: ""
EOF
```

### Step 2: Create test pod

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: s3-test-pod
spec:
  containers:
    - name: s3-test-container
      image: busybox:1.36
      command: ["/bin/sh", "-c", "sleep 3600"]
      volumeMounts:
        - name: s3-volume
          mountPath: /data
  volumes:
    - name: s3-volume
      persistentVolumeClaim:
        claimName: s3-pvc-test
EOF
```

### Step 3: Wait for pod to be ready

```bash
kubectl wait --for=condition=Ready pod/s3-test-pod --timeout=60s
```

## Verification

### Check the mount

Verify the pod is running:

```bash
kubectl get pod s3-test-pod
```

Check if S3 bucket is mounted:

```bash
kubectl exec s3-test-pod -- df -h /data
```

List contents of the bucket:

```bash
kubectl exec s3-test-pod -- ls -la /data
```

### Test read/write operations

Write a test file:

```bash
kubectl exec s3-test-pod -- sh -c "echo 'Hello from Scality S3 CSI Driver!' > /data/test-file.txt"
```

Read the file back:

```bash
kubectl exec s3-test-pod -- cat /data/test-file.txt
```

Verify the file exists and check its details:

```bash
kubectl exec s3-test-pod -- ls -la /data/test-file.txt
```

## Troubleshooting

### Check logs if something goes wrong

Check CSI driver logs:

```bash
kubectl logs -n ${NAMESPACE} -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -c s3-csi-driver
```

Check pod events:

```bash
kubectl describe pod s3-test-pod
```

Check PVC status:

```bash
kubectl describe pvc s3-pvc-test
```

### Check credentials and namespace

Verify the namespace exists:

```bash
kubectl get namespace ${NAMESPACE}
```

Verify the secret exists and has the correct keys:

```bash
kubectl get secret ${SECRET_NAME} -n ${NAMESPACE} -o yaml
```

Check if CSI driver has proper access to the secret:

```bash
kubectl get csidriver s3.csi.scality.com -o yaml
```

## Cleanup

### Step 1: Delete test resources

```bash
kubectl delete pod s3-test-pod
```

```bash
kubectl delete pvc s3-pvc-test
```

```bash
kubectl delete pv s3-pv-test
```

### Step 2: Clean up CSI driver and credentials

Uninstall the CSI driver:

```bash
helm uninstall mountpoint-s3-csi-driver --namespace ${NAMESPACE}
```

Delete the credentials secret:

```bash
kubectl delete secret ${SECRET_NAME} --namespace ${NAMESPACE}
```

If you created a dedicated namespace, delete it:

```bash
kubectl delete namespace ${NAMESPACE}
```

### Step 3: Clean up environment variables (security best practice)

Clear sensitive environment variables from your shell session:

```bash
unset AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN
```

Clear shell history (bash):

```bash
history -c && history -w
```

Clear shell history (zsh):

```bash
fc -p
```

## Advanced Configuration

### Using experimental pod mounter

For improved performance in certain scenarios, you can enable the experimental pod mounter:

```bash
helm install mountpoint-s3-csi-driver scality/scality-mountpoint-s3-csi-driver \
  --set node.s3EndpointUrl="${S3_ENDPOINT_URL}" \
  --set node.s3Region="${S3_REGION}" \
  --set awsAccessSecret.name="${SECRET_NAME}" \
  --set experimental.podMounter=true \
  --namespace ${NAMESPACE}
```

This quick start provides a basic overview. For more advanced configurations and features, please refer to the full documentation.
