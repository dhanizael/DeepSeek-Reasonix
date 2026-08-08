// Package enhancedmetrics records local-only counters for reasonix-enhanced
// reliability features. Nothing here is sent to model providers or external
// telemetry endpoints — zero API token cost.
package enhancedmetrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Counters is a point-in-time snapshot of process-local enhanced metrics.
type Counters struct {
	ASTRejects         uint64 `json:"ast_rejects"`
	HarnessAttempted   uint64 `json:"harness_attempted"`
	HarnessPassed      uint64 `json:"harness_passed"`
	HarnessFailed      uint64 `json:"harness_failed"`
	HarnessSkipped     uint64 `json:"harness_skipped"`
	HarnessSilentPass  uint64 `json:"harness_silent_pass"` // pass with zero model-visible text
	BacktrackTriggered uint64 `json:"backtrack_triggered"`
}

var (
	astRejects         atomic.Uint64
	harnessAttempted   atomic.Uint64
	harnessPassed      atomic.Uint64
	harnessFailed      atomic.Uint64
	harnessSkipped     atomic.Uint64
	harnessSilentPass  atomic.Uint64
	backtrackTriggered atomic.Uint64

	persistMu   sync.Mutex
	persistPath string // empty = disabled until SetPersistPath
)

// SetPersistPath enables append-only JSONL logging under path (usually
// ~/.reasonix/enhanced-metrics.jsonl). Empty disables persistence.
func SetPersistPath(path string) {
	persistMu.Lock()
	persistPath = path
	persistMu.Unlock()
}

// PersistPath returns the active JSONL path, or empty when persistence is off.
func PersistPath() string {
	persistMu.Lock()
	defer persistMu.Unlock()
	return persistPath
}

// Snapshot returns current process-local counters.
func Snapshot() Counters {
	return Counters{
		ASTRejects:         astRejects.Load(),
		HarnessAttempted:   harnessAttempted.Load(),
		HarnessPassed:      harnessPassed.Load(),
		HarnessFailed:      harnessFailed.Load(),
		HarnessSkipped:     harnessSkipped.Load(),
		HarnessSilentPass:  harnessSilentPass.Load(),
		BacktrackTriggered: backtrackTriggered.Load(),
	}
}

// Zero reports whether every counter is zero.
func (c Counters) Zero() bool {
	return c.ASTRejects == 0 &&
		c.HarnessAttempted == 0 &&
		c.HarnessPassed == 0 &&
		c.HarnessFailed == 0 &&
		c.HarnessSkipped == 0 &&
		c.HarnessSilentPass == 0 &&
		c.BacktrackTriggered == 0
}

// FormatCompact returns one short diagnostic line for /status (empty if Zero).
// Never includes file paths or command text — safe for issue reports.
func (c Counters) FormatCompact() string {
	if c.Zero() {
		return ""
	}
	// thrift ratio: silent passes / passes (how often success stayed out of context)
	parts := []string{
		fmt.Sprintf("ast_rej=%d", c.ASTRejects),
		fmt.Sprintf("harness=%d/%d/%d", c.HarnessPassed, c.HarnessFailed, c.HarnessSkipped),
		fmt.Sprintf("silent=%d", c.HarnessSilentPass),
		fmt.Sprintf("backtrack=%d", c.BacktrackTriggered),
	}
	return strings.Join(parts, " ")
}

// FormatLines returns multi-line detail for doctor / expanded diagnostics.
func (c Counters) FormatLines(prefix string) []string {
	if c.Zero() {
		return nil
	}
	if prefix == "" {
		prefix = "  "
	}
	return []string{
		fmt.Sprintf("%sast rejects     %d", prefix, c.ASTRejects),
		fmt.Sprintf("%sharness pass    %d", prefix, c.HarnessPassed),
		fmt.Sprintf("%sharness fail    %d", prefix, c.HarnessFailed),
		fmt.Sprintf("%sharness skip    %d", prefix, c.HarnessSkipped),
		fmt.Sprintf("%ssilent pass     %d  (zero tool-result text on success)", prefix, c.HarnessSilentPass),
		fmt.Sprintf("%sbacktrack       %d", prefix, c.BacktrackTriggered),
		fmt.Sprintf("%sattempts        %d", prefix, c.HarnessAttempted),
	}
}

// Reset clears process counters (tests only).
func Reset() {
	astRejects.Store(0)
	harnessAttempted.Store(0)
	harnessPassed.Store(0)
	harnessFailed.Store(0)
	harnessSkipped.Store(0)
	harnessSilentPass.Store(0)
	backtrackTriggered.Store(0)
}

// RecordASTReject increments pre-write syntax rejections.
func RecordASTReject(path string) {
	astRejects.Add(1)
	persist("ast_reject", map[string]any{"path": path})
}

// RecordHarnessAttempt records a completed verify run.
func RecordHarnessAttempt(passed bool, command, scope string, durationMS int64) {
	harnessAttempted.Add(1)
	if passed {
		harnessPassed.Add(1)
	} else {
		harnessFailed.Add(1)
	}
	persist("harness", map[string]any{
		"passed":      passed,
		"command":     command,
		"scope":       scope,
		"duration_ms": durationMS,
	})
}

// RecordHarnessSkip records a verify that was skipped (in-flight or cooldown).
func RecordHarnessSkip(reason, command string) {
	harnessSkipped.Add(1)
	persist("harness_skip", map[string]any{"reason": reason, "command": command})
}

// RecordHarnessSilentPass records a pass whose FormatFeedback was empty
// (silent_pass thrift — zero tool-result tokens for the model).
func RecordHarnessSilentPass(command string) {
	harnessSilentPass.Add(1)
	persist("harness_silent_pass", map[string]any{"command": command})
}

// RecordBacktrack increments 3-strike triggers.
func RecordBacktrack(path string) {
	backtrackTriggered.Add(1)
	persist("backtrack", map[string]any{"path": path})
}

type logEvent struct {
	TS   string         `json:"ts"`
	Kind string         `json:"kind"`
	Data map[string]any `json:"data,omitempty"`
}

func persist(kind string, data map[string]any) {
	persistMu.Lock()
	path := persistPath
	persistMu.Unlock()
	if path == "" {
		return
	}
	ev := logEvent{
		TS:   time.Now().UTC().Format(time.RFC3339Nano),
		Kind: kind,
		Data: data,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	b = append(b, '\n')

	persistMu.Lock()
	defer persistMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(b)
	_ = f.Close()
}
