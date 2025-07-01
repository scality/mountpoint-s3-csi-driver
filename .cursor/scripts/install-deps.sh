#!/bin/bash
set -e

echo "🚀 Installing dependencies for Scality S3 CSI Driver development..."

# Ensure we're in the right directory
cd /workspace

# Set up Go environment variables
export PATH="/usr/local/go/bin:/go/bin:${PATH}"
export GOPATH="/go"
export GOBIN="/go/bin"
export GO111MODULE=on
export GOPROXY=direct

# Download Go modules and dependencies
echo "📦 Downloading Go modules..."
go mod download
go mod tidy

# Run the project's download-tools target if available
if make -n download-tools >/dev/null 2>&1; then
    echo "🔧 Running project download-tools..."
    make download-tools
fi

# Set up Python virtual environment and install requirements
echo "🐍 Setting up Python virtual environment..."
if [ ! -d ".venv" ]; then
    python3 -m venv .venv
fi

source .venv/bin/activate
pip install --upgrade pip
pip install -r requirements.txt

# Install additional Go tools if not already present
echo "🛠️  Installing/updating Go development tools..."
go install golang.org/x/tools/cmd/goimports@latest || echo "goimports already installed"
go install mvdan.cc/gofumpt@latest || echo "gofumpt already installed"
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0 || echo "golangci-lint already installed"

# Install controller-runtime tools for testing
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest || echo "setup-envtest already installed"

# Verify critical tools are available
echo "✅ Verifying tool installations..."
go version
python3 --version
helm version --short
kubectl version --client --output=yaml
docker --version || echo "⚠️  Docker daemon not running (expected in container)"
golangci-lint --version
lychee --version
markdownlint --version

# Set up pre-commit hooks
echo "🪝 Installing pre-commit hooks..."
source .venv/bin/activate
pre-commit install

# Create any necessary directories
mkdir -p bin
mkdir -p tests/bin

echo "🎉 Dependencies installation completed successfully!"
echo ""
echo "Available commands:"
echo "  make test           - Run all tests"
echo "  make unit-test      - Run unit tests"
echo "  make lint           - Run linting"
echo "  make precommit      - Run pre-commit checks"
echo "  make docs           - Build and serve documentation"
echo "  make bin            - Build binaries"
echo ""
