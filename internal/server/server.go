package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/contextiq/internal/cache"
	"github.com/example/contextiq/internal/config"
	"github.com/example/contextiq/internal/compressor"
	"github.com/example/contextiq/internal/database"
	"github.com/example/contextiq/internal/graph"
	"github.com/example/contextiq/internal/masker"
	"github.com/example/contextiq/internal/model"
	"github.com/example/contextiq/internal/parser"
	"github.com/example/contextiq/internal/pb"
	"github.com/example/contextiq/internal/provider"
	"github.com/example/contextiq/internal/ranker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Platform orchestrates all platform components.
type Platform struct {
	Config      *config.Config
	DB          *database.DBEngine
	Cache       *cache.CacheManager
	GoParser    *parser.GoParser
	GenParser   *parser.GeneralParser
	Masker      *masker.Masker
	GraphEngine *graph.GraphEngine
}

// NewPlatform initializes the platform engine.
func NewPlatform(cfg *config.Config) (*Platform, error) {
	db, err := database.NewDBEngine(cfg.DatabaseURL, cfg.QdrantURL)
	if err != nil {
		return nil, fmt.Errorf("failed to init database: %w", err)
	}

	if err := db.InitSQLSchema(); err != nil {
		return nil, fmt.Errorf("failed to init sql schema: %w", err)
	}

	return &Platform{
		Config:      cfg,
		DB:          db,
		Cache:       cache.NewCacheManager(),
		GoParser:    parser.NewGoParser(),
		GenParser:   parser.NewGeneralParser(),
		Masker:      masker.NewMasker(),
		GraphEngine: graph.NewGraphEngine(),
	}, nil
}

// IndexRepository scans and indices code symbols in a directory.
func (p *Platform) IndexRepository(ctx context.Context, repoPath string) (int, int, error) {
	filesIndexed := 0
	symbolsIndexed := 0

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, hidden files/folders (e.g. .git)
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Detect language
		lang := parser.DetectLanguage(path)
		if lang == "unknown" {
			return nil // skip unsupported languages
		}

		// Read content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Check context cache (incremental parsing)
		sha := parser.GetSHA256(content)
		var fileNode *model.FileNode
		cachedNode, hit := p.Cache.GetContext(path, sha)

		if hit {
			fileNode = cachedNode
		} else {
			// Parse file
			var parseErr error
			if lang == "go" {
				fileNode, parseErr = p.GoParser.Parse(path, content)
			} else {
				fileNode, parseErr = p.GenParser.Parse(path, content)
			}

			if parseErr != nil {
				// Log warning and continue rather than failing the whole index operation
				fmt.Printf("Warning: failed to parse %s: %v\n", path, parseErr)
				return nil
			}

			p.Cache.SetContext(path, fileNode)
		}

		p.GraphEngine.AddFileNode(fileNode)
		filesIndexed++
		symbolsIndexed += len(fileNode.Symbols)

		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	// Build dependency relationships between symbols
	p.GraphEngine.LinkSymbols()

	// Asynchronously generate embeddings and index in vector store to avoid blocking response
	go func() {
		p.indexVectors(context.Background())
	}()

	return filesIndexed, symbolsIndexed, nil
}

// indexVectors generates embeddings for symbols and writes them to the DB.
func (p *Platform) indexVectors(ctx context.Context) {
	prov, err := provider.NewProvider(p.Config.DefaultProv, provider.ClientConfig{
		APIKey:   p.getAPIKey(p.Config.DefaultProv),
		Endpoint: p.getEndpoint(p.Config.DefaultProv),
	})
	if err != nil {
		fmt.Printf("Error building provider for embedding: %v\n", err)
		return
	}

	var records []database.VectorRecord
	for _, sym := range p.GraphEngine.Graph.Symbols {
		// Use signature + body preview for embedding
		textToEmbed := fmt.Sprintf("%s\n%s", sym.Signature, sym.Body)
		if len(textToEmbed) > 800 {
			textToEmbed = textToEmbed[:800]
		}

		emb, err := prov.GenerateEmbedding(ctx, textToEmbed)
		if err != nil {
			// fail silently for individual embedding errors
			continue
		}

		// Unique ID generation for vector
		hasher := sha256.New()
		hasher.Write([]byte(sym.ID))
		vectorID := hex.EncodeToString(hasher.Sum(nil))[:32] // Qdrant prefers 32-char hex string

		records = append(records, database.VectorRecord{
			ID:     vectorID,
			Vector: emb,
			Payload: map[string]interface{}{
				"id":        sym.ID,
				"name":      sym.Name,
				"type":      string(sym.Type),
				"file_path": sym.FilePath,
			},
		})
	}

	if len(records) > 0 {
		if err := p.DB.UpsertVectors(ctx, "code_symbols", records); err != nil {
			fmt.Printf("Error upserting vectors: %v\n", err)
		}
	}
}

// Helper methods to get credentials
func (p *Platform) getAPIKey(prov string) string {
	switch prov {
	case "openai":
		return p.Config.OpenAIKey
	case "claude":
		return p.Config.ClaudeKey
	case "gemini":
		return p.Config.GeminiKey
	case "deepseek":
		return p.Config.DeepSeekKey
	default:
		return ""
	}
}

func (p *Platform) getEndpoint(prov string) string {
	switch prov {
	case "ollama":
		return p.Config.OllamaURL
	default:
		return ""
	}
}

// ==========================================
// gRPC Server Implementation
// ==========================================
type GRPCServer struct {
	pb.UnimplementedContextServiceServer
	platform *Platform
}

func NewGRPCServer(p *Platform) *GRPCServer {
	return &GRPCServer{platform: p}
}

func (s *GRPCServer) Index(ctx context.Context, req *pb.IndexRequest) (*pb.IndexResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path is required")
	}

	files, symbols, err := s.platform.IndexRepository(ctx, req.RepoPath)
	if err != nil {
		return &pb.IndexResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.IndexResponse{
		Success:        true,
		Message:        "Repository indexed successfully",
		FilesIndexed:   int32(files),
		SymbolsIndexed: int32(symbols),
	}, nil
}

func (s *GRPCServer) Optimize(ctx context.Context, req *pb.OptimizeRequest) (*pb.OptimizeResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	// 1. Rank symbols
	relevance := ranker.NewRelevanceEngine(s.platform.GraphEngine.Graph)
	ranked := relevance.RankContext(req.Query, req.OpenFiles, req.CursorFile, int(req.CursorLine))

	// 2. Compress prompt
	maxToks := 4096
	if req.MaxTokens > 0 {
		maxToks = int(req.MaxTokens)
	}
	comp := compressor.NewCompressor(maxToks, s.platform.Cache)
	prompt, stats := comp.Compress(ranked)

	// 3. Extract included symbol names
	var included []string
	for _, r := range ranked {
		included = append(included, r.Symbol.ID)
	}

	savings := stats["savings_percent"].(float64)
	rawBytes := stats["raw_bytes"].(int)
	optBytes := stats["optimized_bytes"].(int)

	return &pb.OptimizeResponse{
		CompressedPrompt: prompt,
		TokenSavings:     savings,
		RawTokens:        int32(rawBytes / 4), // rough token estimation
		OptimizedTokens:  int32(optBytes / 4),
		IncludedSymbols:  included,
	}, nil
}

func (s *GRPCServer) Chat(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	startTime := time.Now()

	// Setup provider client
	prov, err := provider.NewProvider(req.Provider, provider.ClientConfig{
		APIKey:   s.platform.getAPIKey(req.Provider),
		Endpoint: s.platform.getEndpoint(req.Provider),
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider config: %v", err)
	}

	// 1. Generate query embedding for semantic cache lookup
	embedding, err := prov.GenerateEmbedding(ctx, req.Query)
	if err != nil {
		// Log embedding generation warning, proceed without cache
		fmt.Printf("Warning: embedding generation failed: %v\n", err)
	}

	// 2. Extract current hashes of open files
	currentHashes := make(map[string]string)
	for _, filePath := range req.OpenFiles {
		if content, err := os.ReadFile(filePath); err == nil {
			currentHashes[filePath] = parser.GetSHA256(content)
		}
	}

	// 3. Check semantic cache
	if len(embedding) > 0 {
		if cachedResp, hit := s.platform.Cache.GetSemanticCache(ctx, embedding, currentHashes, 0.92); hit {
			duration := time.Since(startTime)
			// Log audit for cache hit
			_ = s.platform.DB.LogAudit(database.AuditLog{
				UserID:          "grpc_user",
				Provider:        req.Provider,
				Model:           req.Model,
				RawTokens:       1000, // Dummy estimate
				OptimizedTokens: 0,    // 100% savings
				SavingsPercent:  100.0,
				DurationMs:      int(duration.Milliseconds()),
				Timestamp:       time.Now(),
			})

			return &pb.ChatResponse{
				Response:        cachedResp,
				OptimizedTokens: 0,
				RawTokens:       0,
				TokenSavings:    100.0,
				FromCache:       true,
			}, nil
		}
	}

	// 4. Optimize context
	optRes, err := s.Optimize(ctx, &pb.OptimizeRequest{
		Query:      req.Query,
		OpenFiles:  req.OpenFiles,
		CursorFile: req.CursorFile,
		CursorLine: req.CursorLine,
		RepoPath:   req.RepoPath,
		MaxTokens:  req.MaxTokens,
	})
	if err != nil {
		return nil, err
	}

	// 5. Mask sensitive data in prompt
	maskedPrompt, _ := s.platform.Masker.Mask(optRes.CompressedPrompt)

	// 6. Construct complete prompt context
	finalPrompt := fmt.Sprintf("%s\n\nUser Question:\n%s", maskedPrompt, req.Query)

	// 7. Generate response from LLM
	llmResponse, err := prov.GenerateResponse(ctx, req.Model, finalPrompt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "LLM provider error: %v", err)
	}

	// 8. Update semantic cache
	if len(embedding) > 0 {
		s.platform.Cache.SetSemanticCache(ctx, req.Query, embedding, llmResponse, currentHashes)
	}

	// 9. Log auditing
	duration := time.Since(startTime)
	_ = s.platform.DB.LogAudit(database.AuditLog{
		UserID:          "grpc_user",
		Provider:        req.Provider,
		Model:           req.Model,
		RawTokens:       int(optRes.RawTokens),
		OptimizedTokens: int(optRes.OptimizedTokens),
		SavingsPercent:  optRes.TokenSavings,
		DurationMs:      int(duration.Milliseconds()),
		Timestamp:       time.Now(),
	})

	return &pb.ChatResponse{
		Response:        llmResponse,
		OptimizedTokens: optRes.OptimizedTokens,
		RawTokens:       optRes.RawTokens,
		TokenSavings:    optRes.TokenSavings,
		FromCache:       false,
	}, nil
}

func (s *GRPCServer) Mask(ctx context.Context, req *pb.MaskRequest) (*pb.MaskResponse, error) {
	masked, count := s.platform.Masker.Mask(req.Text)
	return &pb.MaskResponse{
		MaskedText:  masked,
		ItemsMasked: int32(count),
	}, nil
}

// StartGRPCServer launches the gRPC listener.
func StartGRPCServer(p *Platform, port string) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	s := grpc.NewServer()
	pb.RegisterContextServiceServer(s, NewGRPCServer(p))

	go func() {
		fmt.Printf("Starting gRPC server on port %s...\n", port)
		if err := s.Serve(lis); err != nil {
			fmt.Printf("gRPC server stopped: %v\n", err)
		}
	}()

	return s, nil
}

// ==========================================
// REST HTTP Server Implementation
// ==========================================
type RESTServer struct {
	platform *Platform
}

func NewRESTServer(p *Platform) *RESTServer {
	return &RESTServer{platform: p}
}

func (s *RESTServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RepoPath string `json:"repo_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	files, symbols, err := s.platform.IndexRepository(r.Context(), body.RepoPath)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"files_indexed":   files,
		"symbols_indexed": symbols,
	})
}

func (s *RESTServer) handleOptimize(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query      string   `json:"query"`
		OpenFiles  []string `json:"open_files"`
		CursorFile string   `json:"cursor_file"`
		CursorLine int      `json:"cursor_line"`
		MaxTokens  int      `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	relevance := ranker.NewRelevanceEngine(s.platform.GraphEngine.Graph)
	ranked := relevance.RankContext(body.Query, body.OpenFiles, body.CursorFile, body.CursorLine)

	maxToks := 4096
	if body.MaxTokens > 0 {
		maxToks = body.MaxTokens
	}
	comp := compressor.NewCompressor(maxToks, s.platform.Cache)
	prompt, stats := comp.Compress(ranked)

	var included []string
	for _, r := range ranked {
		included = append(included, r.Symbol.ID)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"compressed_prompt": prompt,
		"token_savings":     stats["savings_percent"],
		"raw_tokens":        stats["raw_bytes"].(int) / 4,
		"optimized_tokens":  stats["optimized_bytes"].(int) / 4,
		"included_symbols":  included,
	})
}

func (s *RESTServer) handleChat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider   string   `json:"provider"`
		Model      string   `json:"model"`
		Query      string   `json:"query"`
		OpenFiles  []string `json:"open_files"`
		CursorFile string   `json:"cursor_file"`
		CursorLine int      `json:"cursor_line"`
		RepoPath   string   `json:"repo_path"`
		MaxTokens  int      `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Reuse gRPC logic
	gs := NewGRPCServer(s.platform)
	resp, err := gs.Chat(r.Context(), &pb.ChatRequest{
		Provider:   body.Provider,
		Model:      body.Model,
		Query:      body.Query,
		OpenFiles:  body.OpenFiles,
		CursorFile: body.CursorFile,
		CursorLine: int32(body.CursorLine),
		RepoPath:   body.RepoPath,
		MaxTokens:  int32(body.MaxTokens),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func (s *RESTServer) handleMask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	masked, count := s.platform.Masker.Mask(body.Text)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"masked_text":  masked,
		"items_masked": count,
	})
}

func (s *RESTServer) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	content, exists := s.platform.Cache.GetCCR(body.Key)
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Key not found in CCR cache",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"original_content": content,
	})
}

func (s *RESTServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *RESTServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Standard Prometheus-style metrics stub
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("# HELP contextiq_requests_total Total requests processed.\n"))
	w.Write([]byte("# TYPE contextiq_requests_total counter\n"))
	w.Write([]byte("contextiq_requests_total 1\n"))
}

// StartRESTServer launches HTTP server.
func StartRESTServer(p *Platform, port string) (*http.Server, error) {
	rest := NewRESTServer(p)
	mux := http.NewServeMux()

	// Map API routes (Go 1.22+ method routing)
	mux.HandleFunc("POST /v1/index", rest.handleIndex)
	mux.HandleFunc("POST /v1/optimize", rest.handleOptimize)
	mux.HandleFunc("POST /v1/chat", rest.handleChat)
	mux.HandleFunc("POST /v1/mask", rest.handleMask)
	mux.HandleFunc("POST /v1/retrieve", rest.handleRetrieve)
	mux.HandleFunc("GET /v1/health", rest.handleHealth)
	mux.HandleFunc("GET /v1/metrics", rest.handleMetrics)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		fmt.Printf("Starting HTTP REST server on port %s...\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server stopped: %v\n", err)
		}
	}()

	return server, nil
}
