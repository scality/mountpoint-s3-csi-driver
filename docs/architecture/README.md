# Architecture Documentation

This directory contains detailed architecture documentation for various components of the Scality S3 CSI Driver.

## Available Documentation

### [Systemd Mounter Architecture](systemd-mounter-architecture.md)

Comprehensive documentation of the Systemd Mounter component, including:

- **Architecture Overview**: High-level component relationships and interactions
- **Static Provisioning Workflow**: Detailed sequence diagrams showing the mount process
- **Systemd Integration**: D-Bus communication and service management
- **Mount Process Deep Dive**: Step-by-step mounting procedure with validation
- **Error Handling**: Failure recovery and cleanup processes
- **Configuration**: Environment variables, mount policies, and deployment considerations

The documentation includes multiple Mermaid diagrams to illustrate:
- Component architecture and dependencies
- Sequential workflows and interactions
- Process flows and decision trees
- Data flow and control flow patterns

## Using Architecture Diagrams

All diagrams in this documentation are created using [Mermaid](https://mermaid.js.org/), which is already configured in the project's MkDocs setup. The diagrams are fully interactive and will render automatically when viewing the documentation.

## Contributing

When adding new architecture documentation:

1. Create comprehensive diagrams showing component interactions
2. Include both high-level overviews and detailed workflows
3. Provide clear explanations of design decisions and trade-offs
4. Update this README to reference new documentation
5. Add navigation entries to `mkdocs.yml`

## Related Documentation

- [Driver Deployment](../driver-deployment/) - Installation and configuration guides
- [Volume Provisioning](../volume-provisioning/) - Usage patterns and examples
- [Concepts and Reference](../concepts-and-reference/) - Technical specifications
- [Troubleshooting](../troubleshooting.md) - Common issues and solutions