package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Result represents the outcome of an automated verification run.
type Result struct {
	Attempted bool
	Command   string
	Passed    bool
	ExitCode  int
	Output    string
	Duration  time.Duration
	Err       error
}

// Config controls verification harness execution behavior.
type Config struct {
	Enabled          bool          `json:"enabled"`
	CustomCommand    string        `json:"custom_command,omitempty"`
	Timeout          time.Duration `json:"timeout"`
	MaxOutputBytes   int           `json:"max_output_bytes"`
}

// DefaultConfig returns reasonable defaults for automated verification.
func DefaultConfig() Config {
	return Config{
		Enabled:        true,
		Timeout:        30 * time.Second,
		MaxOutputBytes: 4096,
	}
}

// Detector identifies test/verification frameworks present in a workspace.
type Detector struct{}

// NewDetector creates a new project detector.
func NewDetector() *Detector {
	return &Detector{}
}

// DetectCommand checks the workspace root for configuration files to infer the appropriate test command.
func (d *Detector) DetectCommand(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}

	// Go project
	if fileExists(filepath.Join(dir, "go.mod")) {
		return "go test ./...", true
	}

	// Node.js project (package.json)
	if fileExists(filepath.Join(dir, "package.json")) {
		if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
			return "pnpm test", true
		}
		if fileExists(filepath.Join(dir, "yarn.lock")) {
			return "yarn test", true
		}
		return "npm test", true
	}

	// Rust project
	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		return "cargo test", true
	}

	// Python project
	if fileExists(filepath.Join(dir, "pytest.ini")) || fileExists(filepath.Join(dir, "pyproject.toml")) || fileExists(filepath.Join(dir, "setup.py")) {
		return "pytest", true
	}

	// Makefile with test target
	if fileExists(filepath.Join(dir, "Makefile")) {
		return "make test", true
	}

	return "", false
}

// Harness executes automated verification loops after mutations.
type Harness struct {
	config   Config
	detector *Detector
}

// NewHarness constructs a verification harness.
func NewHarness(cfg Config) *Harness {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 4096
	}
	return &Harness{
		config:   cfg,
		detector: NewDetector(),
	}
}

// Verify runs the appropriate verification command in workDir and returns a structured Result.
func (h *Harness) Verify(ctx context.Context, workDir string) Result {
	if !h.config.Enabled {
		return Result{Attempted: false}
	}

	cmdStr := h.config.CustomCommand
	if cmdStr == "" {
		detected, ok := h.detector.DetectCommand(workDir)
		if !ok {
			return Result{Attempted: false}
		}
		cmdStr = detected
	}

	ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	startTime := time.Now()

	// Execute shell command
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = workDir

	out, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	res := Result{
		Attempted: true,
		Command:   cmdStr,
		Duration:  duration,
		Output:    truncateOutput(string(out), h.config.MaxOutputBytes),
	}

	if err == nil {
		res.Passed = true
		res.ExitCode = 0
	} else {
		res.Passed = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.ExitCode = -1
		}
		res.Err = err
	}

	return res
}

// FormatFeedback produces a clean markdown notice for the model.
func (r Result) FormatFeedback() string {
	if !r.Attempted {
		return ""
	}

	if r.Passed {
		return fmt.Sprintf("\n[Verification Harness] ✅ Auto-check passed (`%s` completed in %v)\n", r.Command, r.Duration.Round(time.Millisecond))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n[Verification Harness] ❌ Auto-check failed (`%s` exited with code %d in %v):\n", r.Command, r.ExitCode, r.Duration.Round(time.Millisecond)))
	sb.WriteString("```\n")
	sb.WriteString(strings.TrimSpace(r.Output))
	sb.WriteString("\n```\n")
	sb.WriteString("⚠️ Please inspect the verification failure output above and fix the issue in your next step.\n")

	return sb.String()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func truncateOutput(out string, maxBytes int) string {
	if len(out) <= maxBytes {
		return out
	}
	half := (maxBytes - 100) / 2
	return out[:half] + "\n\n... [Verification output truncated] ...\n\n" + out[len(out)-half:]
}
