package harness

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// RollbackFunc restores a path after 3-strike trigger. Prefer session checkpoint
// preimage; git checkout is only a fallback.
type RollbackFunc func(ctx context.Context, workDir, targetFile string) (detail string, err error)

// BacktrackGuard enforces the 3-Strike rule to prevent repeated verification failures.
type BacktrackGuard struct {
	mu         sync.Mutex
	maxStrikes int
	strikes    map[string]int
	rollback   RollbackFunc // optional; nil uses git checkout fallback
}

// NewBacktrackGuard creates a new guard with specified max strikes (default 3).
func NewBacktrackGuard(maxStrikes int) *BacktrackGuard {
	if maxStrikes <= 0 {
		maxStrikes = 3
	}
	return &BacktrackGuard{
		maxStrikes: maxStrikes,
		strikes:    make(map[string]int),
	}
}

// SetRollbackFunc installs a preferred restore backend (e.g. checkpoint preimage).
func (g *BacktrackGuard) SetRollbackFunc(fn RollbackFunc) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.rollback = fn
	g.mu.Unlock()
}

// RecordFailure increments failure strike count for a given target. Returns (currentStrikes, triggeredRollback).
func (g *BacktrackGuard) RecordFailure(target string) (int, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	targetKey := strings.TrimSpace(target)
	if targetKey == "" {
		targetKey = "global"
	}

	g.strikes[targetKey]++
	count := g.strikes[targetKey]

	triggered := count >= g.maxStrikes
	return count, triggered
}

// RecordSuccess resets the strike count for a given target upon successful verification.
func (g *BacktrackGuard) RecordSuccess(target string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	targetKey := strings.TrimSpace(target)
	if targetKey == "" {
		targetKey = "global"
	}
	delete(g.strikes, targetKey)
}

// GetStrikes returns the current strike count for a target.
func (g *BacktrackGuard) GetStrikes(target string) int {
	g.mu.Lock()
	defer g.mu.Unlock()

	targetKey := strings.TrimSpace(target)
	if targetKey == "" {
		targetKey = "global"
	}
	return g.strikes[targetKey]
}

// Rollback restores targetFile. Uses the installed RollbackFunc when set;
// otherwise falls back to `git checkout HEAD -- <path>`.
func (g *BacktrackGuard) Rollback(ctx context.Context, workDir string, targetFile string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("backtrack guard is nil")
	}
	g.mu.Lock()
	fn := g.rollback
	g.mu.Unlock()
	if fn != nil {
		return fn(ctx, workDir, targetFile)
	}
	return gitCheckoutRollback(ctx, workDir, targetFile)
}

// GitCheckoutRollback restores via git (exported for tests / explicit fallback).
func GitCheckoutRollback(ctx context.Context, workDir, targetFile string) (string, error) {
	return gitCheckoutRollback(ctx, workDir, targetFile)
}

func gitCheckoutRollback(ctx context.Context, workDir string, targetFile string) (string, error) {
	var cmd *exec.Cmd
	if targetFile != "" {
		cmd = exec.CommandContext(ctx, "git", "checkout", "HEAD", "--", targetFile)
	} else {
		cmd = exec.CommandContext(ctx, "git", "checkout", "HEAD", "--", ".")
	}
	cmd.Dir = workDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git rollback failed: %w (output: %s)", err, string(out))
	}
	return "git checkout HEAD -- " + targetFile + "\n" + string(out), nil
}

// FormatBacktrackDirective generates a strict directive instructing the agent to switch strategy after 3 strikes.
func (g *BacktrackGuard) FormatBacktrackDirective(target string) string {
	targetLabel := target
	if targetLabel == "" {
		targetLabel = "the project"
	}

	max := 3
	if g != nil && g.maxStrikes > 0 {
		max = g.maxStrikes
	}

	return fmt.Sprintf("\n🛑 [3-Strike Backtrack Triggered]\n"+
		"Your edits on %s have failed verification %d consecutive times.\n"+
		"The affected file has been rolled back toward the last known-good session/git state when available.\n"+
		"⚠️ PROHIBITION: Do NOT attempt the same code change or patch again. Formulate an entirely different approach, hypothesis, or architecture.\n",
		targetLabel, max)
}
