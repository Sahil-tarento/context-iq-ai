package compressor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/example/contextiq/internal/cache"
	"github.com/example/contextiq/internal/ranker"
)

// Compressor compresses prompt context to save tokens.
type Compressor struct {
	MaxTokens int
	Cache     *cache.CacheManager // Optional cache for CCR (reversible compression)
}

// NewCompressor creates a new Compressor.
func NewCompressor(maxTokens int, cacheMgr *cache.CacheManager) *Compressor {
	return &Compressor{MaxTokens: maxTokens, Cache: cacheMgr}
}

// Compress takes ranked symbols and builds an optimized context prompt.
// Highly ranked symbols are included with full body, while lower ranked symbols in the same files
// are compressed into signatures/skeletons to save tokens.
func (c *Compressor) Compress(ranked []ranker.RankedSymbol) (string, map[string]interface{}) {
	if len(ranked) == 0 {
		return "", map[string]interface{}{
			"raw_bytes":       0,
			"optimized_bytes": 0,
			"savings_percent": 0.0,
			"included_count":  0,
		}
	}

	// 1. Group symbols by file path
	fileSymbols := make(map[string][]ranker.RankedSymbol)
	for _, rs := range ranked {
		fileSymbols[rs.Symbol.FilePath] = append(fileSymbols[rs.Symbol.FilePath], rs)
	}

	var sb strings.Builder
	sb.WriteString("Below is the relevant codebase context optimized for this request.\n")
	sb.WriteString("NOTE: Less relevant method/function bodies are optimized out to save tokens.\n")
	sb.WriteString("To retrieve the original full implementation of any optimized block, use the tool 'contextiq_retrieve' with its respective CCR Key.\n\n")

	rawTotalBytes := 0
	optimizedTotalBytes := 0
	includedCount := 0

	for filePath, symbols := range fileSymbols {
		sb.WriteString(fmt.Sprintf("### File: %s\n", filePath))
		sb.WriteString("```" + getMarkdownLanguage(filePath) + "\n")

		// Sort symbols in this file by line number so we rebuild the file in order
		sortSymbolsByLine(symbols)

		for _, rs := range symbols {
			sym := rs.Symbol
			rawTotalBytes += len(sym.Body)
			includedCount++

			// Check if we include the full body or only signature/skeleton
			// Threshold: top ranked symbols (score >= 0.8) or distance <= 1 gets full body
			includeFullBody := rs.Score >= 0.8 || rs.Distance == 0

			var rendered string
			if includeFullBody {
				rendered = sym.Body
			} else {
				// Compute hash of the body for CCR (reversible context compression)
				hasher := sha256.New()
				hasher.Write([]byte(sym.Body))
				hashVal := hex.EncodeToString(hasher.Sum(nil))[:12]

				if c.Cache != nil {
					c.Cache.SetCCR(hashVal, sym.Body)
				}

				// Strip body, output only signature and a skeleton placeholder with CCR key
				commentMarker := fmt.Sprintf("// ... method body optimized out (CCR Key: %s)", hashVal)
				if getMarkdownLanguage(filePath) == "python" {
					commentMarker = fmt.Sprintf("# ... method body optimized out (CCR Key: %s)", hashVal)
				}
				rendered = fmt.Sprintf("%s {\n    %s\n}", sym.Signature, commentMarker)
				if getMarkdownLanguage(filePath) == "python" {
					rendered = fmt.Sprintf("%s\n    %s", sym.Signature, commentMarker)
				}
			}

			// Clean whitespace & comments if requested (we can do a simple line cleaning)
			rendered = cleanWhitespaceAndComments(rendered, filePath)

			sb.WriteString(rendered)
			sb.WriteString("\n\n")
			optimizedTotalBytes += len(rendered)
		}

		sb.WriteString("```\n\n")
	}

	compressedPrompt := sb.String()

	savingsPercent := 0.0
	if rawTotalBytes > 0 {
		savingsPercent = float64(rawTotalBytes-optimizedTotalBytes) / float64(rawTotalBytes) * 100.0
		if savingsPercent < 0 {
			savingsPercent = 0 // prevent negative savings due to formatting overhead on tiny files
		}
	}

	stats := map[string]interface{}{
		"raw_bytes":       rawTotalBytes,
		"optimized_bytes": optimizedTotalBytes,
		"savings_percent": savingsPercent,
		"included_count":  includedCount,
	}

	return compressedPrompt, stats
}

// sortSymbolsByLine sorts ranked symbols by their starting line number.
func sortSymbolsByLine(symbols []ranker.RankedSymbol) {
	for i := 0; i < len(symbols); i++ {
		for j := i + 1; j < len(symbols); j++ {
			if symbols[i].Symbol.LineStart > symbols[j].Symbol.LineStart {
				symbols[i], symbols[j] = symbols[j], symbols[i]
			}
		}
	}
}

// getMarkdownLanguage maps file path to markdown code block language.
func getMarkdownLanguage(filePath string) string {
	ext := strings.ToLower(filePath[strings.LastIndex(filePath, ".")+1:])
	switch ext {
	case "go":
		return "go"
	case "py":
		return "python"
	case "java":
		return "java"
	case "js", "jsx":
		return "javascript"
	case "ts", "tsx":
		return "typescript"
	case "cs":
		return "csharp"
	case "kt":
		return "kotlin"
	default:
		return ""
	}
}

// cleanWhitespaceAndComments performs basic comment and whitespace trimming.
func cleanWhitespaceAndComments(code string, filePath string) string {
	lines := strings.Split(code, "\n")
	var cleaned []string
	ext := getMarkdownLanguage(filePath)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue // remove empty lines
		}

		// Remove full-line comments (except for skeleton placeholders)
		if ext == "python" {
			if strings.HasPrefix(trimmed, "#") && !strings.Contains(trimmed, "optimized out") {
				continue
			}
		} else {
			if strings.HasPrefix(trimmed, "//") && !strings.Contains(trimmed, "optimized out") {
				continue
			}
		}

		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}
