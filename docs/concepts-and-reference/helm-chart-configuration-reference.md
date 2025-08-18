# Helm Chart Configuration Reference

The Scality CSI Driver for S3 is configured primarily through the [`values.yaml`](https://github.com/scality/mountpoint-s3-csi-driver/blob/main/charts/scality-mountpoint-s3-csi-driver/values.yaml)
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
| `image.tag`                                          | The image tag for the CSI driver. Overrides the chart's `appVersion` if set.                                                                       | `1.2.0`                                                | No                          |

## S3 Global Configuration

<!-- markdownlint-disable MD046 -->
!!! important "Required Configuration & Backward Compatibility"
    The S3 endpoint URL must be configured for the CSI driver to function. Starting with version 1.2.0, use the global `s3.endpointUrl` and `s3.region` settings,
    which are used by both node and controller components for dynamic provisioning.

    **Backward Compatibility:** The legacy `node.s3EndpointUrl` and `node.s3Region` parameters (used in versions prior to 1.2.0) are still supported.
    If both global and legacy values are specified, the legacy `node.s3*` values take precedence to maintain backward compatibility with existing deployments.
    The legacy values will be removed in the next major version (2.0.0).
<!-- markdownlint-enable MD046 -->

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `s3.endpointUrl`                                     | The RING S3 endpoint URL used by both node and controller components for all S3 operations. <br/>**Recommended for new deployments** (replaces legacy `node.s3EndpointUrl`).                                                        | `"http://s3.example.com:8000"`                        | **Yes** (unless using legacy)                     |
| `s3.region`                                          | The default AWS region to use for S3 requests. Can be overridden per-volume via PV `mountOptions`. <br/>**Recommended for new deployments** (replaces legacy `node.s3Region`).                                               | `us-east-1`                                            | **Yes** (unless using legacy)                          |

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
| `node.s3EndpointUrl`                                 | **DEPRECATED after v1.2.0:** Legacy S3 endpoint URL setting. For backward compatibility only. Use global `s3.endpointUrl` instead. If both are specified, this legacy setting takes precedence. | `"http://s3.example.com:8000"`                        | No (use `s3.endpointUrl`)   |
| `node.s3Region`                                      | **DEPRECATED after v1.2.0:** Legacy S3 region setting. For backward compatibility only. Use global `s3.region` instead. If both are specified, this legacy setting takes precedence. | `us-east-1`                                            | No (use `s3.region`)        |
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

## Controller Plugin Configuration (Dynamic Provisioning)

<!-- markdownlint-disable MD046 -->
!!! note "Dynamic Provisioning"
    The controller component is enabled by default (`controller.enable: true`) and provides dynamic provisioning capabilities.
    When enabled, it automatically creates and deletes S3 buckets based on PersistentVolumeClaim requests that reference a StorageClass with the CSI driver.
<!-- markdownlint-enable MD046 -->

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `controller.enable`                                  | Enable controller deployment for dynamic provisioning. When enabled, allows automatic S3 bucket creation and deletion.                           | `true`                                                 | No                          |
| `controller.serviceAccount.create`                   | Specifies whether a ServiceAccount should be created for the controller.                                                                          | `true`                                                 | No                          |
| `controller.serviceAccount.name`                     | Name of the ServiceAccount to use for the controller.                                                                                             | `s3-csi-driver-controller-sa`                          | No                          |

## Experimental Features (Unsupported)

**Important:** The Pod Mounter feature is experimental and **not supported for production use**. It should only be used in development environments. The default SystemD mounter is the only supported configuration.

| Parameter                                            | Description                                                                                                                                        | Default                                                | Required                    |
|------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------|-----------------------------|
| `experimental.podMounter`                            | **EXPERIMENTAL, DO NOT USE:** Enables the Pod Mounter feature instead of the default SystemD mounter. Should be `false` for standard configurations.          | `false`                                                | No                          |
| `mountpointPod.namespace`                            | Namespace for Mountpoint pods spawned by the controller. *Only used if `experimental.podMounter` is true.*                                        | `mount-s3`                                             | No                          |
| `mountpointPod.priorityClassName`                    | Priority class name for Mountpoint pods. *Only used if `experimental.podMounter` is true.*                                                        | `mount-s3-critical`                                    | No                          |
