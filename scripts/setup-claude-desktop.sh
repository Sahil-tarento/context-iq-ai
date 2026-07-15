#!/bin/bash
# ContextIQ MCP — Claude Desktop Config Generator
# Run: bash scripts/setup-claude-desktop.sh
#
# Requires contextiq-mcp to be on PATH (run `make install` first).

set -e

# Resolve binary from PATH — works for any user on any machine
BINARY_PATH="$(which contextiq-mcp 2>/dev/null || true)"

if [ -z "$BINARY_PATH" ]; then
  echo "❌ 'contextiq-mcp' not found in PATH."
  echo "   Run one of the following first:"
  echo "     make install          (installs to /usr/local/bin)"
  echo "     go install ./cmd/mcp/ (installs to \$GOPATH/bin)"
  exit 1
fi

echo "✅ Found binary: $BINARY_PATH"

# Claude Desktop config location (Linux: ~/.config/claude, macOS: ~/Library/Application Support/Claude)
if [[ "$OSTYPE" == "darwin"* ]]; then
  CONFIG_DIR="$HOME/Library/Application Support/Claude"
else
  CONFIG_DIR="$HOME/.config/claude"
fi

mkdir -p "$CONFIG_DIR"
CONFIG_FILE="$CONFIG_DIR/claude_desktop_config.json"

cat > "$CONFIG_FILE" <<EOF
{
  "mcpServers": {
    "contextiq": {
      "command": "$BINARY_PATH",
      "args": ["--daemon-url", "http://localhost:9009"],
      "env": {}
    }
  }
}
EOF

echo "✅ Claude Desktop MCP config written to:"
echo "   $CONFIG_FILE"
echo ""
echo "👉 Restart Claude Desktop to apply."
echo "   Then look for the 🔧 tool icon in Claude Desktop chat."
