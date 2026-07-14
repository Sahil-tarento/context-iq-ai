package provider

import (
	"context"
	"math"
	"strings"
	"testing"
)

func TestMockProvider(t *testing.T) {
	p, err := NewProvider("mock", ClientConfig{})
	if err != nil {
		t.Fatalf("failed to create mock provider: %v", err)
	}

	ctx := context.Background()

	// Test GenerateResponse
	prompt := "Test query message"
	resp, err := p.GenerateResponse(ctx, "gpt-4", prompt)
	if err != nil {
		t.Errorf("expected no response error, got %v", err)
	}
	if !strings.Contains(resp, "[Mock Response for gpt-4]") {
		t.Errorf("unexpected response content: %q", resp)
	}

	// Test GenerateEmbedding
	emb, err := p.GenerateEmbedding(ctx, "hello world")
	if err != nil {
		t.Errorf("expected no embedding error, got %v", err)
	}
	if len(emb) != 1536 {
		t.Errorf("expected embedding length of 1536, got %d", len(emb))
	}

	// Verify normalization
	sumSquares := 0.0
	for _, val := range emb {
		sumSquares += float64(val * val)
	}
	norm := math.Sqrt(sumSquares)
	if math.Abs(norm-1.0) > 1e-4 {
		t.Errorf("expected embedding to be normalized (norm=1.0), got norm=%f", norm)
	}
}
