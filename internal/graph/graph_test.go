package graph

import (
	"testing"

	"github.com/example/contextiq/internal/model"
)

func TestGraphEngine_LinkAndTraverse(t *testing.T) {
	engine := NewGraphEngine()

	// Add file 1 (defines Calculate)
	file1 := &model.FileNode{
		FilePath: "/workspace/math.go",
		Language: "go",
		Imports:  []string{},
		Symbols: map[string]model.Symbol{
			"/workspace/math.go:Calculate": {
				ID:        "/workspace/math.go:Calculate",
				Name:      "Calculate",
				Type:      model.SymbolFunction,
				FilePath:  "/workspace/math.go",
				Body:      "func Calculate(x int) int { return x * x }",
				LineStart: 1,
				LineEnd:   3,
			},
		},
	}
	engine.AddFileNode(file1)

	// Add file 2 (calls Calculate)
	file2 := &model.FileNode{
		FilePath: "/workspace/main.go",
		Language: "go",
		Imports:  []string{"/workspace/math.go"},
		Symbols: map[string]model.Symbol{
			"/workspace/main.go:Main": {
				ID:           "/workspace/main.go:Main",
				Name:         "Main",
				Type:         model.SymbolFunction,
				FilePath:     "/workspace/main.go",
				Body:         "func Main() { val := Calculate(10) }",
				Dependencies: []string{"Calculate"},
				LineStart:    1,
				LineEnd:      3,
			},
		},
	}
	engine.AddFileNode(file2)

	// Link symbols
	engine.LinkSymbols()

	// Verify Main depends on Calculate
	mainSym := engine.Graph.Symbols["/workspace/main.go:Main"]
	if len(mainSym.Dependencies) != 1 || mainSym.Dependencies[0] != "/workspace/math.go:Calculate" {
		t.Errorf("expected Main to depend on Calculate, got deps: %v", mainSym.Dependencies)
	}

	// Traverse from Main
	visited := engine.TraverseRelatedSymbols([]string{"/workspace/main.go:Main"}, 2)
	dist, exists := visited["/workspace/math.go:Calculate"]
	if !exists {
		t.Fatalf("expected Calculate to be traversed")
	}
	if dist != 1 {
		t.Errorf("expected Calculate distance to be 1, got %d", dist)
	}
}
