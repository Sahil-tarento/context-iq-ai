package parser

import (
	"regexp"
	"strings"

	"github.com/example/contextiq/internal/model"
)

// GeneralParser parses non-Go languages using structural scanning.
type GeneralParser struct{}

// NewGeneralParser creates a new GeneralParser.
func NewGeneralParser() *GeneralParser {
	return &GeneralParser{}
}

// Regex patterns for declarations
var (
	// Imports
	javaImportRegex   = regexp.MustCompile(`^\s*import\s+(?:static\s+)?([\w\.\*]+);`)
	pythonImportRegex = regexp.MustCompile(`^\s*(?:import\s+([\w\s,]+)|from\s+([\w\.]+)\s+import\s+([\w\s,\*]+))`)
	jsImportRegex     = regexp.MustCompile(`^\s*(?:import\s+.*?\s+from\s+['"](.*?)['"]|const\s+.*?\s+=\s+require\(['"](.*?)['"]\))`)
	csImportRegex     = regexp.MustCompile(`^\s*using\s+([\w\.\s=]+);`)

	// Classes
	classRegex = map[string]*regexp.Regexp{
		"java":       regexp.MustCompile(`\b(?:class|interface|enum|record)\s+([A-Za-z0-9_]+)`),
		"python":     regexp.MustCompile(`^\s*class\s+([A-Za-z0-9_]+)`),
		"javascript": regexp.MustCompile(`\bclass\s+([A-Za-z0-9_]+)`),
		"typescript": regexp.MustCompile(`\b(?:class|interface|enum)\s+([A-Za-z0-9_]+)`),
		"csharp":     regexp.MustCompile(`\b(?:class|interface|struct|enum|record)\s+([A-Za-z0-9_]+)`),
		"kotlin":     regexp.MustCompile(`\b(?:class|interface|object|enum\s+class)\s+([A-Za-z0-9_]+)`),
	}

	// Functions / Methods
	funcRegex = map[string]*regexp.Regexp{
		"java":       regexp.MustCompile(`\b(?:public|protected|private|static|\s)+[\w<>\[\]]+\s+([A-Za-z0-9_]+)\s*\([^\)]*\)\s*(?:throws\s+[\w\s,]+)?\s*\{`),
		"python":     regexp.MustCompile(`^\s*def\s+([A-Za-z0-9_]+)\s*\(`),
		"javascript": regexp.MustCompile(`(?:function\s+([A-Za-z0-9_]+)\b|([A-Za-z0-9_]+)\s*\([^\)]*\)\s*\{|\b([A-Za-z0-9_]+)\s*=\s*(?:\([^\)]*\)|[A-Za-z0-9_]+)\s*=>)`),
		"typescript": regexp.MustCompile(`(?:function\s+([A-Za-z0-9_]+)\b|([A-Za-z0-9_]+)\s*\([^\)]*\)\s*(?::\s*[\w<>|]+)?\s*\{|\b([A-Za-z0-9_]+)\s*=\s*(?:\([^\)]*\)|[A-Za-z0-9_]+)\s*=>)`),
		"csharp":     regexp.MustCompile(`\b(?:public|protected|private|static|internal|override|\s)+[\w<>\[\]]+\s+([A-Za-z0-9_]+)\s*\([^\)]*\)\s*(?:\{|=>)`),
		"kotlin":     regexp.MustCompile(`\bfun\s+(?:<[\w\s,]+>\s+)?(?:[\w\.]+\.)?([A-Za-z0-9_]+)\s*\(`),
	}
)

// Parse implements Parser.
func (p *GeneralParser) Parse(filePath string, content []byte) (*model.FileNode, error) {
	lang := DetectLanguage(filePath)
	sha := GetSHA256(content)

	fileNode := &model.FileNode{
		FilePath: filePath,
		SHA256:   sha,
		Language: lang,
		Symbols:  make(map[string]model.Symbol),
		Imports:  make([]string, 0),
	}

	lines := strings.Split(string(content), "\n")

	// 1. Extract imports
	for _, line := range lines {
		lineTrimmed := strings.TrimSpace(line)
		if lineTrimmed == "" {
			continue
		}

		var imp string
		switch lang {
		case "java", "kotlin":
			if m := javaImportRegex.FindStringSubmatch(line); len(m) > 1 {
				imp = m[1]
			}
		case "python":
			if m := pythonImportRegex.FindStringSubmatch(line); len(m) > 1 {
				if m[1] != "" {
					imp = m[1]
				} else if m[2] != "" {
					imp = m[2] + "." + m[3]
				}
			}
		case "javascript", "typescript", "typescriptreact", "javascriptreact":
			if m := jsImportRegex.FindStringSubmatch(line); len(m) > 1 {
				if m[1] != "" {
					imp = m[1]
				} else {
					imp = m[2]
				}
			}
		case "csharp":
			if m := csImportRegex.FindStringSubmatch(line); len(m) > 1 {
				imp = m[1]
			}
		}

		if imp != "" {
			fileNode.Imports = append(fileNode.Imports, strings.TrimSpace(imp))
		}
	}

	// 2. Extract structural symbols (Classes and Functions)
	isBraceLang := lang == "java" || lang == "javascript" || lang == "typescript" || lang == "typescriptreact" || lang == "javascriptreact" || lang == "csharp" || lang == "kotlin"

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check for Class declarations
		if classPat, exists := classRegex[lang]; exists {
			if matches := classPat.FindStringSubmatch(line); len(matches) > 1 {
				className := matches[1]
				startLine := i + 1
				endLine, body := extractBlock(lines, i, isBraceLang)

				symID := filePath + ":" + className
				fileNode.Symbols[symID] = model.Symbol{
					ID:           symID,
					Name:         className,
					Type:         model.SymbolClass,
					FilePath:     filePath,
					LineStart:    startLine,
					LineEnd:      endLine,
					Signature:    strings.TrimSpace(line),
					Body:         body,
					Dependencies: make([]string, 0),
				}
			}
		}

		// Check for Function / Method declarations
		if funcPat, exists := funcRegex[lang]; exists {
			if matches := funcPat.FindStringSubmatch(line); len(matches) > 1 {
				// Find first non-empty match group
				funcName := ""
				for k := 1; k < len(matches); k++ {
					if matches[k] != "" {
						funcName = matches[k]
						break
					}
				}

				if funcName != "" {
					startLine := i + 1
					endLine, body := extractBlock(lines, i, isBraceLang)

					symID := filePath + ":" + funcName
					fileNode.Symbols[symID] = model.Symbol{
						ID:           symID,
						Name:         funcName,
						Type:         model.SymbolFunction,
						FilePath:     filePath,
						LineStart:    startLine,
						LineEnd:      endLine,
						Signature:    strings.TrimSpace(line),
						Body:         body,
						Dependencies: make([]string, 0),
					}
				}
			}
		}
	}

	return fileNode, nil
}

// extractBlock parses curly-braces or Python indentation levels to isolate code blocks.
func extractBlock(lines []string, startIdx int, isBraceLang bool) (int, string) {
	if isBraceLang {
		return extractBraceBlock(lines, startIdx)
	}
	return extractIndentBlock(lines, startIdx)
}

// extractBraceBlock counts curly braces starting from startIdx until balance is 0.
func extractBraceBlock(lines []string, startIdx int) (int, string) {
	braceCount := 0
	foundOpen := false
	var blockLines []string

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		blockLines = append(blockLines, line)

		for _, char := range line {
			if char == '{' {
				braceCount++
				foundOpen = true
			} else if char == '}' {
				braceCount--
			}
		}

		if foundOpen && braceCount <= 0 {
			return i + 1, strings.Join(blockLines, "\n")
		}
	}

	return len(lines), strings.Join(blockLines, "\n")
}

// extractIndentBlock uses indentation level to determine the end of a block (Python).
func extractIndentBlock(lines []string, startIdx int) (int, string) {
	headerLine := lines[startIdx]
	baseIndent := getIndentLevel(headerLine)
	var blockLines []string
	blockLines = append(blockLines, headerLine)

	endIdx := startIdx
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Ignore blank lines and comments in indentation tracking
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			blockLines = append(blockLines, line)
			endIdx = i
			continue
		}

		indent := getIndentLevel(line)
		if indent <= baseIndent {
			// Outdented, block ends
			break
		}

		blockLines = append(blockLines, line)
		endIdx = i
	}

	return endIdx + 1, strings.Join(blockLines, "\n")
}

// getIndentLevel returns count of leading spaces/tabs.
func getIndentLevel(line string) int {
	level := 0
	for _, char := range line {
		if char == ' ' {
			level++
		} else if char == '\t' {
			level += 4 // Treat tab as 4 spaces
		} else {
			break
		}
	}
	return level
}
