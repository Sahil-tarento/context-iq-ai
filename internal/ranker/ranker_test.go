package ranker

import (
	"testing"

	"github.com/example/contextiq/internal/model"
)

func TestRelevanceEngine_RankContext(t *testing.T) {
	graph := model.NewDependencyGraph()

	// Math calculation symbol
	calcSym := model.Symbol{
		ID:           "/workspace/math.go:Calculate",
		Name:         "Calculate",
		Type:         model.SymbolFunction,
		FilePath:     "/workspace/math.go",
		Body:         "func Calculate(x int) int { return x * x }",
		Signature:    "func Calculate(x int) int",
		LineStart:    1,
		LineEnd:      5,
		Dependencies: []string{},
	}
	graph.Symbols[calcSym.ID] = &calcSym

	// Main symbol calling Calculate
	mainSym := model.Symbol{
		ID:           "/workspace/main.go:Main",
		Name:         "Main",
		Type:         model.SymbolFunction,
		FilePath:     "/workspace/main.go",
		Body:         "func Main() { val := Calculate(10) }",
		Signature:    "func Main()",
		LineStart:    1,
		LineEnd:      5,
		Dependencies: []string{calcSym.ID},
	}
	graph.Symbols[mainSym.ID] = &mainSym

	// Unrelated symbol
	unrelatedSym := model.Symbol{
		ID:           "/workspace/unrelated.go:LogMsg",
		Name:         "LogMsg",
		Type:         model.SymbolFunction,
		FilePath:     "/workspace/unrelated.go",
		Body:         "func LogMsg(s string) { println(s) }",
		Signature:    "func LogMsg(s string)",
		LineStart:    1,
		LineEnd:      3,
		Dependencies: []string{},
	}
	graph.Symbols[unrelatedSym.ID] = &unrelatedSym

	engine := NewRelevanceEngine(graph)

	// User is currently editing main.go on line 2, querying about "Calculate math"
	ranked := engine.RankContext("Calculate math", []string{"/workspace/main.go"}, "/workspace/main.go", 2)

	if len(ranked) == 0 {
		t.Fatalf("expected ranked symbols, got 0")
	}

	// Main symbol should be high score because it contains the cursor (line 2)
	if ranked[0].Symbol.ID != "/workspace/main.go:Main" {
		t.Errorf("expected Main to be first, got %s", ranked[0].Symbol.ID)
	}

	// Calculate should be ranked high because:
	// - Main depends on it (graph distance 1)
	// - Query matches "Calculate"
	foundCalc := false
	for _, r := range ranked {
		if r.Symbol.ID == "/workspace/math.go:Calculate" {
			foundCalc = true
			if r.Distance != 1 {
				t.Errorf("expected Calculate distance to be 1, got %d", r.Distance)
			}
			break
		}
	}
	if !foundCalc {
		t.Error("expected Calculate to be in ranked list")
	}

	// Unrelated should be lower score or not present
	for _, r := range ranked {
		if r.Symbol.ID == "/workspace/unrelated.go:LogMsg" {
			if r.Score >= ranked[0].Score {
				t.Errorf("expected unrelated symbol to have a lower score than Main, got %f vs %f", r.Score, ranked[0].Score)
			}
		}
	}
}
