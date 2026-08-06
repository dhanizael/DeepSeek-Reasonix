package repair

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ASTSyntaxGuard performs fast, in-memory AST syntax validation for file content before writing.
type ASTSyntaxGuard struct{}

// NewASTSyntaxGuard constructs a syntax guard.
func NewASTSyntaxGuard() *ASTSyntaxGuard {
	return &ASTSyntaxGuard{}
}

// ValidateSyntax inspects the content of a target file based on its extension.
// Returns (isValid, errorDetail). If the extension is unsupported, it returns (true, "").
func (g *ASTSyntaxGuard) ValidateSyntax(filePath string, content string) (bool, string) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return g.validateGoSyntax(filePath, content)
	case ".json":
		return g.validateJSONSyntax(content)
	case ".js", ".ts", ".jsx", ".tsx", ".py":
		return g.validateBracketBalance(content)
	default:
		return true, ""
	}
}

// validateGoSyntax validates Go code using stdlib go/parser.
func (g *ASTSyntaxGuard) validateGoSyntax(filePath string, content string) (bool, string) {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, filePath, content, parser.AllErrors)
	if err != nil {
		return false, fmt.Sprintf("❌ [AST Syntax Guard Rejected Write] Syntax error in Go file (%s):\n%s", filepath.Base(filePath), err.Error())
	}
	return true, ""
}

// validateJSONSyntax validates JSON using stdlib encoding/json.
func (g *ASTSyntaxGuard) validateJSONSyntax(content string) (bool, string) {
	if strings.TrimSpace(content) == "" {
		return true, ""
	}
	var js json.RawMessage
	if err := json.Unmarshal([]byte(content), &js); err != nil {
		return false, fmt.Sprintf("❌ [AST Syntax Guard Rejected Write] Invalid JSON format:\n%s", err.Error())
	}
	return true, ""
}

// validateBracketBalance performs fast bracket matching check for curly braces, parens, and brackets.
func (g *ASTSyntaxGuard) validateBracketBalance(content string) (bool, string) {
	var stack []rune
	line := 1
	col := 1

	for _, ch := range content {
		if ch == '\n' {
			line++
			col = 1
			continue
		}

		switch ch {
		case '{', '(', '[':
			stack = append(stack, ch)
		case '}', ')', ']':
			if len(stack) == 0 {
				return false, fmt.Sprintf("❌ [AST Syntax Guard Rejected Write] Unmatched closing bracket '%c' at line %d, col %d", ch, line, col)
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if !isMatchingPair(top, ch) {
				return false, fmt.Sprintf("❌ [AST Syntax Guard Rejected Write] Mismatched bracket '%c' (expected closing for '%c') at line %d, col %d", ch, top, line, col)
			}
		}
		col++
	}

	if len(stack) > 0 {
		top := stack[len(stack)-1]
		return false, fmt.Sprintf("❌ [AST Syntax Guard Rejected Write] Unclosed bracket '%c' at end of file", top)
	}

	return true, ""
}

func isMatchingPair(open, close rune) bool {
	return (open == '{' && close == '}') ||
		(open == '(' && close == ')') ||
		(open == '[' && close == ']')
}
