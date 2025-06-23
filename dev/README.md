# Developer Resources

Welcome to the Scality S3 CSI Driver development environment! This directory contains all the tools and documentation needed for contributing to the project.

## Quick Setup

```bash
# 1. Run setup script
./scripts/setup-dev-tools.sh

# 2. Activate virtual environment for commitizen tools
source venv/bin/activate

# 3. Start using conventional commits
cz commit
```

This script will install and configure all necessary development tools automatically.

## Directory Structure

- `docs/` - Internal development documentation
- `scripts/` - Development automation scripts

## What Gets Installed

The setup script configures:

- Conventional commit tools (Commitizen, pre-commit hooks)
- Go development tools (golangci-lint, goimports, gofumpt)
- IDE integration (VS Code extensions and settings)
- Git hooks for automated validation

## Development Workflow

1. Run setup script once: `./scripts/setup-dev-tools.sh`
2. Activate virtual environment: `source venv/bin/activate`
3. Create feature branch: `git checkout -b feature/your-feature`
4. Make changes following our standards
5. Commit with conventional format: `cz commit`
6. Push and create pull request

## Key Benefits

- Consistent development workflow across team members
- Automated code formatting and linting
- Conventional commits for automated changelog generation
- Pre-commit validation to catch issues early
- IDE integration for better developer experience

## Documentation

- **[Conventional Commits Guide](docs/conventional-commits.md)** - Complete guide for standardized commit messages
- **[Scripts Documentation](scripts/README.md)** - Information about development scripts

## Purpose

This directory helps maintain:

- Consistent development workflow across the team
- Automated tooling for code quality and commit standards
- Clear separation between developer resources and user-facing documentation
- Easy onboarding for new contributors

The content here is separate from the user-facing documentation in `/docs` which is published via MkDocs.
