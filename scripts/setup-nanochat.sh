#!/bin/bash
# Setup nanochat repository and dependencies for inference server
# 
# Usage: ./scripts/setup-nanochat.sh [nanochat_dir]
#
# This script:
# 1. Clones nanochat repository from GitHub
# 2. Installs it in editable mode for development
# 3. Exports PYTHONPATH for use with the inference server

set -e

NANOCHAT_DIR="${1:-.nanochat}"
REPO_URL="https://github.com/karpathy/nanochat.git"

echo "=========================================="
echo "Setting up NanoChat for Inference"
echo "=========================================="
echo ""

# Check if directory already exists
if [ -d "$NANOCHAT_DIR" ]; then
    echo "✓ NanoChat directory already exists: $NANOCHAT_DIR"
    echo "  Pulling latest changes..."
    (cd "$NANOCHAT_DIR" && git pull)
else
    echo "↓ Cloning nanochat repository..."
    echo "  From: $REPO_URL"
    echo "  To:   $NANOCHAT_DIR"
    git clone "$REPO_URL" "$NANOCHAT_DIR"
fi

echo ""
echo "↓ Ensuring nanochat is in PYTHONPATH..."

# Get absolute path
NANOCHAT_ABS=$(cd "$NANOCHAT_DIR" && pwd)

echo "  Path: $NANOCHAT_ABS"

# Export PYTHONPATH (user should add to .bashrc/.zshrc for persistence)
export PYTHONPATH="${NANOCHAT_ABS}:${PYTHONPATH}"

echo ""
echo "=========================================="
echo "✓ NanoChat Setup Complete!"
echo "=========================================="
echo ""
echo "Add this to your shell profile (.bashrc, .zshrc, etc.):"
echo ""
echo "  export PYTHONPATH=\"${NANOCHAT_ABS}:\$PYTHONPATH\""
echo ""
echo "Or set it before running the inference server:"
echo ""
echo "  PYTHONPATH=\"${NANOCHAT_ABS}\" python -m cmd.nanochat.inference_server"
echo ""
echo "Verify installation:"
echo ""
echo "  python -c 'import nanochat; print(nanochat.__file__)'"
echo ""
