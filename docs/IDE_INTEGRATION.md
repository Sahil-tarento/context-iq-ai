# Universal IDE Integration Architecture

ContextIQ is built strictly around a **decoupled daemon architecture**. By running all AST parsing, vector caching, and LLM communication in a standalone Go backend daemon, ContextIQ easily integrates with **ANY IDE** (VS Code, JetBrains, Neovim, Emacs, Eclipse).

## 1. Architectural Diagram

```mermaid
graph TD
    subgraph IDE Layer (Zero Config)
        VSCode[VS Code Plugin]
        Neovim[Neovim/Vim Script]
        JetBrains[JetBrains Plugin]
        AnyEditor[Any Text Editor]
    end

    subgraph Universal Intermediary
        CLI[contextiq-cli wrapper]
    end

    subgraph ContextIQ Daemon (localhost:9009)
        API[HTTP REST API Gateway]
        Parser[Code Parser & Indexer]
        Ranker[Graph & Semantic Ranker]
        Compressor[Prompt Compressor]
    end

    subgraph Intelligence & Storage
        Qdrant[(Local Vector DB)]
        LLM[Local / Remote LLM]
    end

    %% Routing
    VSCode -->|HTTP POST| API
    Neovim -->|Subprocess Execution| CLI
    JetBrains -->|HTTP POST| API
    AnyEditor -->|Subprocess Execution| CLI

    CLI -->|HTTP POST| API

    %% Backend processing
    API --> Parser
    Parser --> Ranker
    Ranker --> Compressor
    Ranker -.->|Query| Qdrant
    Compressor --> LLM
```

---

## 2. Setup: Running the ContextIQ Daemon

Before integrating with any IDE, ensure the ContextIQ engine is running in the background. It will bind to port `9009`.

**Option A: Standalone Binary (Lightweight)**
```bash
go build -o contextiq ./cmd/contextiq/main.go
./contextiq
```

**Option B: Docker Compose (Enterprise Setup)**
```bash
docker compose up -d
```

---

## 3. Universal IDE Integration via CLI

For editors without dedicated plugins (like Neovim, Vim, or Emacs), you can use the `contextiq-cli`.

### Build the CLI Wrapper
```bash
go build -o contextiq-cli ./cmd/cli/main.go
# Move it to your path so your IDE can access it
sudo mv contextiq-cli /usr/local/bin/
```

### Neovim / Vim Configuration Example

You can bind a keystroke in Vim to capture the current line and file, pass it to `contextiq-cli`, and display the response in a split window.

Add this to your `.vimrc` or `init.vim`:
```vim
function! ContextIQAsk()
    let l:query = input('Ask ContextIQ: ')
    let l:file = expand('%:p')
    let l:line = line('.')
    let l:repo = getcwd()
    
    " Execute the contextiq-cli
    let l:cmd = 'contextiq-cli ask --query="' . l:query . '" --cursor-file="' . l:file . '" --cursor-line=' . l:line . ' --repo-path="' . l:repo . '"'
    
    " Open a new vertical split and read the command output
    vnew
    setlocal buftype=nofile
    execute 'read !' . l:cmd
    normal! gg
endfunction

" Bind to Leader + c + q
nnoremap <Leader>cq :call ContextIQAsk()<CR>
```

### IntelliJ IDEA / JetBrains (Using External Tools)

If you don't want to install a plugin in IntelliJ, use their **External Tools** feature:
1. Go to **Settings > Tools > External Tools**.
2. Click **+** to add a new tool.
3. Name: `ContextIQ Ask`
4. Program: `contextiq-cli` (ensure it's in your system PATH)
5. Arguments: 
   ```bash
   ask --query="$Prompt$" --cursor-file="$FilePath$" --cursor-line=$LineNumber$ --repo-path="$ProjectFileDir$"
   ```
6. Bind this External Tool to a Keyboard Shortcut in the Keymap settings.

---

## 4. Native REST API Integration

If you prefer to write an extension in your editor's native language (e.g. Python for Sublime Text, Java/Kotlin for IntelliJ, TypeScript for VS Code), you can bypass the CLI and hit the REST API directly.

**POST** `http://localhost:9009/v1/chat`

**Payload:**
```json
{
  "query": "How is area calculated?",
  "cursor_file": "/absolute/path/to/math.go",
  "cursor_line": 15,
  "repo_path": "/absolute/path/to/project",
  "open_files": ["/absolute/path/to/math.go", "/absolute/path/to/app.go"]
}
```

**Response:**
```json
{
  "response": "The area is calculated in CalcArea using length * width...",
  "raw_tokens": 5120,
  "optimized_tokens": 1024,
  "token_savings": 80.0,
  "from_cache": false
}
```

By adhering to this standard HTTP payload, ContextIQ achieves true **IDE Independence**.
