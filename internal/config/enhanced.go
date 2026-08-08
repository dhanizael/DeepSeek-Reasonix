package config

import (
	"os"
	"path/filepath"
	"strings"
)

// EnhancedConfig holds reasonix-enhanced fork knobs. Upstream ignores unknown
// TOML keys only when decoding into a struct that omits them; this section is
// fork-local.
type EnhancedConfig struct {
	ASTGuard  EnhancedASTGuardConfig  `toml:"ast_guard"`
	Harness   EnhancedHarnessConfig   `toml:"harness"`
	Backtrack EnhancedBacktrackConfig `toml:"backtrack"`
}

// EnhancedASTGuardConfig controls pre-write syntax validation for Go/JSON.
// When Enabled is nil, validation defaults to on.
type EnhancedASTGuardConfig struct {
	Enabled *bool `toml:"enabled"`
}

// EnhancedHarnessConfig controls the host-side post-mutation verification runner.
//
// Mode:
//   - "auto" (default): enable package-scoped checks when workspace has go.mod
//   - "on": always enable (requires a detectable or custom command)
//   - "off": never enable
//
// Legacy Enabled=true forces "on". Empty Mode with Enabled=false yields "auto".
type EnhancedHarnessConfig struct {
	Enabled        bool   `toml:"enabled"`
	Mode           string `toml:"mode"` // auto|on|off
	Command        string `toml:"command"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
	MaxOutputBytes int    `toml:"max_output_bytes"`
	// Scope: package (default) | workspace
	Scope string `toml:"scope"`
}

// EnhancedBacktrackConfig controls the 3-strike policy after harness failures.
// When Enabled is nil, backtrack follows the harness (on when harness is on).
type EnhancedBacktrackConfig struct {
	Enabled    *bool `toml:"enabled"`
	MaxStrikes int   `toml:"max_strikes"`
}

// ASTGuardEnabled reports whether pre-write AST/JSON syntax validation is on.
func (c *Config) ASTGuardEnabled() bool {
	if c == nil || c.Enhanced.ASTGuard.Enabled == nil {
		return true
	}
	return *c.Enhanced.ASTGuard.Enabled
}

// HarnessMode returns normalized harness mode: auto|on|off.
func (c *Config) HarnessMode() string {
	if c == nil {
		return "auto"
	}
	mode := strings.ToLower(strings.TrimSpace(c.Enhanced.Harness.Mode))
	switch mode {
	case "on", "off", "auto":
		return mode
	}
	// Legacy: explicit enabled=true => on; otherwise enhanced default auto.
	if c.Enhanced.Harness.Enabled {
		return "on"
	}
	return "auto"
}

// HarnessEnabledForRoot reports whether the host harness should install for workspaceRoot.
func (c *Config) HarnessEnabledForRoot(workspaceRoot string) bool {
	switch c.HarnessMode() {
	case "on":
		return true
	case "off":
		return false
	default: // auto
		root := strings.TrimSpace(workspaceRoot)
		if root == "" {
			root = "."
		}
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return true
		}
		return false
	}
}

// HarnessEnabled is legacy: true only for mode "on" (not auto). Prefer HarnessEnabledForRoot.
func (c *Config) HarnessEnabled() bool {
	return c != nil && c.HarnessMode() == "on"
}

// HarnessScope returns package|workspace.
func (c *Config) HarnessScope() string {
	if c == nil {
		return "package"
	}
	switch strings.ToLower(strings.TrimSpace(c.Enhanced.Harness.Scope)) {
	case "workspace", "full", "all":
		return "workspace"
	default:
		return "package"
	}
}

// BacktrackEnabled reports whether backtrack installs given harnessActive.
// Nil Enabled pointer means follow harness.
func (c *Config) BacktrackEnabled(harnessActive bool) bool {
	if c == nil {
		return harnessActive
	}
	if c.Enhanced.Backtrack.Enabled != nil {
		return *c.Enhanced.Backtrack.Enabled
	}
	return harnessActive
}
