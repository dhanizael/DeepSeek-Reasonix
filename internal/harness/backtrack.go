package harness

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// BacktrackGuard enforces the 3-Strike rule to prevent repeated execution failures.
type BacktrackGuard struct {
	mu         sync.Mutex
	maxStrikes int
	strikes    map[string]int
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

// Rollback automatically reverts uncommitted changes to a target file (or working tree if target is empty).
func (g *BacktrackGuard) Rollback(ctx context.Context, workDir string, targetFile string) (string, error) {
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
	return string(out), nil
}

// FormatBacktrackDirective generates a strict directive instructing the agent to switch strategy after 3 strikes.
func (g *BacktrackGuard) FormatBacktrackDirective(target string) string {
	targetLabel := target
	if targetLabel == "" {
		targetLabel = "the project"
	}

	return fmt.Sprintf("\n🛑 [3-Strike Backtrack Triggered]\n"+
		"Your edits on %s have failed verification 3 consecutive times.\n"+
		"The affected files have been automatically rolled back to the last clean checkpoint.\n"+
		"⚠️ PROHIBITION: Do NOT attempt the same code change or patch again. Formulate an entirely different approach, hypothesis, or architecture.\n", targetLabel)
}
