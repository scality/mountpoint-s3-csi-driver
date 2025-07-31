#!/bin/bash
set -e

echo "🚀 Installing dependencies for pre-commit and mkdocs development..."

# Ensure we're in the right directory
cd /workspace

# Activate Python virtual environment
echo "🐍 Activating Python virtual environment..."
source .venv/bin/activate

# Upgrade pip and install/upgrade requirements
echo "📦 Installing Python packages..."
pip install --upgrade pip
pip install -r requirements.txt

# Set up pre-commit hooks
echo "🪝 Installing pre-commit hooks..."
pre-commit install

echo "🎉 Dependencies installation completed successfully!"
echo ""
echo "Available commands:"
echo "  pre-commit run --all-files  - Run all pre-commit hooks"
echo "  mkdocs serve                - Start mkdocs dev server"
echo "  mkdocs build                - Build documentation"
echo "  codespell                   - Run spelling checker"
echo ""
