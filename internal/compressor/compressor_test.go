package compressor

import (
	"strings"
	"testing"

	"github.com/example/contextiq/internal/model"
	"github.com/example/contextiq/internal/ranker"
)

func TestCompressor(t *testing.T) {
	// A high-ranked symbol
	sym1 := model.Symbol{
		ID:        "main.go:ActiveFunc",
		Name:      "ActiveFunc",
		Type:      model.SymbolFunction,
		FilePath:  "main.go",
		Body:      "func ActiveFunc() {\n    // Inside active function\n    fmt.Println(\"Active\")\n}",
		Signature: "func ActiveFunc()",
		LineStart: 10,
		LineEnd:   14,
	}

	// A low-ranked symbol (should get skeletonized)
	sym2 := model.Symbol{
		ID:        "main.go:HelperFunc",
		Name:      "HelperFunc",
		Type:      model.SymbolFunction,
		FilePath:  "main.go",
		Body:      "func HelperFunc(a int) int {\n    // Unrelated helper logic here\n    b := a * 2\n    return b + 10\n}",
		Signature: "func HelperFunc(a int) int",
		LineStart: 1,
		LineEnd:   6,
	}

	ranked := []ranker.RankedSymbol{
		{
			Symbol:   &sym1,
			Score:    1.5, // High score -> full body
			Distance: 0,
		},
		{
			Symbol:   &sym2,
			Score:    0.3, // Low score -> skeleton
			Distance: 2,
		},
	}

	compressor := NewCompressor(2048, nil)
	prompt, stats := compressor.Compress(ranked)

	if !strings.Contains(prompt, "func ActiveFunc() {") {
		t.Error("expected prompt to contain full ActiveFunc definition")
	}

	if !strings.Contains(prompt, "fmt.Println(\"Active\")") {
		t.Error("expected prompt to contain ActiveFunc body lines")
	}

	if !strings.Contains(prompt, "func HelperFunc(a int) int") {
		t.Error("expected prompt to contain HelperFunc signature")
	}

	if strings.Contains(prompt, "return b + 10") {
		t.Error("expected HelperFunc body to be stripped and not contain body lines")
	}

	if !strings.Contains(prompt, "CCR Key:") {
		t.Error("expected prompt to contain CCR Key for the optimized HelperFunc")
	}

	savings := stats["savings_percent"].(float64)
	if savings <= 0.0 {
		t.Errorf("expected positive savings percentage, got %f", savings)
	}
}
