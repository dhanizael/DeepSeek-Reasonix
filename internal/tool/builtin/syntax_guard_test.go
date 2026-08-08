package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileRejectsInvalidGo(t *testing.T) {
	t.Cleanup(func() { SetASTSyntaxGuardEnabled(true) })
	SetASTSyntaxGuardEnabled(true)

	dir := t.TempDir()
	path := filepath.Join(dir, "broken.go")
	invalid := "package main\n\nfunc main( {\n"
	_, err := (writeFile{}).Execute(context.Background(), argsJSON(t, map[string]any{
		"path":    path,
		"content": invalid,
	}))
	if err == nil {
		t.Fatal("expected syntax rejection for invalid Go")
	}
	if !strings.Contains(err.Error(), "AST Syntax Guard") && !strings.Contains(err.Error(), "Syntax error") {
		t.Fatalf("error should mention AST syntax guard, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("invalid Go must not be written to disk; stat err=%v", statErr)
	}
}

func TestWriteFileAcceptsValidGo(t *testing.T) {
	t.Cleanup(func() { SetASTSyntaxGuardEnabled(true) })
	SetASTSyntaxGuardEnabled(true)

	dir := t.TempDir()
	path := filepath.Join(dir, "ok.go")
	valid := "package main\n\nfunc main() {}\n"
	out, err := (writeFile{}).Execute(context.Background(), argsJSON(t, map[string]any{
		"path":    path,
		"content": valid,
	}))
	if err != nil {
		t.Fatalf("valid Go should write: %v", err)
	}
	if !strings.Contains(out, "wrote") {
		t.Fatalf("unexpected output %q", out)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != valid {
		t.Fatalf("content = %q, want %q", got, valid)
	}
}

func TestWriteFileRejectsInvalidJSON(t *testing.T) {
	t.Cleanup(func() { SetASTSyntaxGuardEnabled(true) })
	SetASTSyntaxGuardEnabled(true)

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	_, err := (writeFile{}).Execute(context.Background(), argsJSON(t, map[string]any{
		"path":    path,
		"content": `{"key": `,
	}))
	if err == nil {
		t.Fatal("expected rejection for invalid JSON")
	}
	if !strings.Contains(err.Error(), "JSON") && !strings.Contains(err.Error(), "AST Syntax Guard") {
		t.Fatalf("error should mention JSON/AST guard, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("invalid JSON must not be written; stat err=%v", statErr)
	}
}

func TestWriteFileAcceptsValidJSON(t *testing.T) {
	t.Cleanup(func() { SetASTSyntaxGuardEnabled(true) })
	SetASTSyntaxGuardEnabled(true)

	dir := t.TempDir()
	path := filepath.Join(dir, "ok.json")
	content := `{"key":"value"}`
	if _, err := (writeFile{}).Execute(context.Background(), argsJSON(t, map[string]any{
		"path":    path,
		"content": content,
	})); err != nil {
		t.Fatalf("valid JSON should write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("content = %q", got)
	}
}

func TestEditFileRejectsInvalidGoResult(t *testing.T) {
	t.Cleanup(func() { SetASTSyntaxGuardEnabled(true) })
	SetASTSyntaxGuardEnabled(true)

	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := (editFile{}).Execute(context.Background(), argsJSON(t, map[string]any{
		"path":       path,
		"old_string": "func main() {}",
		"new_string": "func main( {",
	}))
	if err == nil {
		t.Fatal("expected edit to reject invalid Go result")
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != original {
		t.Fatalf("file must remain unchanged after rejected edit, got %q", got)
	}
}

func TestEditFileAcceptsValidGoResult(t *testing.T) {
	t.Cleanup(func() { SetASTSyntaxGuardEnabled(true) })
	SetASTSyntaxGuardEnabled(true)

	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (editFile{}).Execute(context.Background(), argsJSON(t, map[string]any{
		"path":       path,
		"old_string": "func main() {}",
		"new_string": "func main() {\n\tprintln(\"hi\")\n}",
	})); err != nil {
		t.Fatalf("valid edit should succeed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `println("hi")`) {
		t.Fatalf("edit not applied: %q", got)
	}
}

func TestWriteFileAllowsWhenGuardDisabled(t *testing.T) {
	t.Cleanup(func() { SetASTSyntaxGuardEnabled(true) })
	SetASTSyntaxGuardEnabled(false)

	dir := t.TempDir()
	path := filepath.Join(dir, "broken.go")
	invalid := "package main\n\nfunc main( {\n"
	if _, err := (writeFile{}).Execute(context.Background(), argsJSON(t, map[string]any{
		"path":    path,
		"content": invalid,
	})); err != nil {
		t.Fatalf("disabled guard should allow write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFileNonCodePassthrough(t *testing.T) {
	t.Cleanup(func() { SetASTSyntaxGuardEnabled(true) })
	SetASTSyntaxGuardEnabled(true)

	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	content := "not code at all {{ unmatched"
	if _, err := (writeFile{}).Execute(context.Background(), argsJSON(t, map[string]any{
		"path":    path,
		"content": content,
	})); err != nil {
		t.Fatalf(".txt should not be syntax-checked: %v", err)
	}
}
