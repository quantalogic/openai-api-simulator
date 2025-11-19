#!/bin/bash
# Setup SmolLM repository and dependencies for inference server
# 
# Usage: ./scripts/setup-smollm.sh [smollm_dir]
#
# This script:
# 1. Clones SmolLM repository from GitHub
# 2. Installs it in editable mode for development
# 3. Exports PYTHONPATH for use with the inference server

set -e

SMOLLM_DIR="${1:-.smollm}"
REPO_URL="https://github.com/karpathy/nanochat.git"

echo "=========================================="
echo "Setting up SmolLM for Inference"
echo "=========================================="
echo ""

# Check if directory already exists
if [ -d "$SMOLLM_DIR" ]; then
    echo "✓ SmolLM directory already exists: $SMOLLM_DIR"
    echo "  Pulling latest changes..."
    (cd "$SMOLLM_DIR" && git pull)
else
    echo "↓ Cloning nanochat repository (for SmolLM inference)..."
    echo "  From: $REPO_URL"
    echo "  To:   $SMOLLM_DIR"
    git clone "$REPO_URL" "$SMOLLM_DIR"
fi

echo ""
echo "↓ Ensuring SmolLM is in PYTHONPATH..."

# Get absolute path
SMOLLM_ABS=$(cd "$SMOLLM_DIR" && pwd)

echo "  Path: $SMOLLM_ABS"

# Export PYTHONPATH (user should add to .bashrc/.zshrc for persistence)
export PYTHONPATH="${SMOLLM_ABS}:${PYTHONPATH}"

echo ""
echo "=========================================="
echo "✓ SmolLM Setup Complete!"
echo "=========================================="
echo ""
echo "Add this to your shell profile (.bashrc, .zshrc, etc.):"
echo ""
echo "  export PYTHONPATH=\"${SMOLLM_ABS}:\$PYTHONPATH\""
echo ""
echo "Or set it before running the inference server:"
echo ""
echo "  PYTHONPATH=\"${SMOLLM_ABS}\" python -m cmd.smollm.inference_server"
echo ""
echo "Verify installation:"
echo ""
echo "  python -c 'import nanochat; print(nanochat.__file__)'"
echo ""
