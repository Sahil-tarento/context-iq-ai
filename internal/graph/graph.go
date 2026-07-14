package graph

import (
	"path/filepath"
	"strings"

	"github.com/example/contextiq/internal/model"
)

// GraphEngine manages workspace dependency graphs.
type GraphEngine struct {
	Graph *model.DependencyGraph
}

// NewGraphEngine creates a new GraphEngine.
func NewGraphEngine() *GraphEngine {
	return &GraphEngine{
		Graph: model.NewDependencyGraph(),
	}
}

// AddFileNode adds a parsed file and its symbols to the graph.
func (e *GraphEngine) AddFileNode(node *model.FileNode) {
	e.Graph.Files[node.FilePath] = node
	for _, sym := range node.Symbols {
		s := sym // Create copy
		e.Graph.Symbols[sym.ID] = &s
	}
}

// LinkSymbols resolves dependencies between symbols across the entire workspace.
func (e *GraphEngine) LinkSymbols() {
	// Create a map of symbol name -> Symbol pointer for fast name-based lookup
	nameToSymbols := make(map[string][]*model.Symbol)
	for _, sym := range e.Graph.Symbols {
		// Store simple names (e.g. "Calculate", "Config")
		parts := strings.Split(sym.Name, ".")
		simpleName := parts[len(parts)-1]
		nameToSymbols[simpleName] = append(nameToSymbols[simpleName], sym)
	}

	// Resolve references
	for _, sym := range e.Graph.Symbols {
		resolvedDeps := make(map[string]bool)

		// 1. Resolve based on parsed dependencies (calls/selectors extracted from AST)
		for _, depName := range sym.Dependencies {
			// depName could be a selector like "math.Sqrt" or just "Calculate"
			parts := strings.Split(depName, ".")
			lookupName := parts[len(parts)-1]

			if targets, exists := nameToSymbols[lookupName]; exists {
				for _, target := range targets {
					// Don't depend on itself
					if target.ID == sym.ID {
						continue
					}

					// Verify package import or file proximity
					if e.isImportedOrLocal(sym.FilePath, target.FilePath, depName) {
						resolvedDeps[target.ID] = true
					}
				}
			}
		}

		// 2. Also check if the body references symbol names in scope (lexical fallback)
		// For all simple names in the workspace, check if they appear in the body of the function
		for simpleName, targets := range nameToSymbols {
			if len(targets) == 0 {
				continue
			}

			// Simple word search boundary (avoid matching substrings like "Calculator" for "Cal")
			if strings.Contains(sym.Body, simpleName) {
				for _, target := range targets {
					if target.ID == sym.ID {
						continue
					}

					if e.isImportedOrLocal(sym.FilePath, target.FilePath, simpleName) {
						resolvedDeps[target.ID] = true
					}
				}
			}
		}

		// Convert map to slice
		sym.Dependencies = make([]string, 0, len(resolvedDeps))
		for depID := range resolvedDeps {
			sym.Dependencies = append(sym.Dependencies, depID)
		}
	}
}

// isImportedOrLocal returns true if srcFile imports destFile, or if they are in the same folder.
func (e *GraphEngine) isImportedOrLocal(srcFile, destFile, refName string) bool {
	srcDir := filepath.Dir(srcFile)
	destDir := filepath.Dir(destFile)

	// Case 1: Same directory (package local)
	if srcDir == destDir {
		return true
	}

	// Case 2: Source file imports destination file/package
	srcNode, exists := e.Graph.Files[srcFile]
	if !exists {
		return false
	}

	_, exists = e.Graph.Files[destFile]
	if !exists {
		// Check imports by looking at path matches
		for _, imp := range srcNode.Imports {
			if strings.Contains(destFile, imp) {
				return true
			}
		}
		return false
	}

	// Check directory names in imports
	destPkg := filepath.Base(destDir)
	for _, imp := range srcNode.Imports {
		// e.g. import "math" -> checks if destFile contains "/math/" or ends with "math"
		if imp == destPkg || strings.HasSuffix(imp, "/"+destPkg) || strings.Contains(destFile, imp) {
			return true
		}
	}

	// Fallback: If it's a unique match in the workspace, resolve it
	return false
}

// TraverseRelatedSymbols traverses the dependency graph up to maxDepth starting from a set of seeds.
func (e *GraphEngine) TraverseRelatedSymbols(seedSymbolIDs []string, maxDepth int) map[string]int {
	visited := make(map[string]int) // ID -> distance
	queue := []string{}

	for _, id := range seedSymbolIDs {
		visited[id] = 0
		queue = append(queue, id)
	}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		currentDist := visited[currentID]
		if currentDist >= maxDepth {
			continue
		}

		sym, exists := e.Graph.Symbols[currentID]
		if !exists {
			continue
		}

		// Traverse dependencies
		for _, depID := range sym.Dependencies {
			if _, seen := visited[depID]; !seen {
				visited[depID] = currentDist + 1
				queue = append(queue, depID)
			}
		}

		// Traverse reverse dependencies (what calls current symbol)
		for _, parentSym := range e.Graph.Symbols {
			for _, depID := range parentSym.Dependencies {
				if depID == currentID {
					if _, seen := visited[parentSym.ID]; !seen {
						visited[parentSym.ID] = currentDist + 1
						queue = append(queue, parentSym.ID)
					}
				}
			}
		}
	}

	return visited
}
