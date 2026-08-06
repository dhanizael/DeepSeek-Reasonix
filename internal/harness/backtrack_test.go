package harness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBacktrackGuard(t *testing.T) {
	guard := NewBacktrackGuard(3)

	// Strike 1
	strikes, triggered := guard.RecordFailure("file.go")
	if strikes != 1 || triggered {
		t.Fatalf("expected 1 strike, triggered=false; got %d, %v", strikes, triggered)
	}

	// Strike 2
	strikes, triggered = guard.RecordFailure("file.go")
	if strikes != 2 || triggered {
		t.Fatalf("expected 2 strikes, triggered=false; got %d, %v", strikes, triggered)
	}

	// Strike 3 (Triggers backtrack)
	strikes, triggered = guard.RecordFailure("file.go")
	if strikes != 3 || !triggered {
		t.Fatalf("expected 3 strikes, triggered=true; got %d, %v", strikes, triggered)
	}

	// Record success resets counter
	guard.RecordSuccess("file.go")
	if count := guard.GetStrikes("file.go"); count != 0 {
		t.Fatalf("expected 0 strikes after success reset, got %d", count)
	}
}

func TestBacktrackGuardRollbackGit(t *testing.T) {
	tempDir := t.TempDir()

	// Init git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Skip("git not working in test env")
	}

	// Config git user for test
	exec.Command("git", "-C", tempDir, "config", "user.name", "test").Run()
	exec.Command("git", "-C", tempDir, "config", "user.email", "test@example.com").Run()

	// Create and commit initial file
	filePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(filePath, []byte("clean state\n"), 0644); err != nil {
		t.Fatal(err)
	}
	exec.Command("git", "-C", tempDir, "add", ".").Run()
	exec.Command("git", "-C", tempDir, "commit", "-m", "initial").Run()

	// Mutate file to dirty state
	if err := os.WriteFile(filePath, []byte("broken edit\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Perform rollback
	guard := NewBacktrackGuard(3)
	_, err := guard.Rollback(context.Background(), tempDir, "test.txt")
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	// Verify file was restored to clean state
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "clean state\n" {
		t.Fatalf("expected 'clean state\\n', got %q", string(content))
	}

	directive := guard.FormatBacktrackDirective("test.txt")
	if !strings.Contains(directive, "3-Strike Backtrack Triggered") {
		t.Fatalf("unexpected directive output: %s", directive)
	}
}
