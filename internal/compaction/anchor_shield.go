package compaction

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"reasonix/internal/provider"
)

// FrozenAnchor represents an immutable system prefix anchor to maximize DeepSeek prompt-cache reuse.
type FrozenAnchor struct {
	SystemPrompt string    `json:"system_prompt"`
	ToolsHash    string    `json:"tools_hash"`
	AnchorHash   string    `json:"anchor_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

// ComputeHash generates a deterministic SHA-256 hash of the system prompt and tool definitions.
func ComputeHash(systemPrompt string, toolsJSON string) string {
	h := sha256.New()
	h.Write([]byte(systemPrompt))
	h.Write([]byte("|tools|"))
	h.Write([]byte(toolsJSON))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// NewFrozenAnchor creates an immutable prefix anchor snapshot.
func NewFrozenAnchor(systemPrompt string, toolsJSON string) *FrozenAnchor {
	hash := ComputeHash(systemPrompt, toolsJSON)
	return &FrozenAnchor{
		SystemPrompt: systemPrompt,
		ToolsHash:    ComputeHash("", toolsJSON),
		AnchorHash:   hash,
		CreatedAt:    time.Now(),
	}
}

// AnchorShield protects the static prefix window against unexpected dynamic mutations.
//
// Deprecated: do not call EnforceAnchor on the production control path.
// Upstream cache-aware context projection (CoveredPrefixHash, compact_projection)
// owns prefix stability. See docs/ENHANCED_ROADMAP.md (DROP: AnchorShield enforcer).
type AnchorShield struct {
	mu     sync.RWMutex
	anchor *FrozenAnchor
}

// NewAnchorShield constructs a shield manager.
//
// Deprecated: experimental only; not wired into boot or agent loops.
func NewAnchorShield(systemPrompt string, toolsJSON string) *AnchorShield {
	return &AnchorShield{
		anchor: NewFrozenAnchor(systemPrompt, toolsJSON),
	}
}

// Anchor returns the current immutable anchor.
func (s *AnchorShield) Anchor() *FrozenAnchor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.anchor
}

// IsStable checks whether the current system prompt and tool schemas match the frozen anchor.
func (s *AnchorShield) IsStable(systemPrompt string, toolsJSON string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.anchor == nil {
		return false
	}
	currentHash := ComputeHash(systemPrompt, toolsJSON)
	return s.anchor.AnchorHash == currentHash
}

// EnforceAnchor Ensures that the session message list maintains the exact frozen system prompt at index 0.
//
// Deprecated: must not be called from live agent/boot paths (dual prefix rewriter).
func (s *AnchorShield) EnforceAnchor(messages []provider.Message) []provider.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.anchor == nil || len(messages) == 0 {
		return messages
	}

	result := make([]provider.Message, len(messages))
	copy(result, messages)

	// Ensure message at index 0 is RoleSystem matching the frozen prompt
	if result[0].Role == provider.RoleSystem {
		result[0].Content = s.anchor.SystemPrompt
	} else {
		// Prepend system prompt if missing
		sysMsg := provider.Message{
			Role:    provider.RoleSystem,
			Content: s.anchor.SystemPrompt,
		}
		result = append([]provider.Message{sysMsg}, result...)
	}

	return result
}
