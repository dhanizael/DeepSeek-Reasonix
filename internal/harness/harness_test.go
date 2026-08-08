package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetector(t *testing.T) {
	tempDir := t.TempDir()

	detector := NewDetector()

	// Test empty directory
	cmd, ok := detector.DetectCommand(tempDir)
	if ok || cmd != "" {
		t.Fatalf("expected no detected command for empty dir, got %q", cmd)
	}

	// Test Go detection
	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd, ok = detector.DetectCommand(tempDir)
	if !ok || cmd != "go test ./..." {
		t.Fatalf("expected 'go test ./...', got %q (ok=%v)", cmd, ok)
	}

	// Test Node detection
	nodeDir := t.TempDir()
	pkgPath := filepath.Join(nodeDir, "package.json")
	if err := os.WriteFile(pkgPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	pnpmLock := filepath.Join(nodeDir, "pnpm-lock.yaml")
	if err := os.WriteFile(pnpmLock, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cmd, ok = detector.DetectCommand(nodeDir)
	if !ok || cmd != "pnpm test" {
		t.Fatalf("expected 'pnpm test', got %q (ok=%v)", cmd, ok)
	}
}

func TestHarnessVerifyPassingCommand(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CustomCommand = "echo 'all tests passed'"

	h := NewHarness(cfg)
	res := h.Verify(context.Background(), t.TempDir())

	if !res.Attempted {
		t.Fatal("expected verification to be attempted")
	}
	if !res.Passed {
		t.Fatalf("expected verification to pass, got exit code %d, output: %s", res.ExitCode, res.Output)
	}
	if !strings.Contains(res.Output, "all tests passed") {
		t.Fatalf("unexpected output: %s", res.Output)
	}

	feedback := res.FormatFeedback()
	if !strings.Contains(feedback, "✅ passed") {
		t.Fatalf("unexpected feedback format: %s", feedback)
	}
}

func TestHarnessVerifyFailingCommand(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CustomCommand = "echo 'error: syntax error in main.go' && exit 1"
	cfg.Timeout = 5 * time.Second

	h := NewHarness(cfg)
	res := h.Verify(context.Background(), t.TempDir())

	if !res.Attempted {
		t.Fatal("expected verification to be attempted")
	}
	if res.Passed {
		t.Fatal("expected verification to fail")
	}
	if res.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", res.ExitCode)
	}

	feedback := res.FormatFeedback()
	if !strings.Contains(feedback, "❌ failed") {
		t.Fatalf("unexpected feedback format: %s", feedback)
	}
	if !strings.Contains(feedback, "error: syntax error in main.go") {
		t.Fatalf("feedback missing error output: %s", feedback)
	}
}
