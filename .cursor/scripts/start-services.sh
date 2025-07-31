#!/bin/bash
set -e

echo "🚀 Starting environment for pre-commit and mkdocs development..."

# Ensure we're in the right directory
cd /workspace

# Activate Python virtual environment
if [ -d ".venv" ]; then
    echo "🐍 Activating Python virtual environment..."
    source .venv/bin/activate
else
    echo "⚠️  Python virtual environment not found. Run install script first."
    exit 1
fi

# Print status
echo "✅ Environment ready for development!"
echo ""
echo "Environment status:"
echo "  - Python: $(python3 --version)"
echo "  - pre-commit: $(pre-commit --version)"
echo "  - mkdocs: $(mkdocs --version)"
echo "  - codespell: $(codespell --version)"
echo ""
echo "Ready to:"
echo "  - Run pre-commit hooks"
echo "  - Build and serve documentation"
echo "  - Check spelling with codespell"
echo ""
