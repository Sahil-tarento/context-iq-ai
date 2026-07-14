package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/contextiq/internal/config"
	"github.com/example/contextiq/internal/pb"
)

func TestPlatform_EndToEnd(t *testing.T) {
	// 1. Create temporary directory representing a workspace
	tempDir, err := os.MkdirTemp("", "contextiq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create math.go
	mathCode := `package geom

func CalcArea(w, h float64) float64 {
	// Computes rectangle area
	return w * h
}

func CalcPerimeter(w, h float64) float64 {
	return 2 * (w + h)
}
`
	geomDir := filepath.Join(tempDir, "geom")
	if err := os.MkdirAll(geomDir, 0755); err != nil {
		t.Fatalf("failed to create geom dir: %v", err)
	}
	err = os.WriteFile(filepath.Join(geomDir, "math.go"), []byte(mathCode), 0644)
	if err != nil {
		t.Fatalf("failed to write math.go: %v", err)
	}

	// Create app.go which depends on geom/math
	appCode := `package main

import "geom"

func Main() {
	area := geom.CalcArea(10.0, 5.0)
	println(area)
}
`
	err = os.WriteFile(filepath.Join(tempDir, "app.go"), []byte(appCode), 0644)
	if err != nil {
		t.Fatalf("failed to write app.go: %v", err)
	}

	// 2. Initialize Platform
	cfg := config.LoadConfig()
	cfg.DefaultProv = "mock"
	cfg.DefaultModel = "mock-model"

	platform, err := NewPlatform(cfg)
	if err != nil {
		t.Fatalf("failed to initialize platform: %v", err)
	}

	ctx := context.Background()

	// 3. Index Repository
	files, symbols, err := platform.IndexRepository(ctx, tempDir)
	if err != nil {
		t.Fatalf("indexing failed: %v", err)
	}

	if files != 2 {
		t.Errorf("expected 2 files indexed, got %d", files)
	}

	if symbols < 3 {
		t.Errorf("expected at least 3 symbols, got %d", symbols)
	}

	// 4. Test Optimize via gRPC Server Interface
	gs := NewGRPCServer(platform)
	optRes, err := gs.Optimize(ctx, &pb.OptimizeRequest{
		Query:      "How does Area calculation work?",
		OpenFiles:  []string{filepath.Join(tempDir, "app.go")},
		CursorFile: filepath.Join(tempDir, "app.go"),
		CursorLine: 5,
		MaxTokens:  1024,
	})
	if err != nil {
		t.Fatalf("optimize failed: %v", err)
	}

	if !strings.Contains(optRes.CompressedPrompt, "CalcArea") {
		t.Error("expected optimized prompt to contain CalcArea")
	}

	if optRes.TokenSavings <= 0.0 {
		t.Errorf("expected positive savings, got %f", optRes.TokenSavings)
	}

	// 5. Test Chat routing
	chatRes, err := gs.Chat(ctx, &pb.ChatRequest{
		Provider:   "mock",
		Model:      "mock-model",
		Query:      "Show me CalcArea implementation",
		OpenFiles:  []string{filepath.Join(tempDir, "app.go")},
		CursorFile: filepath.Join(tempDir, "app.go"),
		CursorLine: 5,
		RepoPath:   tempDir,
		MaxTokens:  1024,
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}

	if chatRes.FromCache {
		t.Error("expected first chat call to miss semantic cache")
	}

	if !strings.Contains(chatRes.Response, "[Mock Response for mock-model]") {
		t.Errorf("unexpected chat response: %q", chatRes.Response)
	}

	// 6. Test Semantic Cache Hit (same query)
	chatRes2, err := gs.Chat(ctx, &pb.ChatRequest{
		Provider:   "mock",
		Model:      "mock-model",
		Query:      "Show me CalcArea implementation",
		OpenFiles:  []string{filepath.Join(tempDir, "app.go")},
		CursorFile: filepath.Join(tempDir, "app.go"),
		CursorLine: 5,
		RepoPath:   tempDir,
		MaxTokens:  1024,
	})
	if err != nil {
		t.Fatalf("chat 2 failed: %v", err)
	}

	if !chatRes2.FromCache {
		t.Error("expected second identical chat call to hit semantic cache")
	}

	if chatRes2.Response != chatRes.Response {
		t.Errorf("expected cached response to match original, got: %q", chatRes2.Response)
	}
}
