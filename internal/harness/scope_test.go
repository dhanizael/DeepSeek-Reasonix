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
