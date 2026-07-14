# Architecture

Developer IDE
    |
VS Code / IntelliJ Plugin
    |
Go Local Agent
    |
Context Optimizer
  - AST
  - Symbol Graph
  - Ranking
  - Cache
    |
Enterprise Context Service
    |
LLM Gateway
    |
OpenAI / Claude / Gemini / Ollama

Primary flow:
1. Capture IDE state
2. Resolve dependencies
3. Rank relevant context
4. Compress prompt
5. Send optimized request
6. Cache response
