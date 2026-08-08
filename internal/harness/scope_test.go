package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectScopedCommandGoPackage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(root, "internal", "foo")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(pkgDir, "foo.go")
	if err := os.WriteFile(file, []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	cmd, scope, ok := d.DetectScopedCommand(root, file)
	if !ok {
		t.Fatal("expected scoped go command")
	}
	if scope != ScopePackage {
		t.Fatalf("scope = %q", scope)
	}
	if cmd != "go test ./internal/foo" {
		t.Fatalf("cmd = %q, want go test ./internal/foo", cmd)
	}
}

func TestVerifyPathUsesScopedGoTest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal passing package
	if err := os.WriteFile(filepath.Join(pkgDir, "x.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "x_test.go"), []byte("package pkg\nimport \"testing\"\nfunc TestOK(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewHarness(DefaultConfig())
	res := h.VerifyPath(context.Background(), root, filepath.Join(pkgDir, "x.go"))
	if !res.Attempted {
		t.Fatal("expected attempt")
	}
	if res.Command != "go test ./pkg" {
		t.Fatalf("command = %q", res.Command)
	}
	if !res.Passed {
		t.Fatalf("expected pass, out=%s err=%v", res.Output, res.Err)
	}
	if res.Scope != ScopePackage {
		t.Fatalf("scope = %q", res.Scope)
	}
}

func TestVerifyPathDoesNotUseFullModuleWhenScoped(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	_ = os.MkdirAll(a, 0o755)
	_ = os.MkdirAll(b, 0o755)
	_ = os.WriteFile(filepath.Join(a, "a.go"), []byte("package a\n"), 0o644)
	// Package b fails tests — must not run when editing a
	_ = os.WriteFile(filepath.Join(b, "b.go"), []byte("package b\n"), 0o644)
	_ = os.WriteFile(filepath.Join(b, "b_test.go"), []byte("package b\nimport \"testing\"\nfunc TestFail(t *testing.T) { t.Fatal(\"no\") }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(a, "a_test.go"), []byte("package a\nimport \"testing\"\nfunc TestOK(t *testing.T) {}\n"), 0o644)

	h := NewHarness(DefaultConfig())
	res := h.VerifyPath(context.Background(), root, filepath.Join(a, "a.go"))
	if !res.Passed {
		t.Fatalf("editing package a should not run b tests: cmd=%s out=%s", res.Command, res.Output)
	}
	if strings.Contains(res.Command, "./...") {
		t.Fatalf("must not use full-module ./... when scoped: %s", res.Command)
	}
}

func TestWorkspaceScopeStillUsesFullCommand(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.Scope = ScopeWorkspace
	h := NewHarness(cfg)
	// Detect only — Verify would run full suite; just check command selection path via empty custom + workspace
	cmd, ok := h.detector.DetectCommand(root)
	if !ok || cmd != "go test ./..." {
		t.Fatalf("workspace detect = %q ok=%v", cmd, ok)
	}
	res := h.VerifyPath(context.Background(), root, filepath.Join(root, "x.go"))
	// x.go missing package but command should still be full workspace
	if res.Command != "go test ./..." {
		// file may not exist; command selection still happens first
		if res.Attempted && res.Command != "go test ./..." {
			t.Fatalf("workspace scope cmd = %q", res.Command)
		}
	}
}

func TestDetectScopedCommandNestedGoModule(t *testing.T) {
	// Monorepo: workspace root has no go.mod; nested services/api is its own module.
	root := t.TempDir()
	mod := filepath.Join(root, "services", "api")
	pkg := filepath.Join(mod, "internal", "handler")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.com/api\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(pkg, "h.go")
	if err := os.WriteFile(file, []byte("package handler\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewDetector()
	cmd, scope, ok := d.DetectScopedCommand(root, file)
	if !ok {
		t.Fatal("expected scoped command for nested module")
	}
	if scope != ScopePackage {
		t.Fatalf("scope = %q", scope)
	}
	wantSub := "go test ./internal/handler"
	if !strings.Contains(cmd, wantSub) {
		t.Fatalf("cmd = %q, want contains %q", cmd, wantSub)
	}
	if !strings.Contains(cmd, "cd ") {
		t.Fatalf("nested module must cd into module root, got %q", cmd)
	}
	if strings.Contains(cmd, root) && !strings.Contains(cmd, mod) {
		// command should reference the module dir, not only the workspace root
		t.Fatalf("cmd should target nested module %s: %q", mod, cmd)
	}
	// Must not emit bare relative path without cd (would resolve against workDir).
	if cmd == "go test ./internal/handler" {
		t.Fatal("bare go test from workspace root would miss nested module")
	}
}

func TestVerifyPathNestedModuleRunsInModuleRoot(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "mod-a")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.com/moda\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "x.go"), []byte("package moda\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mod, "x_test.go"), []byte("package moda\nimport \"testing\"\nfunc TestOK(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewHarness(DefaultConfig())
	res := h.VerifyPath(context.Background(), root, filepath.Join(mod, "x.go"))
	if !res.Attempted {
		t.Fatalf("expected attempt: %+v", res)
	}
	if !res.Passed {
		t.Fatalf("nested module test should pass: cmd=%s out=%s err=%v", res.Command, res.Output, res.Err)
	}
	if !strings.Contains(res.Command, "cd ") {
		t.Fatalf("expected cd into module: %s", res.Command)
	}
}

func TestDetectScopedCommandSkipsNonGoFileUnderModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs := filepath.Join(root, "docs")
	_ = os.MkdirAll(docs, 0o755)
	md := filepath.Join(docs, "readme.md")
	_ = os.WriteFile(md, []byte("# hi\n"), 0o644)

	d := NewDetector()
	cmd, scope, ok := d.DetectScopedCommand(root, md)
	// Docs-only edits must not spend turn budget on full go test ./...
	if ok {
		t.Fatalf("markdown under go module should not verify, got scope=%q cmd=%q", scope, cmd)
	}
}

func TestDetectCommandGoWork(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, ok := NewDetector().DetectCommand(root)
	if !ok || cmd != "go test ./..." {
		t.Fatalf("go.work detect = %q ok=%v", cmd, ok)
	}
}

func TestHasGoHarnessSignalNested(t *testing.T) {
	root := t.TempDir()
	if HasGoHarnessSignal(root) {
		t.Fatal("empty dir should not signal")
	}
	mod := filepath.Join(root, "services")
	_ = os.MkdirAll(mod, 0o755)
	_ = os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module m\n"), 0o644)
	if !HasGoHarnessSignal(root) {
		t.Fatal("depth-1 nested go.mod should signal")
	}
}

func TestFindUpBoundedStopsAtWorkDir(t *testing.T) {
	// Parent of workDir has package.json — must not escape workDir.
	outer := t.TempDir()
	_ = os.WriteFile(filepath.Join(outer, "package.json"), []byte("{}"), 0o644)
	work := filepath.Join(outer, "project")
	_ = os.MkdirAll(work, 0o755)
	file := filepath.Join(work, "main.go")
	_ = os.WriteFile(file, []byte("package main\n"), 0o644)

	if _, found := findUpBounded(file, "package.json", work); found {
		t.Fatal("must not find package.json outside workDir")
	}
}

func TestDetectScopedPythonProject(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "svc")
	_ = os.MkdirAll(pkg, 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "pyproject.toml"), []byte("[project]\nname='svc'\n"), 0o644)
	file := filepath.Join(pkg, "app.py")
	_ = os.WriteFile(file, []byte("x=1\n"), 0o644)

	cmd, scope, ok := NewDetector().DetectScopedCommand(root, file)
	if !ok || scope != ScopePackage {
		t.Fatalf("ok=%v scope=%q cmd=%q", ok, scope, cmd)
	}
	if !strings.Contains(cmd, "pytest") || !strings.Contains(cmd, "cd ") {
		t.Fatalf("want cd + pytest, got %q", cmd)
	}
}
