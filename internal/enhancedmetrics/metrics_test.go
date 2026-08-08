package enhancedmetrics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountersAndPersist(t *testing.T) {
	Reset()
	dir := t.TempDir()
	path := filepath.Join(dir, "m.jsonl")
	SetPersistPath(path)
	t.Cleanup(func() { SetPersistPath(""); Reset() })

	RecordASTReject("a.go")
	RecordHarnessAttempt(true, "go test ./a", "package", 12)
	RecordHarnessAttempt(false, "go test ./b", "package", 40)
	RecordHarnessSkip("pass_cooldown", "go test ./a")
	RecordBacktrack("a.go")

	s := Snapshot()
	if s.ASTRejects != 1 || s.HarnessPassed != 1 || s.HarnessFailed != 1 || s.HarnessSkipped != 1 || s.BacktrackTriggered != 1 {
		t.Fatalf("snapshot = %+v", s)
	}
	if s.HarnessAttempted != 2 {
		t.Fatalf("attempted = %d", s.HarnessAttempted)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, kind := range []string{"ast_reject", "harness", "harness_skip", "backtrack"} {
		if !strings.Contains(text, kind) {
			t.Fatalf("persist missing %s in %s", kind, text)
		}
	}
}
