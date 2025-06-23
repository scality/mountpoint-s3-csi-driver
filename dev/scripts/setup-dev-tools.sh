#!/bin/bash
# Setup script for development tools and conventional commits
# Run this script to configure your local development environment

set -e

echo "Setting up development tools for Scality S3 CSI Driver..."

print_status() {
    echo "[INFO] $1"
}

print_success() {
    echo "[SUCCESS] $1"
}

print_warning() {
    echo "[WARNING] $1"
}

print_error() {
    echo "[ERROR] $1"
}

# Check if we're in the right directory
if [[ ! -f "go.mod" ]] || [[ ! -d "pkg" ]]; then
    print_error "This script must be run from the root of the mountpoint-s3-active-dev repository"
    exit 1
fi

# Check Python installation
print_status "Checking Python installation..."
if ! command -v python3 &> /dev/null; then
    print_error "Python 3 is required but not installed"
    exit 1
fi
print_success "Python 3 found: $(python3 --version)"

# Check pip installation
if ! command -v pip3 &> /dev/null; then
    print_error "pip3 is required but not installed"
    exit 1
fi

# Set up Python virtual environment for MkDocs and documentation tools
print_status "Setting up Python virtual environment..."
if [[ ! -d "venv" ]]; then
    python3 -m venv venv
    print_success "Virtual environment created"
else
    print_success "Virtual environment already exists"
fi

# Activate virtual environment and install requirements
print_status "Installing Python dependencies..."
source venv/bin/activate
pip install -r requirements.txt
print_success "Python dependencies installed"

# Install Commitizen
print_status "Installing Commitizen for conventional commits..."
if pip show commitizen &> /dev/null; then
    print_success "Commitizen already installed: $(cz version)"
else
    pip install commitizen==4.8.3
    print_success "Commitizen installed successfully"
fi

# Install pre-commit
print_status "Installing pre-commit hooks..."
if command -v pre-commit &> /dev/null; then
    print_success "pre-commit already installed: $(pre-commit --version)"
else
    pip install pre-commit==4.0.1
    print_success "pre-commit installed successfully"
fi

# Install pre-commit hooks
print_status "Installing pre-commit hooks for this repository..."
pre-commit install --hook-type pre-commit --hook-type commit-msg
print_success "Pre-commit hooks installed"

# Set up git commit template
print_status "Setting up git commit template..."
git config commit.template .github/commit-template.txt
print_success "Git commit template configured"

# Check if Go tools are installed
print_status "Checking Go development tools..."

# Check golangci-lint
if ! command -v golangci-lint &> /dev/null; then
    print_warning "golangci-lint not found. Installing..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.56.2
    print_success "golangci-lint installed"
else
    print_success "golangci-lint found: $(golangci-lint --version)"
fi

# Check goimports
if ! command -v goimports &> /dev/null; then
    print_warning "goimports not found. Installing..."
    go install golang.org/x/tools/cmd/goimports@latest
    print_success "goimports installed"
else
    print_success "goimports found"
fi

# Check gofumpt
if ! command -v gofumpt &> /dev/null; then
    print_warning "gofumpt not found. Installing..."
    go install mvdan.cc/gofumpt@latest
    print_success "gofumpt installed"
else
    print_success "gofumpt found"
fi

# Set up VS Code settings (if .vscode directory exists)
if [[ -d ".vscode" ]]; then
    print_status "Setting up VS Code configuration..."

    # Create settings.json if it doesn't exist
    if [[ ! -f ".vscode/settings.json" ]]; then
        cat > .vscode/settings.json << 'EOF'
{
    "conventionalCommits.scopes": [
        "S3CSI-123",
        "docs",
        "ci",
        "deps",
        "helm",
        "test"
    ],
    "git.inputValidation": "always",
    "git.useCommitInputAsStashMessage": true,
    "gitlens.advanced.messages": {
        "suppressCommitHasNoPreviousCommitWarning": false
    },
    "go.formatTool": "gofumpt",
    "go.lintTool": "golangci-lint",
    "go.lintOnSave": "package",
    "[go]": {
        "editor.formatOnSave": true,
        "editor.codeActionsOnSave": {
            "source.organizeImports": true
        }
    }
}
EOF
        print_success "VS Code settings.json created"
    else
        print_success "VS Code settings.json already exists"
    fi

    # Create extensions.json for recommended extensions
    if [[ ! -f ".vscode/extensions.json" ]]; then
        cat > .vscode/extensions.json << 'EOF'
{
    "recommendations": [
        "golang.go",
        "vivaxy.vscode-conventional-commits",
        "ms-vscode.vscode-json",
        "redhat.vscode-yaml",
        "ms-kubernetes-tools.vscode-kubernetes-tools",
        "eamodio.gitlens"
    ]
}
EOF
        print_success "VS Code extensions.json created"
    else
        print_success "VS Code extensions.json already exists"
    fi
fi

# Test the setup
print_status "Testing the setup..."

# Test pre-commit hooks
print_status "Testing pre-commit hooks..."
if pre-commit run --all-files &> /dev/null; then
    print_success "Pre-commit hooks test passed"
else
    print_warning "Pre-commit hooks found some issues (this is normal for first run)"
fi

# Test commitizen
print_status "Testing commitizen..."
if cz --help &> /dev/null; then
    print_success "Commitizen test passed"
else
    print_error "Commitizen test failed"
fi

# Final instructions
echo
echo "Development environment setup complete!"
echo
echo "IMPORTANT: To use the tools, activate the virtual environment:"
echo "  source venv/bin/activate"
echo
echo "Next steps:"
echo "1. Activate venv: source venv/bin/activate"
echo "2. Try making a commit with: cz commit"
echo "3. Or use manual format: git commit -m \"feat(S3CSI-123): add new feature\""
echo "4. Read the guide: dev/docs/conventional-commits.md"
echo
echo "Available commands (after activating venv):"
echo "  cz commit        - Interactive commit creation"
echo "  cz bump          - Bump version based on commits"
echo "  cz changelog     - Generate changelog"
echo "  pre-commit run   - Run all pre-commit hooks"
echo
echo "Documentation:"
echo "  - Conventional Commits Guide: dev/docs/conventional-commits.md"
echo "  - Pre-commit configuration: .pre-commit-config.yaml"
echo "  - Commitizen configuration: .cz.toml"
echo
print_success "Happy coding! Don't forget to: source venv/bin/activate"
