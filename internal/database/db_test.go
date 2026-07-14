package database

import (
	"context"
	"testing"
	"time"
)

func TestDBEngine_LocalVector(t *testing.T) {
	// Create engine without Postgres connection (local-only / fallback mode)
	engine, err := NewDBEngine("", "")
	if err != nil {
		t.Fatalf("failed to create db engine: %v", err)
	}

	ctx := context.Background()

	// Insert mock vectors
	records := []VectorRecord{
		{
			ID:     "point_1",
			Vector: []float32{1.0, 0.0, 0.0},
			Payload: map[string]interface{}{
				"title": "Go structures",
			},
		},
		{
			ID:     "point_2",
			Vector: []float32{0.0, 1.0, 0.0},
			Payload: map[string]interface{}{
				"title": "Python classes",
			},
		},
	}

	err = engine.UpsertVectors(ctx, "test_collection", records)
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	// Search closest to [0.9, 0.1, 0.0] -> should match point_1
	searchVec := []float32{0.9, 0.1, 0.0}
	results, err := engine.SearchVectors(ctx, "test_collection", searchVec, 1)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}

	if results[0].ID != "point_1" {
		t.Errorf("expected matching ID to be 'point_1', got %s", results[0].ID)
	}

	if results[0].Payload["title"].(string) != "Go structures" {
		t.Errorf("unexpected payload: %v", results[0].Payload)
	}
}

func TestDBEngine_AuditLog(t *testing.T) {
	engine, err := NewDBEngine("", "")
	if err != nil {
		t.Fatalf("failed to create db engine: %v", err)
	}

	log := AuditLog{
		UserID:          "developer_1",
		Provider:        "openai",
		Model:           "gpt-4",
		RawTokens:       1000,
		OptimizedTokens: 250,
		SavingsPercent:  75.0,
		DurationMs:      120,
		Timestamp:       time.Now(),
	}

	err = engine.LogAudit(log)
	if err != nil {
		t.Errorf("failed to log audit: %v", err)
	}
}
