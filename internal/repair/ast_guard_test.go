package repair

import (
	"strings"
	"testing"
)

func TestASTSyntaxGuardGoValid(t *testing.T) {
	guard := NewASTSyntaxGuard()
	code := `package main

import "fmt"

func main() {
	fmt.Println("Hello World")
}
`
	valid, errDetail := guard.ValidateSyntax("main.go", code)
	if !valid || errDetail != "" {
		t.Fatalf("expected valid Go syntax, got valid=%v, errDetail=%s", valid, errDetail)
	}
}

func TestASTSyntaxGuardGoInvalid(t *testing.T) {
	guard := NewASTSyntaxGuard()
	code := `package main

func main() {
	fmt.Println("Broken"
`
	valid, errDetail := guard.ValidateSyntax("main.go", code)
	if valid {
		t.Fatal("expected invalid Go syntax")
	}
	if !strings.Contains(errDetail, "Syntax error in Go file") {
		t.Fatalf("unexpected error detail: %s", errDetail)
	}
}

func TestASTSyntaxGuardJSON(t *testing.T) {
	guard := NewASTSyntaxGuard()

	valid, _ := guard.ValidateSyntax("config.json", `{"key": "value"}`)
	if !valid {
		t.Fatal("expected valid JSON")
	}

	validBad, errDetail := guard.ValidateSyntax("config.json", `{"key": "value"`)
	if validBad || !strings.Contains(errDetail, "Invalid JSON format") {
		t.Fatalf("expected invalid JSON detection, got errDetail: %s", errDetail)
	}
}

func TestASTSyntaxGuardBracketBalance(t *testing.T) {
	guard := NewASTSyntaxGuard()

	// Valid JS
	valid, _ := guard.ValidateSyntax("app.js", "function foo() { return [1, 2, 3]; }")
	if !valid {
		t.Fatal("expected valid JS bracket balance")
	}

	// Unclosed bracket
	validUnclosed, errDetail := guard.ValidateSyntax("app.js", "function foo() { return [1, 2, 3];")
	if validUnclosed || !strings.Contains(errDetail, "Unclosed bracket") {
		t.Fatalf("expected unclosed bracket error, got: %s", errDetail)
	}

	// Unmatched closing bracket
	validUnmatched, errDetail2 := guard.ValidateSyntax("app.js", "function foo() { return [1, 2, 3]; }}")
	if validUnmatched || !strings.Contains(errDetail2, "Unmatched closing bracket") {
		t.Fatalf("expected unmatched closing bracket error, got: %s", errDetail2)
	}
}
