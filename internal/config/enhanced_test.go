package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestEnhancedConfigDecodeFromTOML(t *testing.T) {
	const body = `
[enhanced.ast_guard]
enabled = false

[enhanced.harness]
mode = "on"
command = "echo ok"
timeout_seconds = 15
scope = "package"
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
	if cfg.HarnessMode() != "on" {
		t.Fatalf("mode = %q", cfg.HarnessMode())
	}
	if !cfg.HarnessEnabledForRoot(t.TempDir()) {
		t.Fatal("mode on should enable regardless of go.mod")
	}
	if cfg.HarnessScope() != "package" {
		t.Fatalf("scope = %q", cfg.HarnessScope())
	}
	if !cfg.BacktrackEnabled(true) || cfg.Enhanced.Backtrack.MaxStrikes != 4 {
		t.Fatalf("backtrack = %+v", cfg.Enhanced.Backtrack)
	}
}

func TestEnhancedDefaultsAuto(t *testing.T) {
	cfg := Default()
	if !cfg.ASTGuardEnabled() {
		t.Fatal("default AST on")
	}
	if cfg.HarnessMode() != "auto" {
		t.Fatalf("default mode = %q want auto", cfg.HarnessMode())
	}
	if cfg.HarnessScope() != "package" {
		t.Fatalf("default scope = %q", cfg.HarnessScope())
	}
	// follow harness when backtrack.Enabled nil
	if !cfg.BacktrackEnabled(true) || cfg.BacktrackEnabled(false) {
		t.Fatal("backtrack should follow harness when unset")
	}
}

func TestHarnessEnabledForRootAuto(t *testing.T) {
	cfg := Default()
	empty := t.TempDir()
	if cfg.HarnessEnabledForRoot(empty) {
		t.Fatal("auto without go.mod => off")
	}
	goRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(goRoot, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !cfg.HarnessEnabledForRoot(goRoot) {
		t.Fatal("auto with go.mod => on")
	}
	cfg.Enhanced.Harness.Mode = "off"
	if cfg.HarnessEnabledForRoot(goRoot) {
		t.Fatal("mode off => off")
	}
}

func TestBacktrackExplicitFalse(t *testing.T) {
	cfg := Default()
	off := false
	cfg.Enhanced.Backtrack.Enabled = &off
	if cfg.BacktrackEnabled(true) {
		t.Fatal("explicit false must win")
	}
}
