package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// AuditLog represents a record of token savings and duration.
type AuditLog struct {
	UserID              string    `json:"user_id"`
	Provider            string    `json:"provider"`
	Model               string    `json:"model"`
	RawTokens           int       `json:"raw_tokens"`
	OptimizedTokens     int       `json:"optimized_tokens"`
	SavingsPercent      float64   `json:"savings_percent"`
	DurationMs          int       `json:"duration_ms"`
	Timestamp           time.Time `json:"timestamp"`
}

// VectorRecord represents an embedding entry in Qdrant or our local fallback.
type VectorRecord struct {
	ID        string    `json:"id"`
	Vector    []float32 `json:"vector"`
	Payload   map[string]interface{} `json:"payload"`
}

// DBEngine controls database connections.
type DBEngine struct {
	SQLDB        *sql.DB
	QdrantURL    string
	UseLocal     bool
	mu           sync.RWMutex
	localVectors map[string][]VectorRecord // Collection -> Records
}

// NewDBEngine creates a DBEngine. If pqConnStr is empty, SQLDB will be nil (using mocks/in-memory).
func NewDBEngine(pqConnStr string, qdrantURL string) (*DBEngine, error) {
	var db *sql.DB
	var err error

	if pqConnStr != "" {
		db, err = sql.Open("postgres", pqConnStr)
		if err != nil {
			return nil, fmt.Errorf("failed to open postgres connection: %w", err)
		}
		// Set connection limits
		db.SetMaxOpenConns(25)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	return &DBEngine{
		SQLDB:        db,
		QdrantURL:    qdrantURL,
		UseLocal:     qdrantURL == "",
		localVectors: make(map[string][]VectorRecord),
	}, nil
}

// InitSQLSchema creates PostgreSQL tables if they don't exist.
func (d *DBEngine) InitSQLSchema() error {
	if d.SQLDB == nil {
		return nil // skip if running in local-only mode
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS repositories (
			id VARCHAR(255) PRIMARY KEY,
			path TEXT NOT NULL,
			branch VARCHAR(100),
			last_indexed_at TIMESTAMP WITH TIME ZONE
		);`,
		`CREATE TABLE IF NOT EXISTS files (
			id VARCHAR(255) PRIMARY KEY,
			repository_id VARCHAR(255) REFERENCES repositories(id) ON DELETE CASCADE,
			relative_path TEXT NOT NULL,
			sha256 VARCHAR(64) NOT NULL,
			language VARCHAR(50) NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS symbols (
			id VARCHAR(255) PRIMARY KEY,
			file_id VARCHAR(255) REFERENCES files(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			line_start INT NOT NULL,
			line_end INT NOT NULL,
			signature TEXT NOT NULL,
			body TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			provider VARCHAR(50) NOT NULL,
			model VARCHAR(100) NOT NULL,
			raw_tokens INT NOT NULL,
			optimized_tokens INT NOT NULL,
			savings_percent DOUBLE PRECISION NOT NULL,
			duration_ms INT NOT NULL,
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL
		);`,
	}

	for _, query := range queries {
		if _, err := d.SQLDB.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}
	return nil
}

// LogAudit records token optimizations in the database.
func (d *DBEngine) LogAudit(log AuditLog) error {
	if d.SQLDB == nil {
		// Mock local log
		return nil
	}

	query := `INSERT INTO audit_logs (user_id, provider, model, raw_tokens, optimized_tokens, savings_percent, duration_ms, timestamp)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := d.SQLDB.Exec(query, log.UserID, log.Provider, log.Model, log.RawTokens, log.OptimizedTokens, log.SavingsPercent, log.DurationMs, log.Timestamp)
	return err
}

// ==========================================
// Vector Operations (Qdrant & Local Fallback)
// ==========================================

// UpsertVectors inserts or updates embeddings in Qdrant or our in-memory fallback.
func (d *DBEngine) UpsertVectors(ctx context.Context, collection string, records []VectorRecord) error {
	if d.UseLocal {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.localVectors[collection] = append(d.localVectors[collection], records...)
		return nil
	}

	// Qdrant REST API Client
	url := fmt.Sprintf("%s/collections/%s/points", d.QdrantURL, collection)

	// Format Qdrant Points payload
	points := make([]map[string]interface{}, len(records))
	for i, r := range records {
		points[i] = map[string]interface{}{
			"id":      r.ID,
			"vector":  r.Vector,
			"payload": r.Payload,
		}
	}

	body, _ := json.Marshal(map[string]interface{}{
		"points": points,
	})

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant upsert failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SearchVectors searches for nearest neighbors.
func (d *DBEngine) SearchVectors(ctx context.Context, collection string, vector []float32, limit int) ([]VectorRecord, error) {
	if d.UseLocal {
		d.mu.RLock()
		defer d.mu.RUnlock()

		records, exists := d.localVectors[collection]
		if !exists || len(records) == 0 {
			return nil, nil
		}

		// Calculate similarity for all local records
		type scoredRecord struct {
			record VectorRecord
			score  float64
		}
		var scored []scoredRecord

		for _, r := range records {
			score := cosineSimilarity(vector, r.Vector)
			scored = append(scored, scoredRecord{record: r, score: score})
		}

		// Sort by score descending
		for i := 0; i < len(scored); i++ {
			for j := i + 1; j < len(scored); j++ {
				if scored[i].score < scored[j].score {
					scored[i], scored[j] = scored[j], scored[i]
				}
			}
		}

		// Truncate to limit
		resultCount := limit
		if len(scored) < limit {
			resultCount = len(scored)
		}

		results := make([]VectorRecord, resultCount)
		for i := 0; i < resultCount; i++ {
			results[i] = scored[i].record
		}
		return results, nil
	}

	// Qdrant REST API Search
	url := fmt.Sprintf("%s/collections/%s/points/search", d.QdrantURL, collection)

	body, _ := json.Marshal(map[string]interface{}{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qdrant search failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var qdrantResp struct {
		Result []struct {
			ID      string                 `json:"id"`
			Payload map[string]interface{} `json:"payload"`
			Score   float64                `json:"score"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&qdrantResp); err != nil {
		return nil, err
	}

	results := make([]VectorRecord, len(qdrantResp.Result))
	for i, r := range qdrantResp.Result {
		results[i] = VectorRecord{
			ID:      r.ID,
			Payload: r.Payload,
		}
	}

	return results, nil
}

// cosineSimilarity helper.
func cosineSimilarity(a, b []float32) float64 {
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
