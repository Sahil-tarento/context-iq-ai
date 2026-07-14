package cache

import (
	"context"
	"testing"

	"github.com/example/contextiq/internal/model"
)

func TestContextCache(t *testing.T) {
	mgr := NewCacheManager()

	node := &model.FileNode{
		FilePath: "main.go",
		SHA256:   "hash123",
	}

	mgr.SetContext("main.go", node)

	// Hit
	n, ok := mgr.GetContext("main.go", "hash123")
	if !ok || n.FilePath != "main.go" {
		t.Error("expected context cache hit")
	}

	// Miss (modified file)
	_, ok = mgr.GetContext("main.go", "hash999")
	if ok {
		t.Error("expected context cache miss due to SHA mismatch")
	}

	// Miss (non-existent file)
	_, ok = mgr.GetContext("other.go", "hash123")
	if ok {
		t.Error("expected context cache miss")
	}
}

func TestSemanticCache(t *testing.T) {
	mgr := NewCacheManager()
	ctx := context.Background()

	query := "How to calculate sqrt in go?"
	embedding := []float32{1.0, 0.0, 0.0} // Unit vector
	response := "Use math.Sqrt(x)"
	hashes := map[string]string{
		"math.go": "sha_math_v1",
	}

	mgr.SetSemanticCache(ctx, query, embedding, response, hashes)

	// Exact match search
	searchEmbedding := []float32{1.0, 0.0, 0.0}
	currentHashes := map[string]string{
		"math.go": "sha_math_v1",
	}

	resp, hit := mgr.GetSemanticCache(ctx, searchEmbedding, currentHashes, 0.95)
	if !hit || resp != response {
		t.Errorf("expected semantic cache hit, got hit=%t, resp=%s", hit, resp)
	}

	// High similarity match search (cosine = 0.96)
	similarEmbedding := []float32{0.96, 0.28, 0.0} // length is ~1, dot product with embedding is 0.96
	resp, hit = mgr.GetSemanticCache(ctx, similarEmbedding, currentHashes, 0.95)
	if !hit || resp != response {
		t.Errorf("expected semantic cache hit on similar vector, got hit=%t, resp=%s", hit, resp)
	}

	// Invalidation due to file modification
	modifiedHashes := map[string]string{
		"math.go": "sha_math_v2", // File changed!
	}
	_, hit = mgr.GetSemanticCache(ctx, searchEmbedding, modifiedHashes, 0.95)
	if hit {
		t.Error("expected cache miss because the referenced file hash changed")
	}
}
