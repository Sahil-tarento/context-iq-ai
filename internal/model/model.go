package model

// SymbolType defines the category of an AST symbol.
type SymbolType string

const (
	SymbolClass     SymbolType = "class"
	SymbolInterface SymbolType = "interface"
	SymbolMethod    SymbolType = "method"
	SymbolFunction  SymbolType = "function"
	SymbolVariable  SymbolType = "variable"
	SymbolImport    SymbolType = "import"
	SymbolStruct    SymbolType = "struct"
)

// Symbol represents an AST symbol extracted from code.
type Symbol struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         SymbolType `json:"type"`
	FilePath     string     `json:"file_path"`
	LineStart    int        `json:"line_start"`
	LineEnd      int        `json:"line_end"`
	Signature    string     `json:"signature"`
	Body         string     `json:"body"`
	Dependencies []string   `json:"dependencies"` // IDs of other symbols this symbol references/depends on
}

// FileNode represents a single code file within a project.
type FileNode struct {
	FilePath string            `json:"file_path"`
	SHA256   string            `json:"sha256"`
	Language string            `json:"language"`
	Symbols  map[string]Symbol `json:"symbols"`  // Name/ID to Symbol mapping
	Imports  []string          `json:"imports"`  // Imported files/modules
}

// DependencyGraph represents the workspace-wide code relationships.
type DependencyGraph struct {
	Files   map[string]*FileNode `json:"files"`
	Symbols map[string]*Symbol   `json:"symbols"`
}

// NewDependencyGraph creates an empty dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Files:   make(map[string]*FileNode),
		Symbols: make(map[string]*Symbol),
	}
}
