# Understanding ContextIQ: The Core Concepts

This document provides a deep dive into exactly *how* ContextIQ prevents token burning and how its decoupled architecture integrates with any IDE.

## 1. How ContextIQ Achieves Massive Token Savings (Not Burning Tokens)

The fundamental problem with most AI coding assistants (like GitHub Copilot or standard IDE chat plugins) is that when you ask a question, they indiscriminately grab entire files—often thousands of lines long—and dump them directly into the LLM prompt. Since LLMs charge per token, this burns money rapidly and bloats the prompt so much that the AI often gets confused, hallucinating details or responding slowly due to the massive context window.

ContextIQ solves this by acting as a highly intelligent **Semantic Compressor** *before* the prompt is ever sent to the AI. Here is the step-by-step workflow of how it increases efficiency:

1. **Intelligent AST Parsing**: Instead of looking at your code as a raw block of text, ContextIQ parses your code as an Abstract Syntax Tree (AST). It understands the exact syntax boundaries—knowing exactly where a function starts, where a class ends, and what packages are imported.
2. **Dependency Tracing**: If you are working in `File A` and call a function from `File B`, the engine maps this dependency relationship dynamically in memory using a directed graph. 
3. **The "Skeleton" Compression Technique (The Secret Sauce)**: 
   - When preparing the payload for the LLM, ContextIQ sends the *full source code* **only** for the specific function your cursor is actively sitting on.
   - For all the surrounding context (the utilities, the external classes, the imported files), ContextIQ **strips out the function bodies entirely**. It compresses them into "Code Skeletons" (just the function signatures, parameter lists, and return types).
   - **Why this is brilliant**: The LLM doesn't need to know *how* an external utility sorts an array in order to help you write your current code; it only needs to know *what parameters* that utility accepts! This reduces a 5,000-token file down to 500 tokens, maintaining 100% of the structural context the AI needs.
4. **Semantic Caching**: Before even generating a prompt, the engine calculates a vector embedding of your question. If another developer on your team (or you, 5 minutes ago) asked a mathematically similar question, and the underlying files haven't changed (verified via rapid SHA256 hashing), ContextIQ intercepts the request and instantly returns the cached response. **This costs 0 tokens and executes in milliseconds.**

## 2. How the Integration Architecture Works

By decoupling the heavy intelligence into a **Background Daemon**, the IDE integration becomes trivial, lightning-fast, and "Zero-Config".

- **The Daemon (`localhost:9009`)**: You run this background service (either via the compiled Go binary or Docker Compose). It securely holds your API keys, manages the vector database connections, and executes the heavy AST parsing and graph traversals.
- **The IDE (VS Code, JetBrains, Neovim)**: The editor plugins are entirely "dumb clients." All they do is capture your current cursor position, collect the list of open files, and send a simple JSON HTTP POST request to the Daemon. 

Because the IDE doesn't handle the complex logic or sensitive API keys, you never have to configure your editor. You configure the powerful Daemon once, and any IDE you decide to use instantly reaps the benefits of 70%+ token savings!
