package cache

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/example/contextiq/internal/model"
)

// SemanticCacheEntry stores a cached prompt and its LLM response.
type SemanticCacheEntry struct {
	Query      string            `json:"query"`
	Response   string            `json:"response"`
	Embedding  []float32         `json:"embedding"`
	FileHashes map[string]string `json:"file_hashes"` // FilePath -> SHA256 hash at cache time
	CreatedAt  time.Time         `json:"created_at"`
}

// CacheManager handles local/distributed caching for ASTs, LLM responses, and CCR.
type CacheManager struct {
	mu            sync.RWMutex
	contextCache  map[string]*model.FileNode  // FilePath -> FileNode
	semanticCache []SemanticCacheEntry        // Slice of cached queries for local vector lookup
	ccrCache      map[string]string           // Hash -> Original Body (CCR)
}

// NewCacheManager creates a new CacheManager.
func NewCacheManager() *CacheManager {
	return &CacheManager{
		contextCache:  make(map[string]*model.FileNode),
		semanticCache: make([]SemanticCacheEntry, 0),
		ccrCache:      make(map[string]string),
	}
}

// SetCCR caches uncompressed original body by its hash.
func (c *CacheManager) SetCCR(hash, body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ccrCache[hash] = body
}

// GetCCR retrieves cached uncompressed original body by its hash.
func (c *CacheManager) GetCCR(hash string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	body, exists := c.ccrCache[hash]
	return body, exists
}

// GetContext retrieves a cached FileNode if SHA matches.
func (c *CacheManager) GetContext(filePath, currentSHA string) (*model.FileNode, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	node, exists := c.contextCache[filePath]
	if !exists {
		return nil, false
	}

	if node.SHA256 != currentSHA {
		return nil, false // SHA mismatch (file changed)
	}

	return node, true
}

// SetContext caches a parsed FileNode.
func (c *CacheManager) SetContext(filePath string, node *model.FileNode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.contextCache[filePath] = node
}

// SetSemanticCache adds a new LLM response to the semantic cache.
func (c *CacheManager) SetSemanticCache(ctx context.Context, query string, embedding []float32, response string, fileHashes map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.semanticCache = append(c.semanticCache, SemanticCacheEntry{
		Query:      query,
		Response:   response,
		Embedding:  embedding,
		FileHashes: fileHashes,
		CreatedAt:  time.Now(),
	})
}

// GetSemanticCache searches for a semantically similar query where referenced files haven't changed.
func (c *CacheManager) GetSemanticCache(ctx context.Context, queryEmbedding []float32, currentHashes map[string]string, similarityThreshold float64) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(queryEmbedding) == 0 || len(c.semanticCache) == 0 {
		return "", false
	}

	bestScore := -1.0
	var bestMatch *SemanticCacheEntry

	for i := range c.semanticCache {
		entry := &c.semanticCache[i]
		score := CosineSimilarity(queryEmbedding, entry.Embedding)
		if score > bestScore {
			bestScore = score
			bestMatch = entry
		}
	}

	if bestScore >= similarityThreshold && bestMatch != nil {
		// Verify file integrity: if any file has changed since the cache was created, invalidate it
		for path, cachedHash := range bestMatch.FileHashes {
			currHash, exists := currentHashes[path]
			if !exists || currHash != cachedHash {
				return "", false // File changed or missing, cache invalid
			}
		}
		return bestMatch.Response, true
	}

	return "", false
}

// CosineSimilarity computes cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
