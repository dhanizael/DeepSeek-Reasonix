package enhancedmetrics

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// DiskSummary aggregates event kinds from the append-only JSONL log.
// Survives process restarts; process Snapshot() does not.
type DiskSummary struct {
	Path               string `json:"path,omitempty"`
	Exists             bool   `json:"exists"`
	Events             uint64 `json:"events"`
	ASTRejects         uint64 `json:"ast_rejects"`
	HarnessAttempted   uint64 `json:"harness_attempted"`
	HarnessPassed      uint64 `json:"harness_passed"`
	HarnessFailed      uint64 `json:"harness_failed"`
	HarnessSkipped     uint64 `json:"harness_skipped"`
	HarnessSilentPass  uint64 `json:"harness_silent_pass"`
	BacktrackTriggered uint64 `json:"backtrack_triggered"`
	ParseErrors        uint64 `json:"parse_errors,omitempty"`
}

// DefaultPersistPath returns ~/.reasonix/enhanced-metrics.jsonl (or REASONIX_HOME).
// Does not create the file.
func DefaultPersistPath(reasonixHome string) string {
	if reasonixHome == "" {
		return ""
	}
	return filepath.Join(reasonixHome, "enhanced-metrics.jsonl")
}

// ReadDiskSummary scans path and counts known event kinds. Missing file is OK.
func ReadDiskSummary(path string) DiskSummary {
	s := DiskSummary{Path: path}
	if path == "" {
		return s
	}
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	defer f.Close()
	s.Exists = true

	sc := bufio.NewScanner(f)
	// JSONL lines stay small; allow a bit of headroom for long commands.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev struct {
			Kind string         `json:"kind"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			s.ParseErrors++
			continue
		}
		s.Events++
		switch ev.Kind {
		case "ast_reject":
			s.ASTRejects++
		case "harness":
			s.HarnessAttempted++
			if passed, _ := ev.Data["passed"].(bool); passed {
				s.HarnessPassed++
			} else {
				s.HarnessFailed++
			}
		case "harness_skip":
			s.HarnessSkipped++
		case "harness_silent_pass":
			s.HarnessSilentPass++
		case "backtrack":
			s.BacktrackTriggered++
		}
	}
	return s
}

// ToCounters maps disk aggregates into Counters for shared formatters.
func (s DiskSummary) ToCounters() Counters {
	return Counters{
		ASTRejects:         s.ASTRejects,
		HarnessAttempted:   s.HarnessAttempted,
		HarnessPassed:      s.HarnessPassed,
		HarnessFailed:      s.HarnessFailed,
		HarnessSkipped:     s.HarnessSkipped,
		HarnessSilentPass:  s.HarnessSilentPass,
		BacktrackTriggered: s.BacktrackTriggered,
	}
}
