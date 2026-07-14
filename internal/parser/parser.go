package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/example/contextiq/internal/model"
)

// Parser defines the interface for parsing AST and symbols from source code.
type Parser interface {
	Parse(filePath string, content []byte) (*model.FileNode, error)
}

// GetSHA256 computes the SHA256 hash of file content.
func GetSHA256(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// DetectLanguage detects programming language from file path.
func DetectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".jsx":
		return "javascriptreact"
	case ".cs":
		return "csharp"
	case ".kt", ".kts":
		return "kotlin"
	default:
		return "unknown"
	}
}
