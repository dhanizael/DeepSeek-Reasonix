package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"reasonix/internal/harness"
)

func TestAgentVerificationHarnessIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create passing verification config
	cfg := harness.DefaultConfig()
	cfg.CustomCommand = "echo 'test execution passed'"
	h := harness.NewHarness(cfg)
	guard := harness.NewBacktrackGuard(3)

	a := &Agent{
		writeWorkspaceRoot: tempDir,
	}
	a.SetVerificationHarness(h)
	a.SetBacktrackGuard(guard)

	dummyPlan := &toolCallPlan{
		mutates:      true,
		mutationPath: filepath.Join(tempDir, "main.go"),
	}

	result := "initial result"
	a.observeAfterMutation(context.Background(), dummyPlan, &result)

	if !strings.Contains(result, "✅ Auto-check passed") {
		t.Fatalf("expected feedback to contain auto-check pass, got %q", result)
	}
}

func TestAgentBacktrackTriggerIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create failing verification config
	cfg := harness.DefaultConfig()
	cfg.CustomCommand = "echo 'test failure' && exit 1"
	h := harness.NewHarness(cfg)
	guard := harness.NewBacktrackGuard(3)

	a := &Agent{
		writeWorkspaceRoot: tempDir,
	}
	a.SetVerificationHarness(h)
	a.SetBacktrackGuard(guard)

	dummyPlan := &toolCallPlan{
		mutates:      true,
		mutationPath: filepath.Join(tempDir, "main.go"),
	}

	// Strike 1 & 2
	r1 := "result 1"
	a.observeAfterMutation(context.Background(), dummyPlan, &r1)
	r2 := "result 2"
	a.observeAfterMutation(context.Background(), dummyPlan, &r2)

	// Strike 3 triggers directive
	r3 := "result 3"
	a.observeAfterMutation(context.Background(), dummyPlan, &r3)

	if !strings.Contains(r3, "🛑 [3-Strike Backtrack Triggered]") {
		t.Fatalf("expected strike 3 result to trigger backtrack directive, got %q", r3)
	}
}
