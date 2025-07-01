#!/bin/bash
set -e

echo "🚀 Starting services for Scality S3 CSI Driver development..."

# Ensure we're in the right directory
cd /workspace

# Set up environment variables
export PATH="/usr/local/go/bin:/go/bin:${PATH}"
export GOPATH="/go"
export GOBIN="/go/bin"

# Start Docker daemon if needed (for container builds)
if ! docker info >/dev/null 2>&1; then
    echo "🐳 Starting Docker daemon..."
    sudo service docker start || echo "⚠️  Could not start Docker daemon"
fi

# Activate Python virtual environment
if [ -d ".venv" ]; then
    echo "🐍 Activating Python virtual environment..."
    source .venv/bin/activate
fi

# Print status
echo "✅ Services started successfully!"
echo ""
echo "Environment ready for development:"
echo "  - Go: $(go version)"
echo "  - Python: $(python3 --version)"
echo "  - Docker: $(docker --version 2>/dev/null || echo 'Not available')"
echo "  - kubectl: $(kubectl version --client --short 2>/dev/null || echo 'Not available')"
echo "  - Helm: $(helm version --short 2>/dev/null || echo 'Not available')"
echo ""
