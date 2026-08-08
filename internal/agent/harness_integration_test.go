package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"reasonix/internal/harness"
)

func TestAgentVerificationHarnessIntegrationSilentPass(t *testing.T) {
	tempDir := t.TempDir()

	// Default SilentPass=true: pass must not grow the tool-result transcript.
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

	if result != "initial result" {
		t.Fatalf("silent pass must leave tool result unchanged, got %q", result)
	}
	if strings.Contains(result, "Verification Harness") {
		t.Fatalf("silent pass leaked harness text: %q", result)
	}
}

func TestAgentVerificationHarnessIntegrationVerbosePass(t *testing.T) {
	tempDir := t.TempDir()

	cfg := harness.DefaultConfig()
	cfg.CustomCommand = "echo 'test execution passed'"
	cfg.SilentPass = false
	h := harness.NewHarness(cfg)

	a := &Agent{
		writeWorkspaceRoot: tempDir,
	}
	a.SetVerificationHarness(h)

	dummyPlan := &toolCallPlan{
		mutates:      true,
		mutationPath: filepath.Join(tempDir, "main.go"),
	}

	result := "initial result"
	a.observeAfterMutation(context.Background(), dummyPlan, &result)

	if !strings.Contains(result, "✅ passed") {
		t.Fatalf("silent_pass=false should append pass notice, got %q", result)
	}
}

func TestAgentRunResetsHarnessTurnBudget(t *testing.T) {
	// Budget is owned by the harness; Agent.Run must call BeginTurn so a long
	// session does not permanently exhaust verifies after one busy turn.
	tempDir := t.TempDir()
	cfg := harness.DefaultConfig()
	cfg.CustomCommand = "echo ok"
	cfg.PassCooldown = -1
	cfg.MaxAttemptsPerTurn = 1
	h := harness.NewHarness(cfg)

	// Exhaust without going through Run.
	r1 := h.Verify(context.Background(), tempDir)
	if !r1.Attempted {
		t.Fatalf("first should attempt: %+v", r1)
	}
	r2 := h.Verify(context.Background(), tempDir)
	if !r2.Skipped || r2.SkipReason != "budget_exhausted" {
		t.Fatalf("second should exhaust: %+v", r2)
	}

	a := &Agent{writeWorkspaceRoot: tempDir}
	a.SetVerificationHarness(h)
	// Mimic Run's budget reset without a full tool loop.
	a.verificationHarness.BeginTurn()

	r3 := h.Verify(context.Background(), tempDir)
	if !r3.Attempted {
		t.Fatalf("after BeginTurn should attempt again: %+v", r3)
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
