package checkpoint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestShadowStore(t *testing.T) {
	tempDir := t.TempDir()
	storeDir := filepath.Join(tempDir, "checkpoints")

	// Init git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Skip("git not available in test env")
	}

	exec.Command("git", "-C", tempDir, "config", "user.name", "test").Run()
	exec.Command("git", "-C", tempDir, "config", "user.email", "test@example.com").Run()

	// Initial commit
	file1 := filepath.Join(tempDir, "app.txt")
	_ = os.WriteFile(file1, []byte("v1 content\n"), 0644)
	exec.Command("git", "-C", tempDir, "add", ".").Run()
	exec.Command("git", "-C", tempDir, "commit", "-m", "v1").Run()

	store := NewShadowStore(storeDir)

	// Create mutation 1 and snapshot
	_ = os.WriteFile(file1, []byte("v2 mutation\n"), 0644)
	meta1, err := store.CreateSnapshot(context.Background(), tempDir, "edit file1")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if meta1.ID == "" {
		t.Fatal("expected non-empty snapshot ID")
	}

	// Create mutation 2
	_ = os.WriteFile(file1, []byte("v3 broken mutation\n"), 0644)

	// Restore latest snapshot (v2 state)
	restored, err := store.RestoreLatest(context.Background(), tempDir)
	if err != nil {
		t.Fatalf("RestoreLatest failed: %v", err)
	}
	if restored.ID != meta1.ID {
		t.Fatalf("expected restored ID %s, got %s", meta1.ID, restored.ID)
	}

	content, _ := os.ReadFile(file1)
	if string(content) != "v2 mutation\n" {
		t.Fatalf("expected 'v2 mutation\\n', got %q", string(content))
	}
}
