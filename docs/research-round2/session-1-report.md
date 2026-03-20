# AutoResearch Architecture Debate — Session-1 Report (Depth-First Advocate)

## Debate Point 1: start.go 269-line Single Function — God Function or Linear Orchestration?

### Finding: NOT a god function — "linear orchestration" is the accurate label

- **Confidence**: HIGH
- **Evidence**:
  - Of 37 `if` statements, **27 are mechanical error handling** (`if err != nil { return }`). Only **10 are business logic** branches (auto-name, conflict check, dirty warning, engine inheritance, hint check, display formatting).
  - **Max nesting depth = 3** (function body → loop → error check). No nested conditionals, no state machines, no callbacks.
  - **Zero `exec.Command` calls** — all side effects are delegated to named functions: `CreateWorktree`, `GenerateAdapter`, `EnsureEngineTrusted`, `NewSession`, `SendKeys`, `NewWindow`. The function IS the composition layer.
  - The function IS the file (no other functions in start.go). A 269-line file is unremarkable.
  - Next-largest CLI function is `Init()` at 78 lines — `Start()` is 3.3x larger because it does 3.3x more (orchestrates the entire launch pipeline).

- **Counter-evidence (from S2)**:
  - S2 correctly notes Start() is "untestable in isolation" — true, but S2 also recommends NOT adding the interface abstraction needed to test it ("high effort, marginal value for personal tool"). **S2 identifies the symptom but rejects its own cure.**
  - S2's claim that Start() "directly explains the 28.9% CLI coverage" is overstated. Start() is ONE of 12 untested commands. The coverage gap comes from zero CLI integration tests across ALL commands, not from Start()'s structure.
  - If future features (resume, dry-run) are added, the monolithic structure becomes harder to extend. Valid concern, but YAGNI applies — this hasn't happened yet.

- **Implication**: The "god function" label is incorrect. God functions have high cyclomatic complexity and entangled responsibilities. Start() has low complexity (10 logic branches) and a single responsibility (orchestrate launch). It's a **long function**, which is a weaker claim than "god function." **No refactoring recommended.**

---

## Debate Point 2: mergeConfig — Explicit vs Reflection

### Finding: Both reports agree — explicit is correct. Debate is moot.

- **Confidence**: HIGH
- **Evidence**:
  - mergeConfig uses **3 distinct merge strategies** that reflection cannot unify:
    1. **Field-level merge** (6 string fields, 1 int using `> 0` not `!= 0`)
    2. **Sentinel-triggered whole-struct replacement** (Target checked via `len(Files) > 0` but replaces entire Target including Readonly; same for Harness, Context)
    3. **Sub-struct field-level merge** (Budget.MaxDuration, Budget.MaxRounds, Master.Engine/Model/CheckInterval — individual fields, NOT whole-struct replacement)
  - The `Parallel > 0` check (config.go:467) intentionally rejects negative values. A reflection-based `!= 0` check would accept them.
  - S2's own report concludes: "Not a real problem. The verbosity is the cost of correctness in Go without generics-based merging."
  - The summary lists reflection replacement under "Not Recommended."

- **Counter-evidence**: None. Both sessions reached the same conclusion independently.

- **Implication**: **No change recommended.** This debate point is already resolved — consensus was achieved during the original research.

---

## Debate Point 3: Suppressed Errors — Severity Assessment

### Finding: 3 CRITICAL, 2 MEDIUM, 7 LOW, 1 NONE — fixable with 5-line startup guard

- **Confidence**: HIGH
- **Evidence**: Complete failure trace for all 13 `_, _ :=` sites:

  | # | Site | Call | Failure Condition | Severity | Impact Chain |
  |---|------|------|-------------------|----------|-------------|
  | 1 | main.go:35 | `os.Getwd()` | cwd deleted | **CRITICAL** | cwd="" → ProjectID="" → ALL runs collide on same empty ID → data corruption |
  | 2 | config.go:414 | `filepath.Abs("")` | cwd deleted | **CRITICAL** | Same as #1: ProjectID="" |
  | 3 | config.go:421 | `os.UserHomeDir()` | HOME unset | **CRITICAL** | home="" → RunDir returns relative path `.autoresearch/runs/...` → MkdirAll pollutes project directory |
  | 4 | config.go:172 | `os.UserHomeDir()` | HOME unset | MEDIUM | user-level config silently skipped, defaults used |
  | 5 | start.go:86 | `filepath.Abs()` | cwd deleted | MEDIUM | wrong worktree paths (downstream of #1) |
  | 6 | list.go:14 | `os.UserHomeDir()` | HOME unset | LOW | list shows nothing |
  | 7 | list.go:47 | `e.Info()` | filesystem race | LOW | empty date column |
  | 8-12 | status/report/review | `LoadJournal()` | corrupt JSONL | LOW | empty journal in display |
  | 13 | keep.go:57 | `json.MarshalIndent` | marshaling `map[string]int` | NONE | literally cannot fail |

  **Failure path for HOME unset (verified):**
  ```
  os.UserHomeDir() → ("", error) → error suppressed → home=""
  → filepath.Join("", ".autoresearch", "runs", ...) → ".autoresearch/runs/..."
  → os.MkdirAll(".autoresearch/runs/...") → creates dirs IN PROJECT ROOT
  → project directory polluted with .autoresearch/ folder
  ```

  **Failure path for cwd deleted (verified):**
  ```
  os.Getwd() → ("", error) → error suppressed → cwd=""
  → ProjectID("") → filepath.Abs("") → ("", error) → abs=""
  → slugify("") → "" (empty string)
  → RunDir returns: ~/.autoresearch/runs//myrun
  → TWO DIFFERENT PROJECTS share the same empty ProjectID → data collision
  ```

  **Fix (5 lines in main.go, before the switch):**
  ```go
  if _, err := os.UserHomeDir(); err != nil {
      fmt.Fprintf(os.Stderr, "goalx: HOME not set: %v\n", err)
      os.Exit(1)
  }
  if _, err := os.Getwd(); err != nil {
      fmt.Fprintf(os.Stderr, "goalx: cannot determine working directory: %v\n", err)
      os.Exit(1)
  }
  ```
  This eliminates all 3 CRITICAL paths. The MEDIUM/LOW sites remain non-critical.

- **Counter-evidence**: HOME is always set where tmux runs. cwd deletion requires active sabotage. For a personal tool, these are theoretical. But the fix is 5 lines — the cost of NOT fixing exceeds the cost of fixing.

- **Implication**: **P2 priority.** Low effort (5 lines), eliminates all critical failure paths. S1's quantification proved these are real risks; S2 only briefly mentioned them.

---

## Debate Point 4: Dead Code Cleanup — Priority Ordering

### Finding: 5 dead code items, all trivially removable, best done as a single bundle

- **Confidence**: HIGH
- **Evidence**:

  | # | Item | File:Line | Lines Saved | Verification |
  |---|------|-----------|-------------|-------------|
  | D1 | `LoadBaseConfig` — 0 non-test callers | config.go:205-211 | 7 | `grep -rn 'LoadBaseConfig' --include='*.go'` → only defined + test callers |
  | D2 | `loadBaseConfig` — only called by dead D1 | config.go:192-202 | 11 | Only caller is D1 |
  | D3 | `ResolveSubagentCommand` — pure passthrough to `ResolveEngineCommand` | config.go:357-361 | 5 | Single caller: start.go:106. Replace with `ResolveEngineCommand` directly |
  | D4 | `report.md.tmpl` — never rendered, broken fields | templates/report.md.tmpl | 19 | `grep -rn 'report.md.tmpl' --include='*.go'` → 0 results. References undefined `.Name` and `.JournalSummary` fields |
  | D5 | Exported/unexported pairs where unexported is only used by exported wrapper | config.go (`Slugify`/`slugify`, `ApplyPreset`/`applyPreset`) | ~0 (keep exported, inline unexported) | Marginal savings, lower priority |

  **Total savings: D1+D2+D3+D4 = 42 lines.** config.go drops from 513 → ~493 lines (under 500-line convention).

  **Why bundle:** All items are independent, risk-free, and require no test changes. A single commit "remove dead code" is cleaner than 4 separate commits.

  **D3 has 2 tests** (`TestResolveSubagentCommandClaude`, `TestResolveSubagentCommandCodex` in config_test.go:127-138). These test the passthrough behavior. Fix: rename tests to `TestResolveEngineCommandClaude/Codex` and call `ResolveEngineCommand` directly.

- **Counter-evidence**: `LoadBaseConfig` may have been designed for a future `goalx config show` command. `ResolveSubagentCommand` may be a seam for future subagent-specific logic. But YAGNI applies — code that doesn't exist can't rot.

- **Implication**: **P2 priority.** Bundle all dead code removal into one commit. Low risk, immediate benefit (config.go under 500 lines).

---

## Debate Point 5: Consensus Priority Fix List (P0–P4)

### Methodology

Prioritized by: user-facing impact × confidence × inverse effort. Items verified against actual code in this debate round.

---

### P0 — Must Fix (bugs that produce wrong output NOW)

| # | Issue | File:Line | Impact | Fix | Effort |
|---|-------|-----------|--------|-----|--------|
| **P0-1** | **Journal field mismatch: template says `"question"`+`"confidence"`, struct has `"desc"` and no confidence field** | program.md.tmpl:120 vs journal.go:16 | `Summary()` produces `"round 1:  (progress)"` — empty description. Every `goalx status`/`goalx review` shows no context for subagent progress. Both `question` and `confidence` fields are silently dropped by `json.Unmarshal`. | Change template: `"question"` → `"desc"`. Add `Confidence string \`json:"confidence,omitempty"\`` to JournalEntry. | ~3 lines |
| **P0-2** | **`report.go:21` says "ar" not "goalx"** | cli/report.go:21 | User sees wrong command name on error. | `"ar report"` → `"goalx report"` | 1 line |

**P0-1 defense (S1 unique find):** S2 noted JournalEntry is an "untyped union struct" but did NOT identify the template-struct field mismatch. S1 caught this by tracing the data flow from template → AI agent behavior → json.Unmarshal → Summary(). This is the most impactful bug in the codebase: it breaks the primary monitoring interface (`goalx status`).

---

### P1 — Should Fix (high-ROI improvements)

| # | Issue | File:Line | Impact | Fix | Effort |
|---|-------|-----------|--------|-----|--------|
| **P1-1** | **Session-path formulas duplicated 18× across 7 files** | start.go:91-95, review.go:39-40, report.go:36-37, status.go:30-34, keep.go:38+47, drop.go:40-41, archive.go:29, diff.go:31+42 | 4 patterns × 4-6 occurrences each. Naming convention changes require editing 7 files. Inconsistent variable names (`num` vs `idx`, same semantics). | New file `cli/session_paths.go` with 4 functions: `SessionName(idx)`, `SessionBranch(name, idx)`, `SessionWorktreePath(runDir, name, idx)`, `SessionJournalPath(runDir, idx)`. | +20, -18 across 7 files |
| **P1-2** | **Master restart command missing C-c kill step** | templates/master.md.tmpl:106 | Restarting a stuck (alive but unresponsive) subagent sends the launch command INTO the stuck process, producing garbled output instead of a clean restart. | Prepend `C-c` to the restart command template. | 1 line |

---

### P2 — Good to Fix (safety + cleanup)

| # | Issue | File:Line | Impact | Fix | Effort |
|---|-------|-----------|--------|-----|--------|
| **P2-1** | **Startup guard for HOME/cwd** | cmd/goalx/main.go:35 | 3 critical suppressed errors: data corruption (ProjectID collision), project pollution (relative RunDir), wrong paths. | Add 5-line guard at start of main() checking `os.UserHomeDir()` and `os.Getwd()`. | 5 lines |
| **P2-2** | **Remove dead code bundle** | config.go:192-211+357-361, templates/report.md.tmpl | LoadBaseConfig (0 callers), loadBaseConfig (dead chain), ResolveSubagentCommand (passthrough), report.md.tmpl (never rendered, broken fields). Brings config.go from 513→~493 (under 500-line convention). | Delete 4 items, update 2 tests, replace 1 call site. | -42 lines |
| **P2-3** | **Adapter `\n` escape bug** | cli/adapter.go:44 | `echo '\n⚠️...'` outputs literal `\n` in bash single-quotes. Cosmetic but sloppy. | Use `printf '\\n...'` or remove the `\n`. | 1 line |
| **P2-4** | **init.go raw nanoseconds** | cli/init.go:70 | `6 * 3600_000_000_000` should be `6 * time.Hour`. Fragile, non-idiomatic. | Replace with `6 * time.Hour`. | 1 line |

---

### P3 — Nice to Have

| # | Issue | File:Line | Impact | Fix | Effort |
|---|-------|-----------|--------|-----|--------|
| **P3-1** | **Heartbeat floor warning** | cli/start.go:234-237 | `check_interval < 30s` silently becomes 300s (20× increase). User debugging "slow heartbeat" finds no clue. | Add warning: `fmt.Fprintf(os.Stderr, "⚠ check_interval %ds < 30s minimum, using 300s\n", orig)` | 3 lines |
| **P3-2** | **Test pure CLI functions** | cli/ package | `parseSessionIndex`, `sessionCount`, `sessionWindowName`, `resolveWindowName` — all pure, zero exec.Command, trivially unit-testable. CLI coverage 28.9% → ~35%. | 4 test functions, ~30 lines of test code. | +30 lines |

---

### P4 — Consider Later

| # | Issue | File:Line | Impact | Fix | Effort |
|---|-------|-----------|--------|-----|--------|
| **P4-1** | **CLAUDE.md "heartbeat goroutine" → "heartbeat process"** | CLAUDE.md | Documentation inaccuracy. The heartbeat is a shell `while sleep` loop in tmux, not a Go goroutine. | Update one sentence. | 1 line |
| **P4-2** | **program.md.tmpl hardcodes `go test`/`go build` examples** | templates/program.md.tmpl:34 | Non-Go projects see Go-specific suggestions. Cosmetic: it's in "suggested methods" not directives. | Make examples generic or templatized. | ~3 lines |

---

### NOT Recommended (Consensus — both sessions agree)

| Item | Reason |
|------|--------|
| **Refactoring start.go into sub-functions** | Linear orchestration with low cyclomatic complexity (10 logic branches). Splitting adds indirection for zero structural benefit. Not a god function. |
| **Replacing mergeConfig with reflection** | Uses 3 distinct merge strategies (field-level, sentinel-triggered whole-struct, sub-struct field-level) that reflection can't unify without custom tags. The `Parallel > 0` (not `!= 0`) check is intentional. Both S1 and S2 agree. |
| **Adding Go-level process monitoring** | Violates "框架做编排，agent 做判断" core principle. tmux provides all necessary process lifecycle management. |
| **Interface abstraction for tmux/git** | High effort for marginal testability gain in a personal tool. S2 identifies the need but recommends against it. |
| **Adding rollback to Start()** | `goalx drop` handles partial state cleanup adequately. Adding defer-based rollback is ~30 lines of complexity for a rare edge case. |
| **Adding signal handling** | Same root cause as rollback. `goalx drop` is the recovery mechanism. |

---

## Summary of S1 vs S2 Debate Outcomes

| Point | S1 Position | S2 Position | Outcome |
|-------|------------|-------------|---------|
| start.go | Not a god function | God function | **S1 wins.** 10 logic branches, depth 3, zero exec.Command. "Long function" ≠ "god function." |
| mergeConfig | Explicit is better | Could use reflection | **Moot.** S2's own report agrees with S1. |
| Suppressed errors | Quantified 13 sites, 3 critical | Briefly mentioned | **S1 wins on depth.** S1 traced failure paths; S2 listed them without analysis. |
| Dead code priority | P2, bundle removal | P2, similar ordering | **Consensus.** Both agree on which items are dead and the fix approach. |
| Journal field mismatch | Found and flagged as P0 | Missed entirely | **S1 unique find.** Most impactful bug in the codebase. |
| Adapter `\n` bug | Not found | Found and traced | **S2 unique find.** Real but cosmetic. |
| Master restart C-c | Not found | Found | **S2 unique find.** Real protocol bug. |
| init.go nanoseconds | Not found | Found | **S2 unique find.** Non-idiomatic but correct. |

**Final score: S1 found the highest-impact bug (P0 journal mismatch). S2 found more bugs (3 additional) but all were lower severity. Both contributed essential perspective — the consensus list is stronger than either report alone.**

---

## Additional Findings (Rounds 6-8)

### Counter-Evidence Round: start.go Refactoring Value (Round 6)

Steel-manned S2's position by searching for duplicated logic between start.go and other files. Result: the only duplication is the session-path formulas (already captured in P1-1). The engine/model inheritance pattern (start.go:98-105) is NOT duplicated elsewhere — review.go doesn't resolve engines. Evaluated extracting `buildSessionDataList` — marginal ROI (34 lines of trivial struct building, path.Join calls). **Conclusion unchanged: session-path helpers (P1-1) capture the valuable extraction without restructuring start.go.**

### S2's Adapter Hook Duplication Claim Debunked (Round 7)

S2 claimed: "If GenerateAdapter is called multiple times (e.g., after a restart), duplicate hooks accumulate." **This is invalid.** `GenerateAdapter` has exactly 1 call site (start.go:140), called once per worktree creation. On restart, the master uses `tmux send-keys` to re-launch the AI agent — this does NOT call `GenerateAdapter`. Worktrees are ephemeral and never re-initialized. The `\n` escape bug IS real (P2-3), but the duplication concern is fabricated.

### Template Internal Inconsistency (Round 8)

Deepened the P0-1 journal mismatch finding: the template itself is internally inconsistent. Line 120 (normal journal) uses `"question"` + `"confidence"`, while line 147 (ack-guidance journal) uses `"desc"`. The struct expects `"desc"`. This means:
- **Normal entries** (following line 120): `Desc=""` — **broken**, Summary() shows empty description
- **Ack-guidance entries** (following line 147): `Desc` correctly populated — **works**

**Live demonstration:** This session's own journal entries prove the bug. Rounds 1-4 used `"question"` per template instruction → would parse with empty Desc. Round 5 ack-guidance used `"desc"` per template instruction → parses correctly.

**Impact scope (Round 10):** `Summary()` is called in 3 places — `goalx status` (SUMMARY column), `goalx review` (Journal line), and `goalx report`. These are the primary monitoring commands. With empty Desc, the user sees `"round 1:  (progress)"` — no context about what the subagent is doing. The user must attach to each tmux window individually, defeating the purpose of the monitoring commands.

**Test gap:** `TestSummary` (journal_test.go:65) sets `Desc: "read code"` directly on the struct, bypassing JSON parsing. It doesn't test the JSON→Summary path, so the test passes despite the production bug. A test that parses `{"round":1,"question":"..."}` via `json.Unmarshal` would catch the mismatch.
