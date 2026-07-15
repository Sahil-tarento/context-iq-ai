# ContextIQ MCP — Setup & IDE Integration Guide

---

## ❓ How Does This Work? (The Key Concept)

There are **two separate AI models** at play. This is the most important thing to understand:

```
┌─────────────────────────────────────────────────────────────────┐
│                         YOUR IDE                                 │
│                                                                  │
│  You type: "explain how handleChat works"                        │
│       │                                                          │
│       ▼                                                          │
│  IDE's own AI  ◄── (Cursor's Claude / Copilot / etc.)           │
│  "I should use the contextiq_chat tool for this"                 │
│       │                                                          │
│       │  calls via MCP (JSON-RPC over stdio)                     │
│       ▼                                                          │
│  contextiq-mcp  ──► ContextIQ Daemon (:9009)                    │
│                           │                                      │
│                           │  compresses context 70%+             │
│                           ▼                                      │
│                     YOUR LLM (Ollama / OpenAI / etc.)            │
│                           │                                      │
│                           │  response                            │
│                           ▼                                      │
│  IDE's AI gets result ◄── contextiq-mcp                         │
│  and shows it to you                                             │
└─────────────────────────────────────────────────────────────────┘
```

**Model 1 — The IDE's built-in AI** (Cursor's Claude, GitHub Copilot, etc.)
- Decides *when* to call ContextIQ tools based on your prompt
- You never configure this — it's the IDE's model

**Model 2 — ContextIQ's configured LLM** (Ollama/OpenAI/Claude/Gemini)
- Does the actual code analysis
- Receives a **70%-compressed** prompt instead of raw files
- Set via `DEFAULT_PROVIDER` env var or the `--provider` argument in the tool call

> The IDE AI is the "orchestrator." ContextIQ is the "smart code context tool" it calls.

---

## Tools Exposed to the IDE

| Tool | What it does |
|---|---|
| `contextiq_chat` | Ask a coding question — context compressed 70%+ before LLM call |
| `contextiq_index` | Index a repo for semantic context retrieval |
| `contextiq_optimize` | Preview compressed context without calling an LLM |
| `contextiq_retrieve` | Retrieve original uncompressed source code for a specific CCR key/hash |
| `contextiq_health` | Check if the daemon is running |

---

## Step 1: Install the Binary (once, per machine)

This puts `contextiq-mcp` on your `$PATH` so **any IDE on your machine** can find it without a hardcoded path.

```bash
cd /path/to/context-iq-ai

# Option A — install to /usr/local/bin (may need sudo)
make install

# Option B — install to $GOPATH/bin (no sudo needed)
make go-install
# then ensure $GOPATH/bin is in your PATH:
# export PATH="$PATH:$(go env GOPATH)/bin"
```

Verify:
```bash
which contextiq-mcp
# /usr/local/bin/contextiq-mcp
```

---

## Step 2: Start the ContextIQ Daemon

```bash
# Local binary
contextiq --port 9009

# or via Docker Compose
docker compose up -d
```

Verify:
```bash
curl http://localhost:9009/v1/health
# {"status":"healthy","time":"..."}
```

---

## Step 3: Configure Your IDE

All configs use just `"command": "contextiq-mcp"` — **no hardcoded paths**. It resolves from PATH automatically.

---

### 🖱️ Cursor IDE

The config `.cursor/mcp.json` is already in the repo. Cursor detects it automatically when you open the project folder.

```json
{
  "mcpServers": {
    "contextiq": {
      "command": "contextiq-mcp",
      "args": ["--daemon-url", "http://localhost:9009"],
      "env": {}
    }
  }
}
```

**Steps:**
1. Open Cursor → Open the `context-iq-ai` project folder
2. Go to `Cursor Settings → MCP` → confirm `contextiq` shows `connected`
3. In AI chat, just ask naturally:
   ```
   explain how the Platform struct works in this codebase
   ```
   Cursor's AI will automatically decide to call `contextiq_chat` for you.

---

### 🖱️ VS Code (Copilot Chat / MCP extension)

The config `.vscode/mcp.json` is already in the repo.

```json
{
  "servers": {
    "contextiq": {
      "type": "stdio",
      "command": "contextiq-mcp",
      "args": ["--daemon-url", "http://localhost:9009"],
      "env": {}
    }
  }
}
```

**Steps:**
1. Open VS Code → Open the project folder
2. Open Copilot Chat panel → click **Tools** → enable `contextiq_*` tools
3. Ask in chat:
   ```
   @contextiq_chat how does the semantic cache work?
   ```

---

### 🖱️ Claude Desktop (Linux / macOS)

```bash
# This script auto-finds contextiq-mcp from PATH — no hardcoded paths
bash scripts/setup-claude-desktop.sh
```

Then restart Claude Desktop. You'll see the 🔧 tool icon in chat.

---

### 🖱️ Any Other MCP IDE (Windsurf, Zed, Neovim)

```json
{
  "mcpServers": {
    "contextiq": {
      "command": "contextiq-mcp",
      "args": ["--daemon-url", "http://localhost:9009"],
      "env": {}
    }
  }
}
```

---

## Example Prompts in IDE Chat

The IDE AI handles tool selection automatically. Just ask naturally:

```
# Index first (once per project)
"Index my current workspace with ContextIQ"

# Ask coding questions
"How does the token compression work in this codebase?"
"What does the GraphEngine do and what files does it depend on?"
"Explain the chat handler and show me how the masker is applied"

# Check savings
"Show me how much context compression ContextIQ is applying to this query"
```

---

## Makefile Reference

```bash
make build        # Build all 3 binaries locally
make install      # Build + copy to /usr/local/bin (may need sudo)
make go-install   # go install to $GOPATH/bin (no sudo)
make run          # Start daemon on port 9009
make test         # Run all Go tests
make clean        # Remove local binaries
```

---

## Troubleshooting

| Issue | Fix |
|---|---|
| `contextiq-mcp: command not found` | Run `make install` or `make go-install` |
| Cursor shows `contextiq: error` | Run `contextiq --port 9009` first |
| MCP shows `daemon unreachable` | Check daemon is running: `curl http://localhost:9009/v1/health` |
| IDE doesn't pick up mcp.json | Restart the IDE after first setup |
| `make install` permission denied | Run with `sudo make install` |
