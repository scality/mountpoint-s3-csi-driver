# Helm Chart Configuration Reference

The Scality S3 CSI Driver is configured primarily through the [`values.yaml`](https://github.com/scality/mountpoint-s3-csi-driver/blob/main/charts/scality-mountpoint-s3-csi-driver/values.yaml)
file when deploying via Helm.
These parameters configure the overall behavior of the CSI driver components.

## Global Helm Configuration

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `nameOverride`                                       | Override the chart name.                                                                                                                           | `""`                                                   | No                          |
| `fullnameOverride`                                   | Override the full name of the release.                                                                                                             | `""`                                                   | No                          |
| `imagePullSecrets`                                   | Secrets for pulling images from private registries.                                                                                                | `[]`                                                   | No                          |

## Container Image Configuration

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `image.repository`                                   | The container image repository for the CSI driver.                                                                                                 | `ghcr.io/scality/mountpoint-s3-csi-driver`             | No                          |
| `image.pullPolicy`                                   | The image pull policy.                                                                                                                             | `IfNotPresent`                                         | No                          |
| `image.tag`                                          | The image tag for the CSI driver. Overrides the chart's `appVersion` if set.                                                                       | `1.0.0`                                                | No                          |

## S3 Credentials Secret Configuration

<!-- markdownlint-disable MD046 -->
!!! important "Security Note"
    The Helm chart **does not create secrets automatically**. A Kubernetes Secret containing S3 credentials must be created before installing the chart. The secret must contain the following keys:

    - `access_key_id`: S3 Access Key ID.
    - `secret_access_key`: S3 Secret Access Key.
    - `session_token` (optional): S3 Session Token, if using temporary credentials.
<!-- markdownlint-enable MD046 -->

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `s3CredentialSecret.name`                            | Name of the Kubernetes Secret containing AWS credentials (`access_key_id`, `secret_access_key`, optionally `session_token`). The secret must be created manually. | `s3-secret`                                           | No                          |
| `s3CredentialSecret.accessKeyId`                     | Key within the secret for Access Key ID.                                                                                                           | `access_key_id`                                        | No                          |
| `s3CredentialSecret.secretAccessKey`                 | Key within the secret for Secret Access Key.                                                                                                       | `secret_access_key`                                    | No                          |
| `s3CredentialSecret.sessionToken`                    | Key within the secret for Session Token (optional).                                                                                                | `session_token`                                        | No                          |

## Node Plugin Configuration

<!-- markdownlint-disable MD046 -->
!!! note "SELinux Context Note"
    The `node.seLinuxOptions.*` parameters define the SELinux security context for the CSI driver containers.
    These settings are applied to CSI Node DaemonSet containers and allow the containers to interact with systemd and manage mount points in SELinux-enforced environments.
    **Only the default SELinux values are tested and supported. Custom SELinux configurations are not supported.** The default values are:

    - `user`: `system_u`
    - `type`: `super_t`
    - `role`: `system_r`
    - `level`: `s0`
<!-- markdownlint-enable MD046 -->

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `node.kubeletPath`                                   | The path to the kubelet directory on the host node. Used by the node plugin to register itself and manage mount points.                               | `/var/lib/kubelet`                                     | No                          |
| `node.logLevel`                                      | Log verbosity level for the CSI driver (higher numbers = more verbose). 1-2: Basic operational info (recommended for production), 3: Credential authentication info, 4: All CSI operations and mount details (default), 5: Very detailed debug info. | `4`                                                    | No                          |
| `node.s3EndpointUrl`                                 | The RING S3 endpoint URL to be used by the driver for all mount operations.                                                                             | `"http://s3.example.com:8000"`                        | **Yes**                     |
| `node.s3Region`                                      | The default AWS region to use for S3 requests. Can be overridden per-volume via PV `mountOptions`.                                               | `us-east-1`                                            | No                          |
| `node.mountpointInstallPath`                         | Path on the host where the `mount-s3` binary will be installed by the initContainer. Should end with a `/`. *Only used with SystemD mounter (default).* | `/opt/mountpoint-s3-csi/bin/`                          | No                          |
| `node.seLinuxOptions.user`                           | SELinux user for the CSI driver container security context.                                                                                        | `system_u`                                             | No                          |
| `node.seLinuxOptions.type`                           | SELinux type for the CSI driver container security context.                                                                                        | `super_t`                                              | No                          |
| `node.seLinuxOptions.role`                           | SELinux role for the CSI driver container security context.                                                                                        | `system_r`                                             | No                          |
| `node.seLinuxOptions.level`                          | SELinux level for the CSI driver container security context.                                                                                       | `s0`                                                   | No                          |
| `node.serviceAccount.create`                         | Specifies whether a ServiceAccount should be created for the node plugin.                                                                          | `true`                                                 | No                          |
| `node.serviceAccount.name`                           | Name of the ServiceAccount to use for the node plugin.                                                                                             | `s3-csi-driver-sa`                                     | No                          |
| `node.nodeSelector`                                  | Node selector for scheduling the node plugin DaemonSet.                                                                                            | `{}`                                                   | No                          |
| `node.resources.requests.cpu`                        | CPU resource requests for the node plugin container.                                                                                               | `10m`                                                  | No                          |
| `node.resources.requests.memory`                     | Memory resource requests for the node plugin container.                                                                                            | `40Mi`                                                 | No                          |
| `node.resources.limits.memory`                       | Memory resource limits for the node plugin container.                                                                                              | `256Mi`                                                | No                          |
| `node.tolerateAllTaints`                             | If true, the node plugin DaemonSet will tolerate all taints. Overrides `defaultTolerations`.                                                      | `false`                                                | No                          |
| `node.defaultTolerations`                            | If true, adds default tolerations (`CriticalAddonsOnly`, `NoExecute` for 300s) to the node plugin.                                                 | `true`                                                 | No                          |
| `node.tolerations`                                   | Custom tolerations for the node plugin DaemonSet.                                                                                                  | `[]`                                                   | No                          |
| `node.podInfoOnMountCompat.enable`                   | Enable `podInfoOnMount` for older Kubernetes versions (&lt;1.30) if the API server supports it but Kubelet version in Helm doesn't reflect it.    | `false`                                                | No                          |

## Sidecar and Init Container Configuration

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `sidecars.nodeDriverRegistrar.image.repository`      | Image repository for the `csi-node-driver-registrar` sidecar.                                                                                      | `ghcr.io/scality/mountpoint-s3-csi-driver/csi-node-driver-registrar`     | No                          |
| `sidecars.nodeDriverRegistrar.image.tag`             | Image tag for the `csi-node-driver-registrar` sidecar.                                                                                             | `v2.14.0`                                              | No                          |
| `sidecars.nodeDriverRegistrar.image.pullPolicy`      | Image pull policy for the `csi-node-driver-registrar` sidecar.                                                                                     | `IfNotPresent`                                         | No                          |
| `sidecars.nodeDriverRegistrar.resources`             | Resource requests and limits for the `csi-node-driver-registrar` sidecar.                                                                          | `{}` (inherits from `node.resources` if not set)       | No                          |
| `sidecars.livenessProbe.image.repository`            | Image repository for the `livenessprobe` sidecar.                                                                                                  | `ghcr.io/scality/mountpoint-s3-csi-driver/livenessprobe`            | No                          |
| `sidecars.livenessProbe.image.tag`                   | Image tag for the `livenessprobe` sidecar.                                                                                                         | `v2.16.0`                                              | No                          |
| `sidecars.livenessProbe.image.pullPolicy`            | Image pull policy for the `livenessprobe` sidecar.                                                                                                 | `IfNotPresent`                                         | No                          |
| `sidecars.livenessProbe.resources`                   | Resource requests and limits for the `livenessprobe` sidecar.                                                                                      | `{}` (inherits from `node.resources` if not set)       | No                          |
| `initContainer.installMountpoint.resources`          | Resource requests and limits for the `install-mountpoint` initContainer. *Only used with SystemD mounter (default).*                              | `{}` (inherits from `node.resources` if not set)       | No                          |

## Experimental Features (Unsupported)

**Important:** The Pod Mounter feature is experimental and **not supported for production use**. It should only be used in development environments. The default SystemD mounter is the only supported configuration.

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `experimental.podMounter`                            | **EXPERIMENTAL, DO NOT USE:** Enables the Pod Mounter feature instead of the default SystemD mounter. Should be `false` for standard configurations.          | `false`                                                | No                          |
| `controller.serviceAccount.create`                   | Specifies whether a ServiceAccount should be created for the controller. *Only used if `experimental.podMounter` is true.*                        | `true`                                                 | No                          |
| `controller.serviceAccount.name`                     | Name of the ServiceAccount to use for the controller. *Only used if `experimental.podMounter` is true.*                                           | `s3-csi-driver-controller-sa`                          | No                          |
| `mountpointPod.namespace`                            | Namespace for Mountpoint pods spawned by the controller. *Only used if `experimental.podMounter` is true.*                                        | `mount-s3`                                             | No                          |
| `mountpointPod.priorityClassName`                    | Priority class name for Mountpoint pods. *Only used if `experimental.podMounter` is true.*                                                        | `mount-s3-critical`                                    | No                          |
