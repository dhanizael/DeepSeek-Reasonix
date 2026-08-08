package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestEnhancedConfigDecodeFromTOML(t *testing.T) {
	const body = `
[enhanced.ast_guard]
enabled = false

[enhanced.harness]
enabled = true
command = "echo ok"
timeout_seconds = 15
max_output_bytes = 2048

[enhanced.backtrack]
enabled = true
max_strikes = 4
`
	var cfg Config
	if _, err := toml.Decode(body, &cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.ASTGuardEnabled() {
		t.Fatal("ast_guard.enabled=false should disable")
	}
	if !cfg.HarnessEnabled() {
		t.Fatal("harness should be enabled")
	}
	if cfg.Enhanced.Harness.Command != "echo ok" {
		t.Fatalf("command = %q", cfg.Enhanced.Harness.Command)
	}
	if cfg.Enhanced.Harness.TimeoutSeconds != 15 {
		t.Fatalf("timeout = %d", cfg.Enhanced.Harness.TimeoutSeconds)
	}
	if !cfg.BacktrackEnabled() || cfg.Enhanced.Backtrack.MaxStrikes != 4 {
		t.Fatalf("backtrack = %+v", cfg.Enhanced.Backtrack)
	}
}

func TestEnhancedDefaults(t *testing.T) {
	cfg := Default()
	if !cfg.ASTGuardEnabled() {
		t.Fatal("default AST on")
	}
	if cfg.HarnessEnabled() || cfg.BacktrackEnabled() {
		t.Fatal("default harness/backtrack off")
	}
}

func TestEnhancedMissingSectionDefaultsASTOn(t *testing.T) {
	var cfg Config
	if _, err := toml.Decode("default_model = \"x\"\n", &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.ASTGuardEnabled() {
		t.Fatal("absent enhanced section must default AST on")
	}
	_ = strings.TrimSpace
}
