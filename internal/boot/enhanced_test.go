package boot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"reasonix/internal/agent"
	"reasonix/internal/config"
	"reasonix/internal/event"
	"reasonix/internal/harness"
	"reasonix/internal/tool"
	"reasonix/internal/tool/builtin"
)

func TestApplyEnhancedConfigAutoGoEnablesHarness(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module t\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	// default mode=auto, no explicit harness.enabled
	a := agent.New(nil, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{
		WriteWorkspaceRoot: root,
	}, event.Discard)
	applyEnhancedConfig(cfg, a, root)

	if !builtin.ASTSyntaxGuardEnabled() {
		t.Fatal("AST on")
	}
	if a.VerificationHarness() == nil {
		t.Fatal("auto mode + go.mod should install harness")
	}
	if a.BacktrackGuard() == nil {
		t.Fatal("backtrack should follow harness by default")
	}
}

func TestApplyEnhancedConfigAutoNoGoDisablesHarness(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	root := t.TempDir() // no go.mod
	cfg := config.Default()
	a := agent.New(nil, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{}, event.Discard)
	applyEnhancedConfig(cfg, a, root)

	if a.VerificationHarness() != nil {
		t.Fatal("auto mode without go.mod should not install harness")
	}
	if a.BacktrackGuard() != nil {
		t.Fatal("backtrack should not install without harness")
	}
}

func TestApplyEnhancedConfigModeOff(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module t\n"), 0o644)
	cfg := config.Default()
	cfg.Enhanced.Harness.Mode = "off"
	a := agent.New(nil, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{}, event.Discard)
	applyEnhancedConfig(cfg, a, root)
	if a.VerificationHarness() != nil {
		t.Fatal("mode=off must disable harness even with go.mod")
	}
}

func TestApplyEnhancedConfigModeOnWithCustomCommand(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	cfg := config.Default()
	cfg.Enhanced.Harness.Mode = "on"
	cfg.Enhanced.Harness.Command = "echo harness-ok"
	cfg.Enhanced.Harness.TimeoutSeconds = 5
	off := false
	cfg.Enhanced.Backtrack.Enabled = &off

	a := agent.New(nil, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{
		WriteWorkspaceRoot: t.TempDir(),
	}, event.Discard)
	applyEnhancedConfig(cfg, a, t.TempDir())

	if a.VerificationHarness() == nil {
		t.Fatal("mode=on should install harness")
	}
	if a.BacktrackGuard() != nil {
		t.Fatal("explicit backtrack.enabled=false should skip guard")
	}

	res := a.VerificationHarness().Verify(context.Background(), t.TempDir())
	if !res.Attempted || !res.Passed {
		t.Fatalf("custom command should pass: %+v", res)
	}
	// Default silent_pass: zero model-visible text on pass.
	if res.FormatFeedback() != "" {
		t.Fatalf("default silent pass feedback = %q want empty", res.FormatFeedback())
	}
}

func TestApplyEnhancedConfigSilentPassFalse(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	verbose := false
	cfg := config.Default()
	cfg.Enhanced.Harness.Mode = "on"
	cfg.Enhanced.Harness.Command = "echo harness-ok"
	cfg.Enhanced.Harness.SilentPass = &verbose
	off := false
	cfg.Enhanced.Backtrack.Enabled = &off

	a := agent.New(nil, tool.NewRegistry(), agent.NewSession("sys"), agent.Options{
		WriteWorkspaceRoot: t.TempDir(),
	}, event.Discard)
	applyEnhancedConfig(cfg, a, t.TempDir())

	res := a.VerificationHarness().Verify(context.Background(), t.TempDir())
	if !res.Passed {
		t.Fatalf("want pass: %+v", res)
	}
	if !strings.Contains(res.FormatFeedback(), "✅ passed") {
		t.Fatalf("silent_pass=false feedback = %q", res.FormatFeedback())
	}
}

func TestApplyEnhancedConfigDisablesAST(t *testing.T) {
	t.Cleanup(func() { builtin.SetASTSyntaxGuardEnabled(true) })

	off := false
	cfg := config.Default()
	cfg.Enhanced.ASTGuard.Enabled = &off
	applyEnhancedConfig(cfg, nil, "")
	if builtin.ASTSyntaxGuardEnabled() {
		t.Fatal("AST guard should be off")
	}
}

func TestHarnessEnabledFalseDoesNotAttemptVerify(t *testing.T) {
	h := harness.NewHarness(harness.Config{Enabled: false, CustomCommand: "false"})
	res := h.Verify(context.Background(), t.TempDir())
	if res.Attempted {
		t.Fatal("disabled harness must not attempt verification")
	}
}
