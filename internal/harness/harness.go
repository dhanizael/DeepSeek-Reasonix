package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"reasonix/internal/enhancedmetrics"
)

// Scope controls how broad verification runs when a mutation path is known.
const (
	// ScopePackage runs the smallest useful check for the edited package/dir
	// (e.g. go test ./internal/foo for a file under that package). Default.
	ScopePackage = "package"
	// ScopeWorkspace runs the full workspace command (e.g. go test ./...).
	ScopeWorkspace = "workspace"
)

// DefaultMaxOutputBytes caps harness stdout embedded into tool results (model
// context). Keep small for token thrift; full logs stay local in the process.
const DefaultMaxOutputBytes = 2048

// DefaultPassCooldown is how long to skip re-running the same command after a pass.
// Bursts of multi_edit / chained writes in one package should not re-tax CPU
// or re-append identical pass notices into the transcript.
const DefaultPassCooldown = 3 * time.Second

// Result represents the outcome of an automated verification run.
type Result struct {
	Attempted  bool
	Skipped    bool
	SkipReason string
	Command    string
	Passed     bool
	ExitCode   int
	Output     string
	Duration   time.Duration
	Err        error
	Scope      string // package|workspace|custom
}

// Config controls verification harness execution behavior.
type Config struct {
	Enabled        bool          `json:"enabled"`
	CustomCommand  string        `json:"custom_command,omitempty"`
	Timeout        time.Duration `json:"timeout"`
	MaxOutputBytes int           `json:"max_output_bytes"`
	// Scope is "package" (default) or "workspace". Ignored when CustomCommand is set.
	Scope string `json:"scope,omitempty"`
	// PassCooldown skips re-running the same command after a recent pass.
	// 0 uses DefaultPassCooldown; negative disables cooldown.
	PassCooldown time.Duration `json:"pass_cooldown,omitempty"`
}

// DefaultConfig returns defaults tuned for reasonix-enhanced: package-scoped
// checks, tight output cap (token thrift), hard timeout.
func DefaultConfig() Config {
	return Config{
		Enabled:        true,
		Timeout:        45 * time.Second,
		MaxOutputBytes: DefaultMaxOutputBytes,
		Scope:          ScopePackage,
		PassCooldown:   DefaultPassCooldown,
	}
}

// Detector identifies test/verification frameworks present in a workspace.
type Detector struct{}

// NewDetector creates a new project detector.
func NewDetector() *Detector {
	return &Detector{}
}

// DetectCommand checks the workspace root for configuration files to infer the
// full-workspace verification command.
func (d *Detector) DetectCommand(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}

	if fileExists(filepath.Join(dir, "go.mod")) {
		return "go test ./...", true
	}

	if fileExists(filepath.Join(dir, "package.json")) {
		if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
			return "pnpm test", true
		}
		if fileExists(filepath.Join(dir, "yarn.lock")) {
			return "yarn test", true
		}
		return "npm test", true
	}

	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		return "cargo test", true
	}

	if fileExists(filepath.Join(dir, "pytest.ini")) || fileExists(filepath.Join(dir, "pyproject.toml")) || fileExists(filepath.Join(dir, "setup.py")) {
		return "pytest", true
	}

	if fileExists(filepath.Join(dir, "Makefile")) {
		return "make test", true
	}

	return "", false
}

// DetectScopedCommand builds a package/dir-scoped command for mutationPath when
// possible. Falls back to DetectCommand when scoping is not available.
func (d *Detector) DetectScopedCommand(workDir, mutationPath string) (cmd string, scope string, ok bool) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		workDir = "."
	}
	mutationPath = strings.TrimSpace(mutationPath)

	if mutationPath == "" {
		c, ok := d.DetectCommand(workDir)
		return c, ScopeWorkspace, ok
	}

	absMut := mutationPath
	if !filepath.IsAbs(absMut) {
		absMut = filepath.Join(workDir, mutationPath)
	}
	absMut = filepath.Clean(absMut)

	if _, pkgRel, found := goPackageForFile(absMut); found {
		if pkgRel == "" || pkgRel == "." {
			return "go test .", ScopePackage, true
		}
		return "go test ./" + filepath.ToSlash(pkgRel), ScopePackage, true
	}

	if pkgDir, found := findUp(absMut, "package.json"); found {
		var runner string
		if fileExists(filepath.Join(pkgDir, "pnpm-lock.yaml")) {
			runner = "pnpm test"
		} else if fileExists(filepath.Join(pkgDir, "yarn.lock")) {
			runner = "yarn test"
		} else {
			runner = "npm test"
		}
		return fmt.Sprintf("cd %s && %s", shellSingleQuote(pkgDir), runner), ScopePackage, true
	}

	if crateDir, found := findUp(absMut, "Cargo.toml"); found {
		return fmt.Sprintf("cd %s && cargo test", shellSingleQuote(crateDir)), ScopePackage, true
	}

	c, ok := d.DetectCommand(workDir)
	return c, ScopeWorkspace, ok
}

// Harness executes automated verification loops after mutations.
type Harness struct {
	config   Config
	detector *Detector

	mu        sync.Mutex
	inFlight  map[string]struct{} // key: workDir\x00cmd
	lastPass  map[string]time.Time
	passCD    time.Duration
}

// NewHarness constructs a verification harness.
func NewHarness(cfg Config) *Harness {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 45 * time.Second
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = DefaultMaxOutputBytes
	}
	if cfg.MaxOutputBytes > 4096 {
		// Hard ceiling so config cannot dump huge logs into model context.
		cfg.MaxOutputBytes = 4096
	}
	cfg.Scope = normalizeScope(cfg.Scope)
	cd := cfg.PassCooldown
	if cd == 0 {
		cd = DefaultPassCooldown
	}
	return &Harness{
		config:   cfg,
		detector: NewDetector(),
		inFlight: make(map[string]struct{}),
		lastPass: make(map[string]time.Time),
		passCD:   cd,
	}
}

// Verify runs verification for the whole workDir (workspace scope).
func (h *Harness) Verify(ctx context.Context, workDir string) Result {
	return h.VerifyPath(ctx, workDir, "")
}

// VerifyPath runs verification scoped to mutationPath when Config.Scope is
// package (default). CustomCommand always wins and runs at workDir.
//
// Token thrift: skips when the same command is already running, or passed
// within PassCooldown (no extra tool-result text, local metric only).
func (h *Harness) VerifyPath(ctx context.Context, workDir, mutationPath string) Result {
	if h == nil || !h.config.Enabled {
		return Result{Attempted: false}
	}

	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		workDir = "."
	}

	var cmdStr, usedScope string
	if strings.TrimSpace(h.config.CustomCommand) != "" {
		cmdStr = h.config.CustomCommand
		usedScope = "custom"
	} else if normalizeScope(h.config.Scope) == ScopeWorkspace || strings.TrimSpace(mutationPath) == "" {
		c, ok := h.detector.DetectCommand(workDir)
		if !ok {
			return Result{Attempted: false}
		}
		cmdStr = c
		usedScope = ScopeWorkspace
	} else {
		c, sc, ok := h.detector.DetectScopedCommand(workDir, mutationPath)
		if !ok {
			return Result{Attempted: false}
		}
		cmdStr = c
		usedScope = sc
	}

	key := workDir + "\x00" + cmdStr

	if skip, reason := h.beginRun(key); skip {
		enhancedmetrics.RecordHarnessSkip(reason, cmdStr)
		return Result{
			Attempted:  false,
			Skipped:    true,
			SkipReason: reason,
			Command:    cmdStr,
			Scope:      usedScope,
		}
	}
	defer h.endRun(key)

	ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	startTime := time.Now()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = workDir

	out, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	res := Result{
		Attempted: true,
		Command:   cmdStr,
		Duration:  duration,
		Output:    truncateOutput(string(out), h.config.MaxOutputBytes),
		Scope:     usedScope,
	}

	if err == nil {
		res.Passed = true
		res.ExitCode = 0
		h.notePass(key)
	} else {
		res.Passed = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.ExitCode = -1
		}
		res.Err = err
	}

	enhancedmetrics.RecordHarnessAttempt(res.Passed, cmdStr, usedScope, duration.Milliseconds())
	return res
}

func (h *Harness) beginRun(key string) (skip bool, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, busy := h.inFlight[key]; busy {
		return true, "already_running"
	}
	if h.passCD > 0 {
		if t, ok := h.lastPass[key]; ok && time.Since(t) < h.passCD {
			return true, "pass_cooldown"
		}
	}
	h.inFlight[key] = struct{}{}
	return false, ""
}

func (h *Harness) endRun(key string) {
	h.mu.Lock()
	delete(h.inFlight, key)
	h.mu.Unlock()
}

func (h *Harness) notePass(key string) {
	if h.passCD < 0 {
		return
	}
	h.mu.Lock()
	h.lastPass[key] = time.Now()
	h.mu.Unlock()
}

// FormatFeedback produces a short markdown notice for the model.
// Skipped runs return empty string (zero transcript tokens).
func (r Result) FormatFeedback() string {
	if r.Skipped || !r.Attempted {
		return ""
	}

	scopeNote := ""
	if r.Scope != "" && r.Scope != "custom" {
		scopeNote = fmt.Sprintf(" [%s]", r.Scope)
	}

	if r.Passed {
		// One line only — enough for the model, minimal tokens.
		return fmt.Sprintf("\n[Verification Harness]%s ✅ passed (`%s`, %v)\n",
			scopeNote, r.Command, r.Duration.Round(time.Millisecond))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n[Verification Harness]%s ❌ failed (`%s`, code %d, %v):\n",
		scopeNote, r.Command, r.ExitCode, r.Duration.Round(time.Millisecond)))
	// Prefer tail of failure output (errors usually last); already byte-capped.
	out := strings.TrimSpace(r.Output)
	if out != "" {
		sb.WriteString("```\n")
		sb.WriteString(out)
		sb.WriteString("\n```\n")
	}
	sb.WriteString("Fix the failure above; do not repeat the same broken patch.\n")
	return sb.String()
}

func normalizeScope(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ScopeWorkspace, "full", "all":
		return ScopeWorkspace
	default:
		return ScopePackage
	}
}

func goPackageForFile(absFile string) (modRoot, pkgRel string, ok bool) {
	dir := absFile
	if info, err := os.Stat(absFile); err == nil && !info.IsDir() {
		dir = filepath.Dir(absFile)
	}
	modRoot, found := findUp(dir, "go.mod")
	if !found {
		return "", "", false
	}
	rel, err := filepath.Rel(modRoot, dir)
	if err != nil {
		return modRoot, ".", true
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return modRoot, ".", true
	}
	return modRoot, rel, true
}

func findUp(start, name string) (dir string, ok bool) {
	dir = start
	if info, err := os.Stat(start); err == nil && !info.IsDir() {
		dir = filepath.Dir(start)
	}
	for {
		if fileExists(filepath.Join(dir, name)) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func truncateOutput(out string, maxBytes int) string {
	if maxBytes <= 0 || len(out) <= maxBytes {
		return out
	}
	// Keep the tail: test failures usually print the assertion last.
	if maxBytes < 80 {
		return out[len(out)-maxBytes:]
	}
	head := maxBytes / 4
	tail := maxBytes - head - 40
	if tail < 1 {
		return out[len(out)-maxBytes:]
	}
	return out[:head] + "\n\n... [truncated for token thrift] ...\n\n" + out[len(out)-tail:]
}
