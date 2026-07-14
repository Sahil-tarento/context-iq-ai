#!/bin/bash
# ContextIQ MCP Claude Desktop Config Generator
# Run: bash scripts/setup-claude-desktop.sh

BINARY_PATH="/home/sahilchaudhary/Downloads/Personal/PersonalGitRepo/context-iq-ai/contextiq-mcp"
CONFIG_DIR="$HOME/.config/claude"
CONFIG_FILE="$CONFIG_DIR/claude_desktop_config.json"

mkdir -p "$CONFIG_DIR"

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

echo "✅ Claude Desktop MCP config written to: $CONFIG_FILE"
echo "   Restart Claude Desktop to apply."
