# ContextIQ VS Code Extension

This is the official IDE integration for **ContextIQ**, the Enterprise AI Context Optimization Platform.

## Zero Configuration Philosophy

This extension is designed to be **Zero Config**. You do not need to paste OpenAI API keys into your VS Code settings, configure endpoints, or manage complex IDE configurations.

Instead, the extension automatically communicates with the **ContextIQ Local Daemon** (running on `http://localhost:9009`). The daemon handles:
- Project Indexing & AST parsing.
- Context Ranking and Prompt Compression (saving 70%+ of tokens).
- Connecting to your local LLM (e.g. Ollama) or routing to an enterprise-provisioned LLM gateway via your backend.

By decoupling the IDE from the API configurations, your credentials and parsing logic remain completely out of the editor!

## Installation

1. Navigate to this directory in your terminal:
   ```bash
   cd plugins/vscode
   ```
2. Install dependencies:
   ```bash
   npm install
   ```
3. Compile the extension and package it:
   ```bash
   npm run package
   ```
4. Install it in VS Code:
   ```bash
   npm run install-ext
   ```
   *(Or manually install the `.vsix` file via the Extensions panel in VS Code).*

## Usage

1. **Start the ContextIQ Daemon**:
   Make sure you have compiled and run the Go application in your terminal:
   ```bash
   # From the project root
   go run ./cmd/contextiq/main.go
   ```

2. **Index Your Workspace**:
   Open the VS Code Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`) and run:
   **`ContextIQ: Index Workspace`**

3. **Ask ContextIQ**:
   Press `Ctrl+Alt+C` (or `Cmd+Alt+C` on Mac) while focusing on an open file, or run **`ContextIQ: Ask AI (Zero Config)`** from the Command Palette.
   
   Enter your query, and ContextIQ will instantly fetch the relevant context, compress it, mask sensitive data, and return a response seamlessly inside a Webview Panel!
