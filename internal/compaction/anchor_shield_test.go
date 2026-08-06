package compaction

import (
	"testing"

	"reasonix/internal/provider"
)

func TestAnchorShieldStability(t *testing.T) {
	sysPrompt := "You are DeepSeek-Reasonix, a cache-optimized coding agent."
	toolsJSON := `[{"name":"read_file"},{"name":"write_file"}]`

	shield := NewAnchorShield(sysPrompt, toolsJSON)

	// Same prompt and tools must be stable
	if !shield.IsStable(sysPrompt, toolsJSON) {
		t.Fatal("expected anchor to be stable for identical system prompt and tools")
	}

	// Modified prompt must detect instability
	modifiedSysPrompt := sysPrompt + " Extra dynamic text."
	if shield.IsStable(modifiedSysPrompt, toolsJSON) {
		t.Fatal("expected anchor to detect instability when system prompt changes")
	}

	// Modified tools must detect instability
	modifiedToolsJSON := `[{"name":"read_file"}]`
	if shield.IsStable(sysPrompt, modifiedToolsJSON) {
		t.Fatal("expected anchor to detect instability when tool schema changes")
	}
}

func TestEnforceAnchor(t *testing.T) {
	sysPrompt := "System Prompt Anchor"
	shield := NewAnchorShield(sysPrompt, "{}")

	// Test replacing existing system prompt
	msgs := []provider.Message{
		{Role: provider.RoleSystem, Content: "Corrupted System Prompt"},
		{Role: provider.RoleUser, Content: "Hello"},
	}

	enforced := shield.EnforceAnchor(msgs)
	if len(enforced) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(enforced))
	}
	if enforced[0].Content != sysPrompt {
		t.Fatalf("expected enforced content %q, got %q", sysPrompt, enforced[0].Content)
	}

	// Test prepending when system prompt missing
	noSysMsgs := []provider.Message{
		{Role: provider.RoleUser, Content: "Hello without system"},
	}

	enforcedNoSys := shield.EnforceAnchor(noSysMsgs)
	if len(enforcedNoSys) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(enforcedNoSys))
	}
	if enforcedNoSys[0].Role != provider.RoleSystem || enforcedNoSys[0].Content != sysPrompt {
		t.Fatalf("expected prepended system prompt %q, got %+v", sysPrompt, enforcedNoSys[0])
	}
}
