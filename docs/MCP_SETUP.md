# ContextIQ MCP — Setup & IDE Integration Guide

## What Was Built

A **full MCP (Model Context Protocol) server** in Go that:
- Runs as a subprocess launched by your IDE
- Speaks **JSON-RPC 2.0 over stdio** (the MCP standard)
- Proxies all AI calls to your existing **ContextIQ daemon** at `localhost:9009`
- Exposes **4 tools** your IDE's AI can call

```
Your IDE AI (Cursor / Claude Desktop / VS Code)
        │  JSON-RPC 2.0 over stdio
        ▼
  contextiq-mcp  ←── NEW binary (Go)
        │  HTTP REST
        ▼
  contextiq daemon  ←── already existed (port 9009)
        │
        ▼
  Ollama / OpenAI / Claude / Gemini
```

---

## Tools Available

| Tool | What it does |
|---|---|
| `contextiq_chat` | Ask a coding question — context is compressed 70%+ before LLM call |
| `contextiq_index` | Index a repository for semantic context retrieval |
| `contextiq_optimize` | Preview the compressed context without calling an LLM |
| `contextiq_health` | Check if the daemon is running |

---

## Step 1: Build the MCP Binary

```bash
cd /home/sahilchaudhary/Downloads/Personal/PersonalGitRepo/context-iq-ai

go build -o contextiq-mcp ./cmd/mcp/
```

> ✅ Already done — `contextiq-mcp` binary is in the project root.

---

## Step 2: Start the ContextIQ Daemon

The MCP server needs the daemon running to forward requests to.

```bash
# Option A: Local binary
./contextiq --port 9009

# Option B: Docker Compose
docker compose up -d
```

Verify it's healthy:
```bash
curl http://localhost:9009/v1/health
# {"status":"healthy","time":"..."}
```

---

## Step 3: Add to Your IDE

### 🖱️ Cursor IDE

The config is already in the repo at `.cursor/mcp.json`. Cursor reads this automatically.

1. Open Cursor → Open the ContextIQ project folder
2. Cursor detects `.cursor/mcp.json` on workspace load
3. Go to **Cursor Settings → MCP** — you should see `contextiq` listed as `connected`
4. In AI chat, type:
   ```
   Use contextiq_chat to explain what the Platform struct does in server.go
   ```

**Config file** (`.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "contextiq": {
      "command": "/path/to/contextiq-mcp",
      "args": ["--daemon-url", "http://localhost:9009"]
    }
  }
}
```

---

### 🖱️ VS Code (with MCP support)

The config is at `.vscode/mcp.json`. Works with:
- VS Code Insiders with built-in MCP
- GitHub Copilot Chat with MCP extension

1. Open VS Code → Open the project folder
2. VS Code reads `.vscode/mcp.json`
3. Open Copilot Chat → Tools → Enable `contextiq_*`
4. In Copilot Chat: `@contextiq_chat what does handleIndex do?`

---

### 🖱️ Claude Desktop (Linux)

```bash
# Run the setup script
bash scripts/setup-claude-desktop.sh

# Restart Claude Desktop
```

The script writes to `~/.config/claude/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "contextiq": {
      "command": "/path/to/contextiq-mcp",
      "args": ["--daemon-url", "http://localhost:9009"]
    }
  }
}
```

In Claude Desktop, you'll see ContextIQ tools appear in the tool selector (🔧 icon).

---

### 🖱️ Any MCP-compatible IDE (Generic)

For **Windsurf**, **Zed**, **Neovim** (via `mcptools`), or any other client that supports MCP:

```json
{
  "mcpServers": {
    "contextiq": {
      "command": "/absolute/path/to/contextiq-mcp",
      "args": ["--daemon-url", "http://localhost:9009"],
      "env": {}
    }
  }
}
```

---

## Example Usage in IDE Chat

### Index your project first (once per project):
```
Use contextiq_index to index /home/sahilchaudhary/Downloads/Personal/PersonalGitRepo/context-iq-ai
```

### Ask a coding question:
```
Use contextiq_chat to explain how the semantic cache works in this codebase
```

### Check token savings:
```
Use contextiq_optimize with query "how does the masker work" and cursor_file "/path/to/server.go"
```

---

## Files Created

| File | Purpose |
|---|---|
| `internal/mcp/server.go` | Full MCP server (JSON-RPC stdio + all tools) |
| `cmd/mcp/main.go` | Binary entrypoint |
| `.cursor/mcp.json` | Cursor IDE config (auto-detected) |
| `.vscode/mcp.json` | VS Code config (auto-detected) |
| `scripts/setup-claude-desktop.sh` | Claude Desktop setup script |

---

## Troubleshooting

| Issue | Fix |
|---|---|
| Tool shows "daemon unreachable" | Run `./contextiq --port 9009` first |
| Cursor doesn't show contextiq | Check absolute path in `.cursor/mcp.json` matches where binary is |
| MCP server crashes | Check stderr: `./contextiq-mcp --daemon-url http://localhost:9009 2>&1` |
| Binary not found | Run `go build -o contextiq-mcp ./cmd/mcp/` |
