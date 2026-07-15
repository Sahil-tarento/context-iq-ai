#!/bin/bash
# ContextIQ — Install all binaries to /usr/local/bin
# This makes `contextiq`, `contextiq-cli`, and `contextiq-mcp` available system-wide.
#
# Usage: bash scripts/install.sh
#        sudo bash scripts/install.sh   (if /usr/local/bin needs root)

set -e

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=================================================="
echo "  ContextIQ — System Install"
echo "  Install dir: $INSTALL_DIR"
echo "=================================================="

# Build all binaries
echo ""
echo "📦 Building binaries..."
cd "$REPO_ROOT"

go build -o contextiq       ./cmd/contextiq/
go build -o contextiq-cli   ./cmd/cli/
go build -o contextiq-mcp   ./cmd/contextiq-mcp/

echo "  ✅ contextiq"
echo "  ✅ contextiq-cli"
echo "  ✅ contextiq-mcp"

# Install to system PATH
echo ""
echo "📁 Installing to $INSTALL_DIR ..."

install -m 0755 contextiq       "$INSTALL_DIR/contextiq"
install -m 0755 contextiq-cli   "$INSTALL_DIR/contextiq-cli"
install -m 0755 contextiq-mcp   "$INSTALL_DIR/contextiq-mcp"

echo "  ✅ Installed contextiq      → $INSTALL_DIR/contextiq"
echo "  ✅ Installed contextiq-cli  → $INSTALL_DIR/contextiq-cli"
echo "  ✅ Installed contextiq-mcp  → $INSTALL_DIR/contextiq-mcp"

echo ""
echo "🔍 Verifying PATH resolution..."
which contextiq-mcp && echo "  ✅ contextiq-mcp found in PATH: $(which contextiq-mcp)"

echo ""
echo "=================================================="
echo "  Installation complete!"
echo "  Next steps:"
echo "    1. Start the daemon:   contextiq --port 9009"
echo "    2. Setup Claude:       bash scripts/setup-claude-desktop.sh"
echo "    3. Open Cursor/VSCode with the project — MCP auto-connects"
echo "=================================================="
