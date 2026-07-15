// Package mcp implements a Model Context Protocol (MCP) server for ContextIQ.
//
// The MCP protocol uses JSON-RPC 2.0 over stdio. The IDE (Cursor, Claude Desktop,
// VS Code, Windsurf etc.) launches this binary as a subprocess and communicates
// over stdin/stdout. All debug output must go to stderr.
//
// Protocol flow:
//  1. Client sends: initialize
//  2. Server responds with capability declaration (tools list)
//  3. Client sends: tools/call  { name: "contextiq_chat", arguments: {...} }
//  4. Server calls ContextIQ daemon REST API, returns result
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ─── JSON-RPC Types ──────────────────────────────────────────────────────────

// Request represents an incoming JSON-RPC 2.0 message from the MCP client.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"` // can be string, number, or null
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is the outgoing JSON-RPC 2.0 message.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError follows the JSON-RPC 2.0 error object spec.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ─── MCP Protocol Structures ─────────────────────────────────────────────────

// ServerInfo is returned during the initialize handshake.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the full response to initialize.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

// Capabilities declares what this server supports.
type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolsCapability signals tool-calling support.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// Tool defines a single callable tool exposed by this server.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema is a JSON Schema object describing tool parameters.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property describes one field in the input schema.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Items       *Items   `json:"items,omitempty"` // for array types
	Default     string   `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// Items is the schema for array element types.
type Items struct {
	Type string `json:"type"`
}

// ToolsListResult is the response to tools/list.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams is parsed from tools/call requests.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCallResult is the response to tools/call.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock holds one piece of content in a tool response.
type ContentBlock struct {
	Type string `json:"type"` // always "text" for ContextIQ
	Text string `json:"text"`
}

// ─── ContextIQ Daemon API Payloads ───────────────────────────────────────────

type chatRequest struct {
	Provider   string   `json:"provider"`
	Model      string   `json:"model"`
	Query      string   `json:"query"`
	OpenFiles  []string `json:"open_files"`
	CursorFile string   `json:"cursor_file"`
	CursorLine int      `json:"cursor_line"`
	RepoPath   string   `json:"repo_path"`
	MaxTokens  int      `json:"max_tokens"`
}

type chatResponse struct {
	Response        string  `json:"response"`
	RawTokens       int     `json:"raw_tokens"`
	OptimizedTokens int     `json:"optimized_tokens"`
	TokenSavings    float64 `json:"token_savings"`
	FromCache       bool    `json:"from_cache"`
}

type indexRequest struct {
	RepoPath string `json:"repo_path"`
}

type indexResponse struct {
	Success        bool   `json:"success"`
	FilesIndexed   int    `json:"files_indexed"`
	SymbolsIndexed int    `json:"symbols_indexed"`
	Error          string `json:"error,omitempty"`
}

type optimizeRequest struct {
	Query      string   `json:"query"`
	OpenFiles  []string `json:"open_files"`
	CursorFile string   `json:"cursor_file"`
	CursorLine int      `json:"cursor_line"`
	MaxTokens  int      `json:"max_tokens"`
}

type optimizeResponse struct {
	CompressedPrompt string  `json:"compressed_prompt"`
	TokenSavings     float64 `json:"token_savings"`
	RawTokens        int     `json:"raw_tokens"`
	OptimizedTokens  int     `json:"optimized_tokens"`
}

// ─── Chat Tool Arguments ─────────────────────────────────────────────────────

type chatArgs struct {
	Query      string   `json:"query"`
	Provider   string   `json:"provider"`
	Model      string   `json:"model"`
	CursorFile string   `json:"cursor_file"`
	CursorLine int      `json:"cursor_line"`
	OpenFiles  []string `json:"open_files"`
	RepoPath   string   `json:"repo_path"`
	MaxTokens  int      `json:"max_tokens"`
}

type indexArgs struct {
	RepoPath string `json:"repo_path"`
}

type optimizeArgs struct {
	Query      string   `json:"query"`
	CursorFile string   `json:"cursor_file"`
	CursorLine int      `json:"cursor_line"`
	OpenFiles  []string `json:"open_files"`
	MaxTokens  int      `json:"max_tokens"`
}

type healthArgs struct{}

type retrieveArgs struct {
	Key string `json:"key"`
}

type retrieveResponse struct {
	Success         bool   `json:"success"`
	OriginalContent string `json:"original_content"`
	Message         string `json:"message,omitempty"`
}

// ─── Server ──────────────────────────────────────────────────────────────────

// Server is the ContextIQ MCP server. It reads JSON-RPC messages from stdin
// and writes responses to stdout, proxying all tool calls to the ContextIQ daemon.
type Server struct {
	daemonURL  string
	httpClient *http.Client
	tools      []Tool
}

// NewServer creates a new MCP server that talks to the daemon at daemonURL.
func NewServer(daemonURL string) *Server {
	s := &Server{
		daemonURL: daemonURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
	s.tools = s.buildTools()
	return s
}

// buildTools returns the full list of MCP tools exposed by ContextIQ.
func (s *Server) buildTools() []Tool {
	return []Tool{
		{
			Name:        "contextiq_chat",
			Description: "Ask a coding question. ContextIQ compresses your codebase context by 70%+ using AST analysis and skeleton compression before sending to the LLM. Supports Ollama (local), OpenAI, Claude, Gemini, and DeepSeek.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "The coding question or task to ask the AI.",
					},
					"provider": {
						Type:        "string",
						Description: "LLM provider to use. Defaults to 'ollama' (local, zero-config).",
						Default:     "ollama",
						Enum:        []string{"ollama", "openai", "claude", "gemini", "deepseek", "mock"},
					},
					"model": {
						Type:        "string",
						Description: "Model name for the provider. E.g. 'codellama' for ollama, 'gpt-4o' for openai.",
						Default:     "codellama",
					},
					"cursor_file": {
						Type:        "string",
						Description: "Absolute path to the file currently open in the editor.",
					},
					"cursor_line": {
						Type:        "number",
						Description: "Line number where the cursor is positioned (1-indexed).",
					},
					"open_files": {
						Type:        "array",
						Description: "List of absolute paths to files currently open in the IDE.",
						Items:       &Items{Type: "string"},
					},
					"repo_path": {
						Type:        "string",
						Description: "Absolute path to the workspace/repository root.",
					},
					"max_tokens": {
						Type:        "number",
						Description: "Maximum tokens for the compressed context window. Default is 4096.",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "contextiq_index",
			Description: "Index a repository for semantic search and context optimization. Run this once after opening a new project. Creates AST-based symbol graph and vector embeddings for fast context retrieval.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"repo_path": {
						Type:        "string",
						Description: "Absolute path to the repository root to index.",
					},
				},
				Required: []string{"repo_path"},
			},
		},
		{
			Name:        "contextiq_optimize",
			Description: "Compress and optimize code context for an AI prompt without calling the LLM. Returns the compressed prompt and token savings statistics. Useful for understanding what context will be sent.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {
						Type:        "string",
						Description: "The query to optimize context for.",
					},
					"cursor_file": {
						Type:        "string",
						Description: "Absolute path to the active file.",
					},
					"cursor_line": {
						Type:        "number",
						Description: "Cursor line number (1-indexed).",
					},
					"open_files": {
						Type:        "array",
						Description: "Open file paths.",
						Items:       &Items{Type: "string"},
					},
					"max_tokens": {
						Type:        "number",
						Description: "Max tokens budget. Default 4096.",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "contextiq_health",
			Description: "Check the health status of the ContextIQ daemon. Use this to verify the daemon is running before making other calls.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "contextiq_retrieve",
			Description: "Retrieve original uncompressed source code for a specific CCR key/hash shown in code skeleton comments.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"key": {
						Type:        "string",
						Description: "The CCR Key/hash string to retrieve the original code block for.",
					},
				},
				Required: []string{"key"},
			},
		},
	}
}

// Run starts the JSON-RPC stdio loop. Blocks until stdin is closed or an error occurs.
func (s *Server) Run() error {
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	fmt.Fprintf(os.Stderr, "[ContextIQ MCP] Ready. Listening on stdio.\n")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintf(os.Stderr, "[ContextIQ MCP] stdin closed, shutting down.\n")
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "[ContextIQ MCP] Parse error: %v | raw: %s\n", err, line)
			writeError(encoder, nil, -32700, "Parse error")
			continue
		}

		fmt.Fprintf(os.Stderr, "[ContextIQ MCP] <- %s (id=%v)\n", req.Method, req.ID)

		resp := s.dispatch(&req)
		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
}

// dispatch routes an incoming request to the correct handler.
func (s *Server) dispatch(req *Request) *Response {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification — no response needed but we must not crash
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "ping":
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: map[string]string{}}
	default:
		return errorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

// ─── Handler: initialize ─────────────────────────────────────────────────────

func (s *Server) handleInitialize(req *Request) *Response {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:    "contextiq-mcp",
			Version: "1.0.0",
		},
		Capabilities: Capabilities{
			Tools: &ToolsCapability{ListChanged: false},
		},
	}
	return &Response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

// ─── Handler: tools/list ─────────────────────────────────────────────────────

func (s *Server) handleToolsList(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  ToolsListResult{Tools: s.tools},
	}
}

// ─── Handler: tools/call ─────────────────────────────────────────────────────

func (s *Server) handleToolsCall(req *Request) *Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, -32602, "Invalid params: "+err.Error())
	}

	fmt.Fprintf(os.Stderr, "[ContextIQ MCP] Tool call: %s\n", params.Name)

	var result *ToolCallResult
	var err error

	switch params.Name {
	case "contextiq_chat":
		result, err = s.toolChat(params.Arguments)
	case "contextiq_index":
		result, err = s.toolIndex(params.Arguments)
	case "contextiq_optimize":
		result, err = s.toolOptimize(params.Arguments)
	case "contextiq_health":
		result, err = s.toolHealth()
	case "contextiq_retrieve":
		result, err = s.toolRetrieve(params.Arguments)
	default:
		return errorResponse(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", params.Name))
	}

	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: &ToolCallResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			},
		}
	}

	return &Response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

// ─── Tool: contextiq_chat ────────────────────────────────────────────────────

func (s *Server) toolChat(raw json.RawMessage) (*ToolCallResult, error) {
	var args chatArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Query == "" {
		return nil, fmt.Errorf("'query' is required")
	}

	// Apply defaults
	if args.Provider == "" {
		args.Provider = "ollama"
	}
	if args.Model == "" {
		args.Model = "codellama"
	}
	if args.MaxTokens == 0 {
		args.MaxTokens = 4096
	}

	reqBody := chatRequest{
		Provider:   args.Provider,
		Model:      args.Model,
		Query:      args.Query,
		OpenFiles:  args.OpenFiles,
		CursorFile: args.CursorFile,
		CursorLine: args.CursorLine,
		RepoPath:   args.RepoPath,
		MaxTokens:  args.MaxTokens,
	}

	var resp chatResponse
	if err := s.postJSON("/v1/chat", reqBody, &resp); err != nil {
		return nil, fmt.Errorf("daemon /v1/chat error: %w", err)
	}

	cacheLabel := "🤖 Live LLM Inference"
	if resp.FromCache {
		cacheLabel = "⚡ Semantic Cache Hit (0 tokens used)"
	}

	text := fmt.Sprintf(`%s

---
📊 Token Stats: %d raw → %d optimized (%.1f%% savings)
🔗 Source: %s
---

%s`, args.Query, resp.RawTokens, resp.OptimizedTokens, resp.TokenSavings, cacheLabel, resp.Response)

	return &ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}, nil
}

// ─── Tool: contextiq_index ───────────────────────────────────────────────────

func (s *Server) toolIndex(raw json.RawMessage) (*ToolCallResult, error) {
	var args indexArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.RepoPath == "" {
		return nil, fmt.Errorf("'repo_path' is required")
	}

	var resp indexResponse
	if err := s.postJSON("/v1/index", indexRequest{RepoPath: args.RepoPath}, &resp); err != nil {
		return nil, fmt.Errorf("daemon /v1/index error: %w", err)
	}

	if !resp.Success {
		return &ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("❌ Indexing failed: %s", resp.Error)}},
			IsError: true,
		}, nil
	}

	text := fmt.Sprintf(`✅ Repository indexed successfully!

📁 Path: %s
📄 Files indexed: %d
🔣 Symbols indexed: %d

The workspace is now optimized for AI context compression. You can use contextiq_chat for queries.`,
		args.RepoPath, resp.FilesIndexed, resp.SymbolsIndexed)

	return &ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}, nil
}

// ─── Tool: contextiq_optimize ────────────────────────────────────────────────

func (s *Server) toolOptimize(raw json.RawMessage) (*ToolCallResult, error) {
	var args optimizeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Query == "" {
		return nil, fmt.Errorf("'query' is required")
	}
	if args.MaxTokens == 0 {
		args.MaxTokens = 4096
	}

	reqBody := optimizeRequest{
		Query:      args.Query,
		OpenFiles:  args.OpenFiles,
		CursorFile: args.CursorFile,
		CursorLine: args.CursorLine,
		MaxTokens:  args.MaxTokens,
	}

	var resp optimizeResponse
	if err := s.postJSON("/v1/optimize", reqBody, &resp); err != nil {
		return nil, fmt.Errorf("daemon /v1/optimize error: %w", err)
	}

	text := fmt.Sprintf(`📦 Context Optimization Report

🔍 Query: %s
📊 Raw tokens:       %d
✂️  Optimized tokens: %d
💰 Savings:          %.1f%%

--- Compressed Prompt Preview ---
%s`,
		args.Query, resp.RawTokens, resp.OptimizedTokens, resp.TokenSavings, resp.CompressedPrompt)

	return &ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}, nil
}

// ─── Tool: contextiq_health ──────────────────────────────────────────────────

func (s *Server) toolHealth() (*ToolCallResult, error) {
	resp, err := s.httpClient.Get(s.daemonURL + "/v1/health")
	if err != nil {
		return &ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("❌ Daemon unreachable at %s\nError: %v\n\nStart the daemon with: ./contextiq --port 9009", s.daemonURL, err)}},
			IsError: true,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return &ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("✅ ContextIQ daemon is healthy at %s", s.daemonURL)}},
		}, nil
	}

	return &ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("⚠️  Daemon returned status %d at %s", resp.StatusCode, s.daemonURL)}},
		IsError: true,
	}, nil
}

// ─── Tool: contextiq_retrieve ────────────────────────────────────────────────

func (s *Server) toolRetrieve(raw json.RawMessage) (*ToolCallResult, error) {
	var args retrieveArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Key == "" {
		return nil, fmt.Errorf("'key' is required")
	}

	var resp retrieveResponse
	if err := s.postJSON("/v1/retrieve", retrieveArgs{Key: args.Key}, &resp); err != nil {
		return nil, fmt.Errorf("daemon /v1/retrieve error: %w", err)
	}

	if !resp.Success {
		return &ToolCallResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("❌ CCR Retrieval failed: %s", resp.Message)}},
			IsError: true,
		}, nil
	}

	return &ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: resp.OriginalContent}},
	}, nil
}

// ─── HTTP Helper ─────────────────────────────────────────────────────────────

// postJSON marshals body, POSTs to path (relative to daemonURL), and unmarshals into result.
func (s *Server) postJSON(path string, body interface{}, result interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	url := s.daemonURL + path
	fmt.Fprintf(os.Stderr, "[ContextIQ MCP] POST %s\n", url)

	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("daemon returned HTTP %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeError(enc *json.Encoder, id interface{}, code int, msg string) {
	enc.Encode(&Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	})
}

func errorResponse(id interface{}, code int, msg string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	}
}
