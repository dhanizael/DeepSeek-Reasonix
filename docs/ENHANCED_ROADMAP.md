# Reasonix-Enhanced Roadmap

Fork-local plan for features that live on top of [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix).

**Branch baseline:** `main-v2` @ post-upstream sync (`97271ce69`, Aug 2026)  
**Binary:** `~/.local/bin/reasonix-enhanced` (build from this tree; not the npm `reasonix` package)

## Decision legend

| Decision | Meaning |
|----------|---------|
| **KEEP** | Unique value. Develop further; wire opt-in via config. |
| **MERGE** | Idea is good, but implement **on top of upstream APIs** (don't dual-stack). |
| **DROP** | Upstream already superior, or prototype too thin. Remove or leave dead code until deleted. |
| **KEEP-POLICY** | Small always-on policy difference (not a subsystem). |

---

## Matrix

| # | Enhanced feature | Status in binary today | Upstream equivalent | Decision | Why |
|---|------------------|------------------------|---------------------|----------|-----|
| 1 | **TUI watchdog stall = 5m** | **Always on** | Default 10s + lifecycle watchdog (grace, phases) | **KEEP-POLICY** | Intentional UX: long idle prompt must not kill the process. Keep 5m + upstream `tuiWatchdogCancelGrace`. Document in fork notes; re-check on each upstream sync. |
| 2 | **Verification harness** (`internal/harness`) package-scoped post-edit checks | **Live (mode=auto)** for Go workspaces; `go test ./pkg` not `./...` | Goal/delivery **verification evidence** (model must run checks; not host auto-runner) | **KEEP** | Host-side automatic feedback loop, **scoped** to the edited package. Mode `auto|on|off`. |
| 3 | **3-strike backtrack + file rollback** (`BacktrackGuard`) | **Live with harness**; rollback prefers **checkpoint preimage**, git fallback | `repeatFailureGuard`; full **checkpoint rewind** | **MERGE** | Strikes after harness fail; restore via `PrepareFileRevert`/`CommitFileRevert` when observer store is live. |
| 4 | **Shadow checkpoints** (`internal/checkpoint/shadow.go`) | **Idle** (setter only; never called) | Production checkpoint + rewind + coverage + barrier | **DROP** (as parallel system) | Upstream is strictly better (preimages, coverage gaps, transactions). Optionally keep a thin **export/debug** helper later; do not wire a second `/undo`. |
| 5 | **AST Syntax Guard** (`internal/repair/ast_guard.go`) | **Idle** (no tool hook) | Atomic write + `FileOverlay` (safe I/O, not syntax) | **KEEP** | Complementary gap: validate **content** before write. Upstream does not parse Go/JSON for agent writes. Wire into write/edit path **after** overlay content is assembled, **before** atomic commit. Opt-in by language / config. |
| 6 | **Prefix Anchor Shield** (`internal/compaction/anchor_shield.go`) | **Idle** | Cache-aware **context projection**, `CoveredPrefixHash`, compact pipeline | **DROP** (as enforcer) | Upstream owns prefix stability end-to-end. Re-implementing `EnforceAnchor` risks fighting projection. If needed later: **telemetry-only** “prefix drift detector” using upstream hashes—not a second rewriter. |

---

## Priority order (recommended)

```
P0  KEEP-POLICY  Watchdog 5m          — already live; re-verify on every sync
P1  KEEP         AST Syntax Guard     — highest unique ROI; small surface
P2  KEEP         Verification harness — opt-in config; timeout + path scoping
P3  MERGE        Backtrack            — only after harness; rollback via checkpoints
P4  DROP         ShadowStore          — delete or quarantine; use upstream rewind
P5  DROP         AnchorShield enforcer— delete or reduce to metrics-only experiment
```

---

## Target architecture (when fully wired)

```
                    ┌─────────────────────────────────────┐
                    │  Upstream (always)                  │
                    │  projection / checkpoint / overlay  │
                    │  atomic writes / repeatFailureGuard │
                    │  goal verification evidence         │
                    └──────────────┬──────────────────────┘
                                   │
         ┌─────────────────────────┼─────────────────────────┐
         │ opt-in enhanced         │                         │
         ▼                         ▼                         ▼
   AST pre-write             Host harness              Backtrack policy
   (write/edit tools)        (after mutates)           (on harness fail)
         │                         │                         │
         │                         │              rollback → upstream
         │                         │              checkpoint/rewind API
         └─────────────────────────┴─────────────────────────┘
```

**Rules**

1. Never dual-run ShadowStore + upstream checkpoint for user-facing undo.
2. Never rewrite system/prefix messages outside upstream projection.
3. Harness default **off** (or off for large monorepos); require explicit config.
4. AST guard default **on for `.go`/`.json` only** once wired; other langs bracket-check optional.
5. All enhanced install points go through **one** boot/wire helper + `reasonix.toml` / `~/.reasonix` flags.

---

## Suggested config sketch (future)

```toml
# reasonix-enhanced only — not in upstream schema until contributed
[enhanced]
# watchdog is compile-time for now (5m); do not expose until needed

[enhanced.ast_guard]
enabled = true
languages = ["go", "json"]   # "brackets" for generic

[enhanced.harness]
enabled = false              # default off
timeout = "30s"
# command = ""               # empty = auto-detect
max_output_bytes = 4096
scope = "package"            # future: package | workspace (avoid go test ./... always)

[enhanced.backtrack]
enabled = false
max_strikes = 3
# rollback_backend = "checkpoint"  # never "git_shadow" once MERGE done
```

---

## Concrete work packages

### WP1 — Housekeeping (low risk)

- [ ] Mark `shadow.go` / `anchor_shield.go` as deprecated in comments (or move under `internal/enhanced/experimental/`).
- [ ] Stop claiming in notes that Shadow is “wired” if `CreateSnapshot` is never called.
- [ ] Add this file to fork README pointer (optional).

### WP2 — AST guard (KEEP)

- [ ] Hook `ValidateSyntax` in write/edit pipeline after content materialization, before atomic write.
- [ ] On reject: return tool error string to the model (do not hard-crash agent).
- [ ] Config + unit/integration tests for go/json/brackets.
- [ ] Ensure FileOverlay / ACP unsaved buffer path still works.

### WP3 — Harness (KEEP)

- [ ] Boot: `SetVerificationHarness` only if `[enhanced.harness].enabled`.
- [ ] Prefer package-scoped command when mutation path known (avoid full-repo `go test ./...`).
- [ ] Cap concurrency / skip if previous harness still running.
- [ ] Append feedback to tool result only (already sketched in `observeAfterMutation`).

### WP4 — Backtrack (MERGE)

- [ ] On N harness failures for same path: directive to model + **checkpoint rewind** of that path if coverage allows.
- [ ] Remove or rewrite `BacktrackGuard.Rollback` that shells out / git-checks without barrier.
- [ ] Coexist with `repeatFailureGuard` (different signals: tool-arg loop vs verify fail).

### WP5 — Drop path

- [ ] Delete or unexport `ShadowStore` production API; migrate any docs to upstream CHECKPOINTS.md.
- [ ] Delete `EnforceAnchor` usage plans; optional: one metric that compares system hash to boot snapshot using upstream projection fields.

---

## Sync hygiene (every upstream pull)

1. `git fetch upstream && git merge upstream/main-v2` on a dry-run branch first.
2. Re-check `tui_diagnostics.go` watchdog constants.
3. Re-check `observeAfterMutation` still compiles with agent refactors.
4. Rebuild:  
   `CGO_ENABLED=0 go build -o ~/.local/bin/reasonix-enhanced ./cmd/reasonix`
5. Keep one `.bak` binary until smoke test (TUI open, one edit, no crash).

---

## One-line summary

| Keep building | Borrow upstream | Stop dual-tracking |
|---------------|-----------------|--------------------|
| AST guard, opt-in harness, 5m watchdog | Backtrack rollback via checkpoints | ShadowStore, AnchorShield enforcer |

This keeps **reasonix-enhanced** as a thin, intentional fork—not a second agent stack fighting official Reasonix.
