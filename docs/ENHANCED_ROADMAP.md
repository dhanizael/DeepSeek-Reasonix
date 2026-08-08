# Reasonix-Enhanced Roadmap

Fork-local plan for features that live on top of [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix).

**Binary:** `~/.local/bin/reasonix-enhanced` (build from this tree; not the npm `reasonix` package)  
**Branch:** `main-v2` (keep synced with `upstream/main-v2`)

---

## Mission (north star)

> **reasonix-enhanced** maximizes **prefix-cache hit rate and token efficiency** by keeping the static request prefix stable, while raising **precision, accuracy, and trust** through **host-side validation and verification** — not through longer model chatter.

### Dual pillars (priority order)

| # | Pillar | Meaning |
|---|--------|---------|
| **1** | **Cache hit maximal** | Optimal, efficient DeepSeek (and compatible) prefix reuse across long sessions. Stable system / tools / static policy. Enhanced logic lives in **tool results / disk / local process**, not in rewriting the system prefix each turn. |
| **2** | **Agent quality & intelligence** | Better than vanilla: more precise, accurate, to-the-point, trustworthy, fewer baseless assumptions and hallucinations — **without** trading away pillar 1. |

These are not a forced trade-off when quality comes from **evidence on the host** (syntax checks, scoped tests, checkpoint restore) instead of **more instructions in the prompt**.

### Product formula

```
vanilla upstream (latest)     = platform: projection, tools, checkpoint, compact
        +
reasonix-enhanced             = reliability layer: validate → verify → recover
        =
higher cache hit + lower wasted turns + more reliable outcomes
```

**Not** our goal: a second agent stack, dual undo, dual prefix enforcers, or “smarter” via ever-longer system prompts.

---

## Definition of Done — every enhanced change

A PR or commit that touches enhanced behavior **must** answer yes to the spirit of these gates:

| Gate | Pass if… | Fail if… |
|------|----------|----------|
| **G1 Cache** | Does **not** mutate system/tools/static prefix every turn; does not fight upstream projection | Rewrites system message, dual Anchor, dynamic tool schemas for “quality” |
| **G2 Token thrift** | Extra model-visible text is minimal, capped, or skippable; prefer **silent skip** when safe | Dumps large logs into tool results; forces extra API turns without reducing total tokens/task |
| **G3 Quality** | Increases **evidence** or **prevents bad writes/loops** (validate / verify / restore) | Makes the model “more confident” without evidence |
| **G4 Upstream respect** | Composes checkpoint, overlay, compact — no parallel production undo/prefix stack | Second ShadowStore undo path, EnforceAnchor on live path |
| **G5 Measurable** | Prefer local metrics (`~/.reasonix/enhanced-metrics.jsonl`) or clear behavior tests | “Feels better” only, no test or observable counter |

**Ship rule:** if a feature helps quality but **breaks G1**, redesign it (host-side or suffix-only) or drop it.

### Success metrics (directionally)

**Cache / efficiency**

- Stable prefix across turns; high cache hit on long sessions (UI/status where available)
- Lower tokens **per completed task** vs vanilla on similar work (fewer repair loops)

**Quality / trust**

- Fewer syntax-invalid writes reaching disk
- Fewer false “done” states when package tests fail (harness on)
- Fewer identical failing edit loops (backtrack)
- Answers more bound to tool evidence than assumption

---

## Feature decision gates (quick filter)

Before building anything enhanced, ask:

1. Does it change system/tools/prefix every turn? → **Reject** (boot-only exceptions only).
2. Does it add model text without new evidence? → **Reject** or silent/opt-in.
3. Does it add host evidence (validate, test, restore)? → **Consider**.
4. Does it force extra API turns? → Only if total tokens/task clearly drop (measure first).
5. Does it increase confidence without evidence? → **Reject**.

### Decision legend

| Decision | Meaning |
|----------|---------|
| **KEEP** | Unique value. Develop further; wire via config when needed. |
| **MERGE** | Idea is good, but implement **on top of upstream APIs** (don't dual-stack). |
| **DROP** | Upstream already superior, or prototype too thin. Quarantine or delete. |
| **KEEP-POLICY** | Small always-on policy difference (not a subsystem). |

---

## Matrix

| # | Enhanced feature | Status in binary today | Upstream equivalent | Decision | Why |
|---|------------------|------------------------|---------------------|----------|-----|
| 1 | **TUI watchdog stall = 5m** | **Always on** | Default ~10s + lifecycle watchdog | **KEEP-POLICY** | Idle prompt must not kill the process. Keep 5m + upstream `tuiWatchdogCancelGrace`. |
| 2 | **Verification harness** | **Live (mode=auto)** multi-module aware; package-scoped; skip in-flight/cooldown; **silent pass**; **turn budget**; capped fail feedback | Goal **verification evidence** (model-run checks) | **KEEP** | Host evidence without full-repo `./...`; token-thrift feedback. |
| 3 | **3-strike backtrack** | **With harness**; checkpoint preimage then git | `repeatFailureGuard` + checkpoint rewind | **MERGE** | Stop bad loops; restore via upstream store when possible. |
| 4 | **Shadow checkpoints** | **Quarantined** (deprecated; not on control path) | Production checkpoint + rewind | **DROP** dual-stack | Upstream wins; do not dual `/undo`. |
| 5 | **AST Syntax Guard** | **Live default-on** on write/edit/multi_edit | Atomic write + FileOverlay | **KEEP** | Content validation before commit; fewer broken-disk turns. |
| 6 | **Prefix Anchor Shield** | **Quarantined** (deprecated enforcer) | Cache-aware projection / `CoveredPrefixHash` | **DROP** enforcer | Upstream owns prefix; dual rewrite kills cache. |
| 7 | **Local enhanced metrics** | **Live** counters + `~/.reasonix/enhanced-metrics.jsonl` | Provider telemetry (different purpose) | **KEEP** | Zero API tokens; observe AST/harness/backtrack. |

---

## Priority order (shipped vs next)

```
DONE  P0  Watchdog 5m
DONE  P1  AST Syntax Guard (default on)
DONE  P2  Harness auto + package scope + thrift skips/caps
DONE  P3  Backtrack with checkpoint-preferring rollback
DONE  P4  ShadowStore quarantined
DONE  P5  AnchorShield enforcer quarantined
DONE  P6  Local metrics (token-free)
DONE  P7  Silent pass (default): zero tool-result text on harness pass;
         metrics harness_silent_pass; opt-out via silent_pass=false
DONE  P8  Budget gate: max harness shell-outs per Agent.Run (default 12);
         skip reason budget_exhausted (silent); reset via BeginTurn
DONE  P9  Tighter package detection / multi-module monorepos:
         nested go.mod → cd module + go test ./pkg; auto on go.work /
         depth-1 modules; bound findUp to workDir; skip non-Go inputs;
         python package scope via nearest pyproject/pytest markers

NEXT  (only if gates G1–G5 pass)
      - Never: system-prompt quality hacks, dual undo, dual prefix enforcers
```

---

## Target architecture (shipped)

```
  ┌──────────────────────────────────────────────────────────┐
  │  CACHE PATH (prefix-stable)                              │
  │  Upstream: system prompt, tools, projection, compact     │
  │  Enhanced: never rewrites this path                      │
  └────────────────────────────┬─────────────────────────────┘
                               │
  ┌────────────────────────────▼─────────────────────────────┐
  │  QUALITY PATH (host evidence, suffix / disk only)        │
  │  AST pre-write → scoped harness → backtrack+checkpoint   │
  │  Metrics: ~/.reasonix/enhanced-metrics.jsonl (local)     │
  └──────────────────────────────────────────────────────────┘
```

**Rules (non-negotiable)**

1. Never dual-run ShadowStore + upstream checkpoint for user-facing undo.
2. Never rewrite system/prefix messages outside upstream projection.
3. Harness **mode=auto** (Go + package scope); use `mode=off` when unwanted — never full-repo by default.
4. AST guard default **on** for Go/JSON (and light bracket checks for a few other langs).
5. One boot helper (`applyEnhancedConfig`) + `[enhanced.*]` in config.
6. Model-visible harness text stays **short / capped / skippable** (token thrift).

---

## Config sketch (current)

```toml
# reasonix-enhanced — see also reasonix.example.toml

[enhanced.ast_guard]
# enabled = true          # default on when omitted

[enhanced.harness]
# mode = "auto"           # auto|on|off — auto when go.mod present
# scope = "package"       # package|workspace
# command = ""            # optional override
# timeout_seconds = 45
# silent_pass = true      # default: no model-visible text on pass
# max_attempts_per_turn = 12  # shell-outs per Run; -1 unlimited

[enhanced.backtrack]
# enabled follows harness when omitted
# max_strikes = 3
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
