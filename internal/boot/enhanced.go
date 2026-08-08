package boot

import (
	"path/filepath"
	"time"

	"reasonix/internal/agent"
	"reasonix/internal/config"
	"reasonix/internal/enhancedmetrics"
	"reasonix/internal/harness"
	"reasonix/internal/tool/builtin"
)

// applyEnhancedConfig installs reasonix-enhanced knobs onto the executor and
// writer tools. AST defaults on; harness defaults to mode=auto (Go workspaces
// get package-scoped post-edit checks); backtrack follows harness.
// Agent.SetBacktrackGuard / SetMutationObserver rebind checkpoint-preferring
// rollback once the observer is live.
func applyEnhancedConfig(cfg *config.Config, executor *agent.Agent, workspaceRoot string) {
	// Local-only JSONL metrics (never sent to providers).
	if home := config.ReasonixHomeDir(); home != "" {
		enhancedmetrics.SetPersistPath(filepath.Join(home, "enhanced-metrics.jsonl"))
	}

	if cfg == nil {
		builtin.SetASTSyntaxGuardEnabled(true)
		return
	}
	builtin.SetASTSyntaxGuardEnabled(cfg.ASTGuardEnabled())
	if executor == nil {
		return
	}

	harnessOn := cfg.HarnessEnabledForRoot(workspaceRoot)
	if harnessOn {
		hcfg := harness.DefaultConfig()
		hcfg.Enabled = true
		hcfg.CustomCommand = cfg.Enhanced.Harness.Command
		hcfg.Scope = cfg.HarnessScope()
		hcfg.SilentPass = cfg.HarnessSilentPass()
		if cfg.Enhanced.Harness.TimeoutSeconds > 0 {
			hcfg.Timeout = time.Duration(cfg.Enhanced.Harness.TimeoutSeconds) * time.Second
		}
		if cfg.Enhanced.Harness.MaxOutputBytes > 0 {
			hcfg.MaxOutputBytes = cfg.Enhanced.Harness.MaxOutputBytes
		}
		executor.SetVerificationHarness(harness.NewHarness(hcfg))
	}

	if cfg.BacktrackEnabled(harnessOn) {
		executor.SetBacktrackGuard(harness.NewBacktrackGuard(cfg.Enhanced.Backtrack.MaxStrikes))
	}
}

// EnhancedHarnessInstalled reports whether a non-nil verification harness is present.
func EnhancedHarnessInstalled(a *agent.Agent) bool {
	return a != nil && a.VerificationHarness() != nil
}

// EnhancedBacktrackInstalled reports whether a non-nil backtrack guard is present.
func EnhancedBacktrackInstalled(a *agent.Agent) bool {
	return a != nil && a.BacktrackGuard() != nil
}
