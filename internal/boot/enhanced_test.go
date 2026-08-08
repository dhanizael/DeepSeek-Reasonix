package boot

import (
	"context"
	"strings"
	"testing"

	"reasonix/internal/agent"
	"reasonix/internal/config"
	"reasonix/internal/event"
	"reasonix/internal/harness"
	"reasonix/internal/tool"
	"reasonix/internal/tool/builtin"
)

func TestApplyEnhancedConfigDefaultsOffHarness(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	cfg := config.Default()
	a := agent.New(nil, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{}, event.Discard)
	applyEnhancedConfig(cfg, a)

	if !builtin.ASTSyntaxGuardEnabled() {
		t.Fatal("AST guard should default on")
	}
	if a.VerificationHarness() != nil {
		t.Fatal("harness must not install when [enhanced.harness].enabled is false")
	}
	if a.BacktrackGuard() != nil {
		t.Fatal("backtrack must not install when [enhanced.backtrack].enabled is false")
	}
}

func TestApplyEnhancedConfigEnablesHarnessAndBacktrack(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	cfg := config.Default()
	cfg.Enhanced.Harness.Enabled = true
	cfg.Enhanced.Harness.Command = "echo harness-ok"
	cfg.Enhanced.Harness.TimeoutSeconds = 5
	cfg.Enhanced.Backtrack.Enabled = true
	cfg.Enhanced.Backtrack.MaxStrikes = 2

	a := agent.New(nil, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{
		WriteWorkspaceRoot: t.TempDir(),
	}, event.Discard)
	applyEnhancedConfig(cfg, a)

	if a.VerificationHarness() == nil {
		t.Fatal("expected verification harness when enabled")
	}
	if a.BacktrackGuard() == nil {
		t.Fatal("expected backtrack guard when enabled")
	}
	if !EnhancedHarnessInstalled(a) || !EnhancedBacktrackInstalled(a) {
		t.Fatal("diagnostic helpers should report installed")
	}

	// Exercise the real shipped Verify path with the safe custom command.
	res := a.VerificationHarness().Verify(context.Background(), t.TempDir())
	if !res.Attempted || !res.Passed {
		t.Fatalf("safe custom command should pass: %+v err=%v", res, res.Err)
	}
	if !strings.Contains(res.FormatFeedback(), "Auto-check passed") {
		t.Fatalf("feedback = %q", res.FormatFeedback())
	}
}

func TestApplyEnhancedConfigDisablesAST(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	off := false
	cfg := config.Default()
	cfg.Enhanced.ASTGuard.Enabled = &off
	applyEnhancedConfig(cfg, nil)
	if builtin.ASTSyntaxGuardEnabled() {
		t.Fatal("AST guard should be off when config sets enabled=false")
	}
}

func TestApplyEnhancedNilConfig(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })
	builtin.SetASTSyntaxGuardEnabled(false)
	applyEnhancedConfig(nil, nil)
	if !builtin.ASTSyntaxGuardEnabled() {
		t.Fatal("nil config should restore AST default on")
	}
}

func TestHarnessEnabledFalseDoesNotAttemptVerify(t *testing.T) {
	h := harness.NewHarness(harness.Config{Enabled: false, CustomCommand: "false"})
	res := h.Verify(context.Background(), t.TempDir())
	if res.Attempted {
		t.Fatal("disabled harness must not attempt verification")
	}
}

func TestConfigEnhancedTOMLRoundTrip(t *testing.T) {
	// Prove config struct accepts the enhanced section used by boot.
	on := true
	cfg := &config.Config{}
	cfg.Enhanced.ASTGuard.Enabled = &on
	cfg.Enhanced.Harness.Enabled = true
	cfg.Enhanced.Harness.Command = "go test ./internal/harness"
	cfg.Enhanced.Backtrack.Enabled = true
	cfg.Enhanced.Backtrack.MaxStrikes = 3
	if !cfg.ASTGuardEnabled() || !cfg.HarnessEnabled() || !cfg.BacktrackEnabled() {
		t.Fatal("enhanced helpers should reflect struct fields")
	}
}
