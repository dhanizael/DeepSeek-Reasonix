package harness

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"reasonix/internal/enhancedmetrics"
)

func TestPassCooldownSkipsRerun(t *testing.T) {
	enhancedmetrics.Reset()
	cfg := DefaultConfig()
	cfg.CustomCommand = "echo ok"
	cfg.PassCooldown = 30 * time.Second
	h := NewHarness(cfg)
	dir := t.TempDir()

	r1 := h.Verify(context.Background(), dir)
	if !r1.Attempted || !r1.Passed {
		t.Fatalf("first run should pass: %+v", r1)
	}
	r2 := h.Verify(context.Background(), dir)
	if r2.Attempted || !r2.Skipped {
		t.Fatalf("second run should skip cooldown: %+v", r2)
	}
	if r2.SkipReason != "pass_cooldown" {
		t.Fatalf("reason = %q", r2.SkipReason)
	}
	if r2.FormatFeedback() != "" {
		t.Fatal("skip must not inject transcript text (token thrift)")
	}
	snap := enhancedmetrics.Snapshot()
	if snap.HarnessSkipped < 1 || snap.HarnessPassed < 1 {
		t.Fatalf("metrics = %+v", snap)
	}
}

func TestInFlightSkipsParallelSameCommand(t *testing.T) {
	enhancedmetrics.Reset()
	cfg := DefaultConfig()
	cfg.CustomCommand = "sleep 0.3"
	cfg.PassCooldown = -1 // disable cooldown
	cfg.Timeout = 5 * time.Second
	h := NewHarness(cfg)
	dir := t.TempDir()

	var wg sync.WaitGroup
	results := make([]Result, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0] = h.Verify(context.Background(), dir)
	}()
	time.Sleep(50 * time.Millisecond) // ensure first holds inFlight
	go func() {
		defer wg.Done()
		results[1] = h.Verify(context.Background(), dir)
	}()
	wg.Wait()

	var skipped, attempted int
	for _, r := range results {
		if r.Skipped {
			skipped++
		}
		if r.Attempted {
			attempted++
		}
	}
	if attempted < 1 {
		t.Fatal("expected at least one attempted run")
	}
	if skipped < 1 {
		t.Fatalf("expected in-flight skip, results=%+v", results)
	}
}

func TestFormatFeedbackSilentPassDefault(t *testing.T) {
	r := Result{
		Attempted:  true,
		Passed:     true,
		Command:    "go test ./pkg",
		Duration:   time.Millisecond,
		Scope:      ScopePackage,
		SilentPass: true,
	}
	if fb := r.FormatFeedback(); fb != "" {
		t.Fatalf("silent pass must be empty, got %q", fb)
	}
}

func TestFormatFeedbackShortPassVerbose(t *testing.T) {
	r := Result{
		Attempted:  true,
		Passed:     true,
		Command:    "go test ./pkg",
		Duration:   time.Millisecond,
		Scope:      ScopePackage,
		SilentPass: false,
	}
	fb := r.FormatFeedback()
	if fb == "" || !strings.Contains(fb, "✅ passed") {
		t.Fatalf("verbose pass should emit notice: %q", fb)
	}
	if strings.Count(fb, "\n") > 3 {
		t.Fatalf("pass feedback should be minimal lines: %q", fb)
	}
	if len(fb) > 200 {
		t.Fatalf("pass feedback too long for token thrift: %d", len(fb))
	}
}

func TestSilentPassRecordsMetric(t *testing.T) {
	enhancedmetrics.Reset()
	cfg := DefaultConfig()
	cfg.CustomCommand = "echo ok"
	cfg.PassCooldown = -1
	h := NewHarness(cfg)
	r := h.Verify(context.Background(), t.TempDir())
	if !r.Passed || r.FormatFeedback() != "" {
		t.Fatalf("want silent pass: %+v feedback=%q", r, r.FormatFeedback())
	}
	snap := enhancedmetrics.Snapshot()
	if snap.HarnessPassed < 1 || snap.HarnessSilentPass < 1 {
		t.Fatalf("metrics = %+v", snap)
	}
}

func TestMaxOutputBytesCeiling(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxOutputBytes = 100000
	h := NewHarness(cfg)
	if h.config.MaxOutputBytes > 4096 {
		t.Fatalf("ceiling broken: %d", h.config.MaxOutputBytes)
	}
}

func TestTurnBudgetExhaustsAndResets(t *testing.T) {
	enhancedmetrics.Reset()
	cfg := DefaultConfig()
	cfg.CustomCommand = "echo ok"
	cfg.PassCooldown = -1 // disable cooldown so every call would shell out
	cfg.MaxAttemptsPerTurn = 2
	h := NewHarness(cfg)
	dir := t.TempDir()

	r1 := h.Verify(context.Background(), dir)
	r2 := h.Verify(context.Background(), dir)
	r3 := h.Verify(context.Background(), dir)

	if !r1.Attempted || !r1.Passed {
		t.Fatalf("r1 should attempt: %+v", r1)
	}
	if !r2.Attempted || !r2.Passed {
		t.Fatalf("r2 should attempt: %+v", r2)
	}
	if r3.Attempted || !r3.Skipped || r3.SkipReason != "budget_exhausted" {
		t.Fatalf("r3 should hit budget: %+v", r3)
	}
	if r3.FormatFeedback() != "" {
		t.Fatal("budget skip must be silent")
	}
	if h.TurnAttempts() != 2 {
		t.Fatalf("turnAttempts = %d want 2", h.TurnAttempts())
	}
	snap := enhancedmetrics.Snapshot()
	if snap.HarnessSkipped < 1 {
		t.Fatalf("expected budget skip metric: %+v", snap)
	}

	h.BeginTurn()
	if h.TurnAttempts() != 0 {
		t.Fatalf("BeginTurn should reset, got %d", h.TurnAttempts())
	}
	r4 := h.Verify(context.Background(), dir)
	if !r4.Attempted || !r4.Passed {
		t.Fatalf("after BeginTurn should attempt again: %+v", r4)
	}
}

func TestTurnBudgetUnlimited(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CustomCommand = "echo ok"
	cfg.PassCooldown = -1
	cfg.MaxAttemptsPerTurn = -1
	h := NewHarness(cfg)
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		r := h.Verify(context.Background(), dir)
		if !r.Attempted {
			t.Fatalf("unlimited budget skipped at i=%d: %+v", i, r)
		}
	}
}
