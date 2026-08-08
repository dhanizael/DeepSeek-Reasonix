package config

// EnhancedConfig holds reasonix-enhanced fork knobs. Upstream ignores unknown
// TOML keys only when decoding into a struct that omits them; this section is
// fork-local and defaults to safe behavior (AST on, harness/backtrack off).
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

// EnhancedHarnessConfig controls the host-side post-mutation verification
// runner. Default Enabled=false so normal sessions never auto-run full-repo
// tests after every edit.
type EnhancedHarnessConfig struct {
	Enabled        bool   `toml:"enabled"`
	Command        string `toml:"command"` // empty = auto-detect from workspace markers
	TimeoutSeconds int    `toml:"timeout_seconds"`
	MaxOutputBytes int    `toml:"max_output_bytes"`
}

// EnhancedBacktrackConfig controls the 3-strike policy after harness failures.
// Only meaningful when harness is also enabled; default Enabled=false.
type EnhancedBacktrackConfig struct {
	Enabled    bool `toml:"enabled"`
	MaxStrikes int  `toml:"max_strikes"`
}

// ASTGuardEnabled reports whether pre-write AST/JSON syntax validation is on.
// Nil Enabled pointer means default-on.
func (c *Config) ASTGuardEnabled() bool {
	if c == nil || c.Enhanced.ASTGuard.Enabled == nil {
		return true
	}
	return *c.Enhanced.ASTGuard.Enabled
}

// HarnessEnabled reports whether the host verification harness should be
// installed on the executor.
func (c *Config) HarnessEnabled() bool {
	return c != nil && c.Enhanced.Harness.Enabled
}

// BacktrackEnabled reports whether the 3-strike backtrack guard should be
// installed on the executor.
func (c *Config) BacktrackEnabled() bool {
	return c != nil && c.Enhanced.Backtrack.Enabled
}
