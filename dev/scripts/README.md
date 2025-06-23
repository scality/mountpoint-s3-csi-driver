# Development Scripts

This directory contains automation scripts for setting up and maintaining the development environment.

## setup-dev-tools.sh

The main development environment setup script that configures all necessary tools for contributing to the project.

### What it does

- Installs Commitizen for interactive commit creation
- Sets up pre-commit hooks for automated validation
- Configures Go development tools (golangci-lint, goimports, gofumpt)
- Sets up IDE integration (VS Code extensions and settings)
- Configures git commit templates and hooks

### Usage

```bash
# Run from repository root
./dev/scripts/setup-dev-tools.sh
```

### Requirements

- Python 3 and pip3
- Go 1.19 or later
- Git

### Installed Tools

- `commitizen==4.8.3` - Interactive commit message creation
- `pre-commit==4.0.1` - Git hook management and validation
- `golangci-lint` - Go code linting and formatting
- `goimports` and `gofumpt` - Go code formatting

After running this script, you'll be ready to use conventional commits with validation and automated tooling.
