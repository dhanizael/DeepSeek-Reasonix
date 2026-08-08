package boot

import (
	"time"

	"reasonix/internal/agent"
	"reasonix/internal/config"
	"reasonix/internal/harness"
	"reasonix/internal/tool/builtin"
)

// applyEnhancedConfig installs reasonix-enhanced knobs onto the executor and
// writer tools. AST guard defaults on; harness and backtrack stay off unless
// explicitly enabled in [enhanced] config.
func applyEnhancedConfig(cfg *config.Config, executor *agent.Agent) {
	if cfg == nil {
		builtin.SetASTSyntaxGuardEnabled(true)
		return
	}
	builtin.SetASTSyntaxGuardEnabled(cfg.ASTGuardEnabled())
	if executor == nil {
		return
	}
	if cfg.HarnessEnabled() {
		hcfg := harness.DefaultConfig()
		hcfg.Enabled = true
		hcfg.CustomCommand = cfg.Enhanced.Harness.Command
		if cfg.Enhanced.Harness.TimeoutSeconds > 0 {
			hcfg.Timeout = time.Duration(cfg.Enhanced.Harness.TimeoutSeconds) * time.Second
		}
		if cfg.Enhanced.Harness.MaxOutputBytes > 0 {
			hcfg.MaxOutputBytes = cfg.Enhanced.Harness.MaxOutputBytes
		}
		executor.SetVerificationHarness(harness.NewHarness(hcfg))
	}
	if cfg.BacktrackEnabled() {
		executor.SetBacktrackGuard(harness.NewBacktrackGuard(cfg.Enhanced.Backtrack.MaxStrikes))
	}
}

// EnhancedHarnessInstalled reports whether a non-nil verification harness is
// present on the agent (test helper / diagnostics).
func EnhancedHarnessInstalled(a *agent.Agent) bool {
	return a != nil && a.VerificationHarness() != nil
}

// EnhancedBacktrackInstalled reports whether a non-nil backtrack guard is
// present on the agent (test helper / diagnostics).
func EnhancedBacktrackInstalled(a *agent.Agent) bool {
	return a != nil && a.BacktrackGuard() != nil
}
