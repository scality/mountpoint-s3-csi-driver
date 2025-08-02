# Glossary

This glossary defines acronyms, technical terms, and concepts used throughout the Scality CSI Driver for S3 documentation.

## Acronyms and Abbreviations

| Acronym | Full Form | Definition |
|---------|-----------|------------|
| **API** | Application Programming Interface | A set of protocols and tools for building software applications |
| **CLI** | Command Line Interface | A text-based interface for interacting with software |
| **CRD** | Custom Resource Definition | Kubernetes extension mechanism for defining custom resources |
| **CRT** | Common Runtime | AWS Common Runtime library used for S3 operations |
| **CSI** | Container Storage Interface | A standard for exposing storage systems to containerized workloads |
| **DNS** | Domain Name System | System that translates domain names to IP addresses |
| **GID** | Group Identifier | Numeric identifier for a group in Unix-like systems |
| **GHCR** | GitHub Container Registry | GitHub's container image registry service |
| **HTTP** | Hypertext Transfer Protocol | Protocol for transferring data over the web |
| **HTTPS** | HTTP Secure | Secure version of HTTP using encryption |
| **IAM** | Identity and Access Management | System for managing user identities and permissions |
| **JSON** | JavaScript Object Notation | Lightweight data interchange format |
| **KMS** | Key Management Service | Service for managing encryption keys |
| **POSIX** | Portable Operating System Interface | Set of standards for Unix-like operating systems |
| **PV** | PersistentVolume | Kubernetes resource representing a piece of storage |
| **PVC** | PersistentVolumeClaim | Kubernetes resource requesting storage from a PV |
| **RBAC** | Role-Based Access Control | Method of restricting access based on user roles |
| **S3** | Simple Storage Service | Object storage service protocol |
| **SDK** | Software Development Kit | Collection of tools for developing applications |
| **SSE** | Server-Side Encryption | Encryption of data at rest on the server |
| **TTL** | Time To Live | Duration for which data is considered valid |
| **UID** | User Identifier | Numeric identifier for a user in Unix-like systems |
| **URL** | Uniform Resource Locator | Web address identifying a resource |
| **YAML** | YAML Ain't Markup Language | Human-readable data serialization standard |

## Technical Terms

### Container and Kubernetes Terms

| Term | Definition |
|------|------------|
| **ClusterRole** | Kubernetes resource defining permissions across the entire cluster |
| **ClusterRoleBinding** | Kubernetes resource binding a ClusterRole to users or service accounts |
| **ConfigMap** | Kubernetes resource for storing configuration data |
| **DaemonSet** | Kubernetes workload that runs one pod per node |
| **Deployment** | Kubernetes workload for managing stateless applications |
| **Helm** | Package manager for Kubernetes applications |
| **initContainer** | Container that runs before main containers in a pod |
| **Kubelet** | Kubernetes agent running on each node |
| **Namespace** | Kubernetes mechanism for isolating resources |
| **Secret** | Kubernetes resource for storing sensitive data |
| **ServiceAccount** | Kubernetes identity for pods and processes |
| **sidecar** | Additional container running alongside the main container |
| **StatefulSet** | Kubernetes workload for managing stateful applications |

### Storage and File System Terms

| Term | Definition |
|------|------------|
| **fsync** | System call to synchronize file data to storage |
| **Mount Options** | Parameters controlling how a file system is mounted |
| **Mount Point** | Directory where a file system is attached |
| **Static Provisioning** | Manual creation of storage resources |
| **subPath** | Kubernetes feature for mounting a subdirectory of a volume |
| **volumeHandle** | Unique identifier for a CSI volume |

### S3 and Storage Terms

| Term | Definition |
|------|------------|
| **Access Key ID** | Public identifier for S3 authentication |
| **Bucket** | Container for objects in S3 storage |
| **Bucket Policy** | JSON document defining access permissions for an S3 bucket |
| **Endpoint** | URL where S3 API requests are sent |
| **Mountpoint for Amazon S3** | Tool for mounting S3 buckets as file systems |
| **Object** | Basic unit of data stored in S3 |
| **Prefix** | String used to filter objects in an S3 bucket |
| **RING** | Scality's distributed storage platform |
| **S3-compatible** | Storage systems that implement the S3 API |
| **Secret Access Key** | Private key for S3 authentication |
| **Session Token** | Temporary credential for S3 access |

### Scality-Specific Terms

| Term | Definition |
|------|------------|
| **Scality RING** | Scality's software-defined storage platform |
| **Scality CSI Driver for S3** | Container Storage Interface driver for Scality S3 storage |

### Operational Terms

| Term | Definition |
|------|------------|
| **Caching** | Storing frequently accessed data locally for faster access |
| **Consistency** | Guarantee about the state of data across different operations |
| **Metadata** | Data that describes other data (file attributes, timestamps, etc.) |
| **Node Selector** | Kubernetes mechanism for constraining pods to specific nodes |
| **Taints and Tolerations** | Kubernetes mechanism for controlling pod scheduling |
| **Troubleshooting** | Process of diagnosing and resolving problems |

## Common Mount Options

| Option | Description |
|--------|-------------|
| `allow-delete` | Allows deletion of files and objects |
| `allow-other` | Allows other users to access the mounted file system |
| `allow-overwrite` | Allows overwriting existing files |
| `cache` | Enables local caching of file data |
| `gid` | Sets the group ID for file ownership |
| `metadata-ttl` | Sets cache duration for file metadata |
| `prefix` | Limits access to objects with a specific prefix |
| `uid` | Sets the user ID for file ownership |

## Error Messages and Status Codes

| Status/Error | Meaning |
|--------------|---------|
| `ContainerCreating` | Pod is being created but containers haven't started |
| `Running` | Pod and containers are running successfully |
| `Terminating` | Pod is being shut down |
| `Access Denied` | Insufficient permissions for the requested operation |
| `Transport endpoint not connected` | Network connectivity issue to S3 endpoint |
