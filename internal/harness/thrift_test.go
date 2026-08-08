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

func TestFormatFeedbackShortPass(t *testing.T) {
	r := Result{Attempted: true, Passed: true, Command: "go test ./pkg", Duration: time.Millisecond, Scope: ScopePackage}
	fb := r.FormatFeedback()
	if strings.Count(fb, "\n") > 3 {
		t.Fatalf("pass feedback should be minimal lines: %q", fb)
	}
	if len(fb) > 200 {
		t.Fatalf("pass feedback too long for token thrift: %d", len(fb))
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
