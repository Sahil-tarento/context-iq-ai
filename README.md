# ContextIQ: Enterprise AI Context Optimization

<div align="center">
  <h3>Reduce LLM Token Consumption by 70%+ Without Losing Code Context</h3>
</div>

---

## 🌟 What is ContextIQ?

When developers ask an AI a question in their IDE (e.g., *"How does the billing module work?"*), most AI coding assistants blindly copy-paste entire files or huge chunks of text into the LLM prompt. This results in **massive token burning**, expensive API bills, and slower AI response times due to bloated contexts.

**ContextIQ** is a Go-based middleware daemon that sits exactly between your IDE and the AI providers (OpenAI, Claude, Gemini, DeepSeek, etc.). Its singular goal is to **semantically compress** the code context before it ever reaches the AI.

### How Does it Save 70% of Tokens? (The Secret Sauce)

Instead of sending raw files, ContextIQ parses your codebase intelligently:

1. **AST & Lexical Parsing**: It reads your code (Go, Python, Java, TS, etc.) and understands where functions, classes, and imports begin and end.
2. **Dependency Graphing**: It builds a relationship web. If you ask about `Function A`, it knows `Function A` calls `Function B` and implements `Interface C`.
3. **Semantic Skeleton Compression**: This is where the magic happens. 
   - For the **direct symbol** you are asking about, it sends the full code body.
   - For **related dependencies** (like a massive utility class), it **removes the function bodies** and only sends the *signatures* (the code skeleton). 
   - The LLM still knows exactly what parameters the utility class takes, but you don't pay tokens for the internal logic of the utility class!
4. **Sensitive Data Masking**: Before the prompt leaves your machine, ContextIQ uses Regex and Shannon Entropy to redact API keys, PII, and credentials, replacing them with tokens like `[MASKED_AWS_KEY]`.

---

## 🏗️ Architecture

ContextIQ is decoupled from your IDE. It runs as a local or remote **Daemon (Server)**. 

Because of this, it is entirely **Zero-Config for the IDE**. You don't need to put API keys in VS Code or IntelliJ. You configure the Daemon once, and *any* IDE can connect to it.

```text
[ VS Code / IntelliJ / Neovim ]  <-- (Zero Config, Just UI)
             │
       (HTTP / gRPC)
             │
[ ContextIQ Daemon Server ]      <-- (Parses AST, Compresses Context, Masks Secrets)
             │
       (Optimized Prompt)
             │
[ OpenAI / Claude / Gemini ]     <-- (Receives 70% fewer tokens, replies 2x faster!)
```

---

## 🚀 Quickstart Guide

### 1. Run the ContextIQ Daemon

You must start the daemon first. You can run it entirely in-memory using Go, or via Docker for a full enterprise setup with persistent PostgreSQL and Qdrant vector databases.

**Option A: Local In-Memory (Fastest for testing)**
```bash
git clone <your-repo>
cd ContextIQ-Architecture

# Export your preferred LLM key (ContextIQ supports OpenAI, Anthropic, Gemini, DeepSeek, Ollama)
export OPENAI_API_KEY="sk-..."
export DEFAULT_PROVIDER="openai"

# Run the daemon (binds to localhost:9009)
go run ./cmd/contextiq/main.go
```

**Option B: Enterprise Docker Compose**
```bash
docker compose up --build -d
```

---

## 🔌 IDE Integration

Once the daemon is running, you can connect your IDE.

### VS Code (Zero-Config Plugin)
We bundle a native VS Code plugin that automatically talks to the daemon.
1. Navigate to the plugin folder: `cd plugins/vscode`
2. Install dependencies: `npm install`
3. Package the extension: `npm run package`
4. Install it: `npm run install-ext`
5. **Usage**: Open a code file, press **`Ctrl+Alt+C`** (or `Cmd+Alt+C`), type your question, and watch the optimized response arrive!

### Universal CLI (Neovim, JetBrains, Emacs)
For editors without native plugins, ContextIQ provides a universal CLI wrapper.
1. Build the CLI: `go build -o contextiq-cli ./cmd/cli/main.go`
2. Ask a question directly from the terminal (or map this command to a shortcut in Neovim/IntelliJ):
   ```bash
   ./contextiq-cli ask --query="Explain this method" --cursor-file="/path/to/app.go" --repo-path="/path/to/project"
   ```

---

## 📁 Repository Map

- **`cmd/contextiq/`**: The main entrypoint for the daemon.
- **`cmd/cli/`**: The universal IDE terminal wrapper.
- **`internal/parser/`**: The AST and Lexical token parsers.
- **`internal/compressor/`**: The skeleton compression algorithm that saves tokens.
- **`internal/masker/`**: The PII and high-entropy secret redaction engine.
- **`plugins/vscode/`**: The TypeScript source code for the VS Code IDE plugin.
- **`docs/`**: Deep-dive architectural diagrams and IDE mapping guides.
