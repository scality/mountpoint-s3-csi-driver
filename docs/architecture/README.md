# Architecture Documentation

This section provides comprehensive architecture documentation for the Scality S3 CSI Driver using the C4 model approach. The documentation helps users understand the system's structure, components, and operational workflows.

## Documentation Structure

### 🏗️ [System Architecture](system-architecture.md)

Comprehensive overview of how the CSI driver fits into the Kubernetes ecosystem, core components, and user workflows for both current static provisioning and future dynamic provisioning capabilities.

### 🚀 [Deployment Architecture](deployment-architecture.md)

Detailed view of what components get installed in your cluster, their resource requirements, communication patterns, and operational characteristics.

### 📋 [Static Provisioning Workflow](static-provisioning-workflow.md)

Step-by-step process of how static volume provisioning works in practice.

### 🔐 [Credential Management](credential-management.md)

Authentication methods and security considerations for accessing S3 storage.

## About C4 Diagrams

The C4 model provides a hierarchical way to visualize software architecture:

- **Context**: Shows the system in its environment
- **Container**: Shows the high-level shape of the software architecture
- **Component**: Shows how containers are made up of components
- **Code**: Shows implementation details (not covered in user docs)

All diagrams are theme-aware and will adapt to your preferred color scheme (light/dark mode).

## Key Architectural Decisions

- **Systemd Integration**: Uses systemd for reliable mount process management and automatic recovery
- **Dual Provisioning Support**: Currently supports static provisioning (pre-existing S3 buckets) with dynamic provisioning (automatic bucket creation) planned for future releases
- **Credential Flexibility**: Supports both node-level and volume-level authentication methods
- **FUSE Filesystem**: Uses AWS Mountpoint for S3 for high-performance POSIX filesystem interface
