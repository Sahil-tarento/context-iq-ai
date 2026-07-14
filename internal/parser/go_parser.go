package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/example/contextiq/internal/model"
)

// GoParser implements the Parser interface for Go source files.
type GoParser struct{}

// NewGoParser creates a new GoParser instance.
func NewGoParser() *GoParser {
	return &GoParser{}
}

// Parse extracts symbols from a Go file.
func (p *GoParser) Parse(filePath string, content []byte) (*model.FileNode, error) {
	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	sha := GetSHA256(content)
	fileNode := &model.FileNode{
		FilePath: filePath,
		SHA256:   sha,
		Language: "go",
		Symbols:  make(map[string]model.Symbol),
		Imports:  make([]string, 0),
	}

	// Capture imports
	for _, imp := range fileAST.Imports {
		if imp.Path != nil {
			pathVal := strings.Trim(imp.Path.Value, "\"")
			fileNode.Imports = append(fileNode.Imports, pathVal)
		}
	}

	// Walk AST to capture classes (structs/interfaces) and methods/functions
	ast.Inspect(fileAST, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeSpec:
			// Struct or Interface definitions
			startPos := fset.Position(node.Pos())
			endPos := fset.Position(node.End())

			symType := model.SymbolStruct
			if _, ok := node.Type.(*ast.InterfaceType); ok {
				symType = model.SymbolInterface
			}

			sig := fmt.Sprintf("type %s", node.Name.Name)
			if structType, ok := node.Type.(*ast.StructType); ok {
				_ = structType // Can extract details if needed
				sig = fmt.Sprintf("type %s struct", node.Name.Name)
			} else if _, ok := node.Type.(*ast.InterfaceType); ok {
				sig = fmt.Sprintf("type %s interface", node.Name.Name)
			}

			body := string(content[node.Pos()-1 : node.End()-1])

			sym := model.Symbol{
				ID:           fmt.Sprintf("%s:%s", filePath, node.Name.Name),
				Name:         node.Name.Name,
				Type:         symType,
				FilePath:     filePath,
				LineStart:    startPos.Line,
				LineEnd:      endPos.Line,
				Signature:    sig,
				Body:         body,
				Dependencies: make([]string, 0),
			}
			fileNode.Symbols[sym.ID] = sym

		case *ast.FuncDecl:
			// Functions and methods
			startPos := fset.Position(node.Pos())
			endPos := fset.Position(node.End())

			var recvName string
			symType := model.SymbolFunction
			if node.Recv != nil && len(node.Recv.List) > 0 {
				symType = model.SymbolMethod
				// Extract receiver type
				typeExpr := node.Recv.List[0].Type
				switch t := typeExpr.(type) {
				case *ast.Ident:
					recvName = t.Name
				case *ast.StarExpr:
					if ident, ok := t.X.(*ast.Ident); ok {
						recvName = "*" + ident.Name
					}
				}
			}

			// Format signature
			var sigBuilder strings.Builder
			sigBuilder.WriteString("func ")
			if recvName != "" {
				sigBuilder.WriteString(fmt.Sprintf("(%s) ", recvName))
			}
			sigBuilder.WriteString(node.Name.Name)

			// Simple signature extraction by taking text up to body start
			var sig string
			if node.Body != nil {
				sig = string(content[node.Pos()-1 : node.Body.Lbrace])
				sig = strings.TrimSpace(sig)
			} else {
				sig = sigBuilder.String()
			}

			body := ""
			if node.Body != nil {
				body = string(content[node.Pos()-1 : node.End()-1])
			}

			symName := node.Name.Name
			if recvName != "" {
				cleanRecv := strings.TrimPrefix(recvName, "*")
				symName = fmt.Sprintf("%s.%s", cleanRecv, node.Name.Name)
			}

			symID := fmt.Sprintf("%s:%s", filePath, symName)

			// Find internal dependencies (calls/references)
			var deps []string
			if node.Body != nil {
				ast.Inspect(node.Body, func(innerNode ast.Node) bool {
					if call, ok := innerNode.(*ast.CallExpr); ok {
						switch funExpr := call.Fun.(type) {
						case *ast.Ident:
							// Direct function call (e.g. foo())
							deps = append(deps, funExpr.Name)
						case *ast.SelectorExpr:
							// Method call on package or receiver (e.g. pkg.Foo() or obj.Foo())
							if ident, ok := funExpr.X.(*ast.Ident); ok {
								deps = append(deps, fmt.Sprintf("%s.%s", ident.Name, funExpr.Sel.Name))
							}
						}
					}
					return true
				})
			}

			sym := model.Symbol{
				ID:           symID,
				Name:         symName,
				Type:         symType,
				FilePath:     filePath,
				LineStart:    startPos.Line,
				LineEnd:      endPos.Line,
				Signature:    sig,
				Body:         body,
				Dependencies: deps,
			}
			fileNode.Symbols[sym.ID] = sym
		}
		return true
	})

	return fileNode, nil
}
