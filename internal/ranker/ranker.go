package ranker

import (
	"math"
	"sort"
	"strings"

	"github.com/example/contextiq/internal/model"
)

// RankedSymbol represents a symbol along with its computed relevance score.
type RankedSymbol struct {
	Symbol   *model.Symbol
	Score    float64
	Distance int
}

// RelevanceEngine ranks symbols for LLM prompt context inclusion.
type RelevanceEngine struct {
	Graph *model.DependencyGraph
}

// NewRelevanceEngine creates a new RelevanceEngine.
func NewRelevanceEngine(g *model.DependencyGraph) *RelevanceEngine {
	return &RelevanceEngine{Graph: g}
}

// RankContext returns the most relevant symbols based on the query and IDE state.
func (e *RelevanceEngine) RankContext(query string, openFiles []string, cursorFile string, cursorLine int) []RankedSymbol {
	var seeds []string
	activeSymbolID := ""

	// 1. Identify active symbol (where cursor is located)
	if cursorFile != "" && cursorLine > 0 {
		for _, sym := range e.Graph.Symbols {
			if sym.FilePath == cursorFile && cursorLine >= sym.LineStart && cursorLine <= sym.LineEnd {
				activeSymbolID = sym.ID
				seeds = append(seeds, sym.ID)
				break
			}
		}
	}

	// 2. Identify symbols in open files
	openFileMap := make(map[string]bool)
	for _, f := range openFiles {
		openFileMap[f] = true
		for _, sym := range e.Graph.Symbols {
			if sym.FilePath == f {
				// Don't duplicate if it was already added as active symbol
				if sym.ID != activeSymbolID {
					seeds = append(seeds, sym.ID)
				}
			}
		}
	}

	// 3. Traverse the dependency graph to find distance from seeds
	distances := make(map[string]int)
	if len(seeds) > 0 {
		distances = e.traverseGraph(seeds, 3)
	}

	// 4. Score all symbols
	var ranked []RankedSymbol
	queryWords := strings.Fields(strings.ToLower(query))

	for id, sym := range e.Graph.Symbols {
		score := 0.0

		// A. Graph Distance Score
		dist, reachable := distances[id]
		if reachable {
			score += 1.0 / float64(dist+1) // dist=0 -> 1.0, dist=1 -> 0.5, dist=2 -> 0.33
		}

		// B. Keyword Relevance Score (TF-IDF style fallback)
		keywordMatchCount := 0
		bodyLower := strings.ToLower(sym.Body)
		nameLower := strings.ToLower(sym.Name)
		sigLower := strings.ToLower(sym.Signature)

		for _, word := range queryWords {
			if len(word) < 3 {
				continue // skip short words
			}
			if strings.Contains(nameLower, word) {
				keywordMatchCount += 5 // high weight for name match
			} else if strings.Contains(sigLower, word) {
				keywordMatchCount += 3 // medium weight for signature match
			} else if strings.Contains(bodyLower, word) {
				keywordMatchCount += 1 // low weight for body match
			}
		}
		if len(queryWords) > 0 {
			score += float64(keywordMatchCount) / float64(len(queryWords))
		}

		// C. IDE State Bonuses
		if id == activeSymbolID {
			score += 1.5 // Big boost for current cursor location
		}
		if openFileMap[sym.FilePath] {
			score += 0.5 // Boost for files open in tabs
		}

		// Filter out very low scoring symbols that are unrelated
		if score > 0 {
			actualDist := -1
			if reachable {
				actualDist = dist
			}
			ranked = append(ranked, RankedSymbol{
				Symbol:   sym,
				Score:    score,
				Distance: actualDist,
			})
		}
	}

	// Sort by score descending, then by name ascending for determinism
	sort.Slice(ranked, func(i, j int) bool {
		if math.Abs(ranked[i].Score-ranked[j].Score) < 1e-9 {
			return ranked[i].Symbol.ID < ranked[j].Symbol.ID
		}
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}

// traverseGraph performs BFS to calculate graph distances from seed IDs.
func (e *RelevanceEngine) traverseGraph(seeds []string, maxDepth int) map[string]int {
	visited := make(map[string]int)
	queue := []string{}

	for _, id := range seeds {
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

		// Forward references
		for _, depID := range sym.Dependencies {
			if _, seen := visited[depID]; !seen {
				visited[depID] = currentDist + 1
				queue = append(queue, depID)
			}
		}

		// Reverse references (what refers to the current symbol)
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
