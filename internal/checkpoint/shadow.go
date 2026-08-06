package checkpoint

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SnapshotMeta stores metadata about an atomic shadow checkpoint.
type SnapshotMeta struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Timestamp time.Time `json:"timestamp"`
	Diff      string    `json:"diff"`
}

// ShadowStore manages lightweight shadow checkpoints for atomic rollback and /undo capabilities.
type ShadowStore struct {
	mu        sync.Mutex
	baseDir   string
	snapshots []SnapshotMeta
}

// NewShadowStore initializes a shadow checkpoint store under baseDir.
func NewShadowStore(baseDir string) *ShadowStore {
	if baseDir == "" {
		baseDir = ".reasonix/checkpoints"
	}
	_ = os.MkdirAll(baseDir, 0755)
	return &ShadowStore{
		baseDir:   baseDir,
		snapshots: make([]SnapshotMeta, 0),
	}
}

// CreateSnapshot captures current uncommitted git changes as an atomic shadow checkpoint.
func (s *ShadowStore) CreateSnapshot(ctx context.Context, workDir string, label string) (SnapshotMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Capture git diff
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: if not in git repo, capture empty diff
		out = []byte("")
	}

	diffText := string(out)
	timestamp := time.Now()
	snapshotID := fmt.Sprintf("chk-%d", timestamp.UnixNano())

	meta := SnapshotMeta{
		ID:        snapshotID,
		Label:     label,
		Timestamp: timestamp,
		Diff:      diffText,
	}

	// Save snapshot patch to disk
	patchPath := filepath.Join(s.baseDir, snapshotID+".patch")
	_ = os.WriteFile(patchPath, out, 0644)

	s.snapshots = append(s.snapshots, meta)
	return meta, nil
}

// RestoreLatest reverts working directory to the most recent shadow checkpoint.
func (s *ShadowStore) RestoreLatest(ctx context.Context, workDir string) (SnapshotMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.snapshots) == 0 {
		return SnapshotMeta{}, fmt.Errorf("no shadow checkpoints available to restore")
	}

	latest := s.snapshots[len(s.snapshots)-1]

	// Revert uncommitted changes
	cmd := exec.CommandContext(ctx, "git", "checkout", "HEAD", "--", ".")
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return latest, fmt.Errorf("failed to revert changes: %w (output: %s)", err, string(out))
	}

	// If there was a patch in latest snapshot, apply it
	if strings.TrimSpace(latest.Diff) != "" {
		patchPath := filepath.Join(s.baseDir, latest.ID+".patch")
		applyCmd := exec.CommandContext(ctx, "git", "apply", patchPath)
		applyCmd.Dir = workDir
		_, _ = applyCmd.CombinedOutput()
	}

	return latest, nil
}

// ListSnapshots returns all captured shadow snapshots.
func (s *ShadowStore) ListSnapshots() []SnapshotMeta {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]SnapshotMeta, len(s.snapshots))
	copy(result, s.snapshots)
	return result
}
