# Debate Report — Session-2 (Breadth + Adversarial Advocate)

**Objective:** Debate 5 disagreement points between S1 (depth-first) and S2 (breadth + adversarial), produce consensus priority fix list.
**Rounds:** 10 rounds of hypothesis-driven analysis
**Outcome:** Consensus achieved on all 5 points with verified evidence

---

## Minimal Viable Fix Set (7 lines, 5 minutes)

If only one batch of fixes can be made, these 5 deliver the most impact:

| Rank | Item | Lines | What it fixes |
|------|------|-------|---------------|
| 1 | P0-1: Journal `"question"`→`"desc"` + add `confidence` field | 3 | Blank summaries in `goalx status/review/report` |
| 2 | P1-3: Master restart add C-c before send-keys | 1 | Protocol correctness for stuck agents |
| 3 | P1-2: Adapter `echo`→`printf` for newline | 1 | Garbled guidance notification display |
| 4 | P0-2: `"ar report"`→`"goalx report"` | 1 | Error message consistency |
| 5 | P3-2: `6 * 3600_000_000_000`→`6 * time.Hour` | 1 | Code correctness |

---

## Debate Point 1: start.go 269-Line Single Function

### Positions
- **S1:** Acceptable. Single linear function, well-commented, readable.
- **S2 (me):** God function by metrics, but splitting is lower priority than originally claimed.

### Evidence

Quantitative analysis of `cli/start.go:14-269`:
- 269 lines, 1 function, 0% test coverage
- 38 side-effect call sites across 8 categories (filesystem: 6, tmux: 6, config: 5, validation: 4, adapter/trust: 3, protocol: 2, git: 1, output: 11)
- 24 `if err` checks, 28 `return` statements

S1's counter-evidence (verified): Of 37 if-blocks, 27 are mechanical error handling — only **10 are business logic**. The function's *essential* complexity is low despite high *accidental* complexity (Go error handling verbosity).

The function already delegates all exec.Command calls to helpers (tmux.go, worktree.go). It has 0 direct exec.Command calls.

### S1's "S2 contradicts itself" claim — rebutted

S1 argued: "S2 labels it a god function but recommends NOT adding the interface abstraction needed to test it." This is a false dichotomy. Extracting `buildSessionDataList` as a **pure function** makes 54% of the loop (6 of 11 operations) testable with zero mocking and zero interfaces. S2 advocates this extraction (P4-2) while correctly opposing full interface abstraction (high effort, low ROI). These positions are consistent.

### Verdict

**Draw.** S2 right on classification (god function by metrics), S1 right on priority (acceptable for current scope). Resolution: P4 improvement for testability, not urgent.

---

## Debate Point 2: mergeConfig — Explicit vs Reflection

### Positions
- **S1:** Explicit is correct, type-safe.
- **S2 (me):** Agree. Despite the debate prompt attributing "reflection could simplify" to S2, both reports independently concluded mergeConfig is acceptable.

### Evidence

`config.go:448-500` — 52 lines, 17 field merges. Each uses the correct zero-value check for its type (`string != ""`, `int > 0`, `[]string len() > 0`, `Duration > 0`). Reflection would lose these type-specific semantics.

### Verdict

**S1 wins (S2 concedes).** No disagreement in the actual analysis. Keep as-is.

---

## Debate Point 3: Suppressed Errors — Severity Assessment

### Positions
- **S1:** Quantified 43 bare returns + 13 suppressed errors with impact analysis. Proposed 5-line startup guard.
- **S2 (me):** Mentioned briefly as "A5" in original report. But found 3 real bugs S1 missed.

### Evidence

S1's error analysis is superior:
- 13 suppressed errors classified by impact (CRITICAL/MEDIUM/LOW/NONE)
- 5 critical-path `os.UserHomeDir()`/`os.Getwd()` sites that produce cryptic failures
- Elegant fix: check HOME + cwd once at main() startup (~5 lines)

S2's additional bugs S1 did not identify:
- **Adapter `\n` bug** (adapter.go:44): Verified — `echo '\n...'` outputs literal `\n`. Fix: use `printf`.
- **Master restart missing C-c** (master.md.tmpl:106): Confirmed — restart command into a stuck (alive but unresponsive) agent produces garbled input. The "dead" case works correctly because shell prompt is visible.
- **Raw nanoseconds** (init.go:70): `6 * 3600_000_000_000` should be `6 * time.Hour`.

### Severity revision after verification

- Adapter `\n`: **P1** (cosmetic — guidance mechanism still works, just garbled display)
- Master C-c: **P1** (only affects "unresponsive" subcase, not "dead")
- Both downgraded from original P0 after master challenged severity assessment

### Verdict

**Both win.** S1 deeper on systematic error analysis. S2 found 3 specific real bugs. Both sets belong in the fix list.

---

## Debate Point 4: Dead Code Cleanup Priority

### Agreed inventory (both sessions)

| Item | Lines | Status |
|------|-------|--------|
| `LoadBaseConfig` (config.go:204-211) | 6 | Zero callers (verified: only called by dead `loadBaseConfig`) |
| `loadBaseConfig` (config.go:192-201) | 10 | Only caller is dead `LoadBaseConfig` |
| `ResolveSubagentCommand` (config.go:357-360) | 4 | Pure passthrough to `ResolveEngineCommand` (1 caller: start.go:106, 2 test callers) |
| `report.md.tmpl` | 18 | Never rendered. References undefined fields: `{{.Name}}` (should be `SessionName`), `{{.JournalSummary}}` (doesn't exist) |

Total: ~38 lines removable. Brings config.go from 513 → ~473 lines (well under 500-line convention).

S2's additional precision: `report.md.tmpl` isn't just dead — it references **wrong** fields. Even if wired up, it would silently render empty strings.

### Verdict

**S2 slightly** (more precise on report.md.tmpl analysis). Bundle as single P2 action.

---

## Debate Point 5: Journal Field Mismatch (S1 found, S2 missed)

### The bug

`program.md.tmpl:120` tells AI agents to write:
```json
{"round":1,"commit":"abc1234","question":"...","finding":"...","confidence":"HIGH","status":"progress"}
```

But `JournalEntry` (journal.go:12-26) has `"desc"` (not `"question"`) and no `"confidence"` field.

### Live proof from this run

Parsed session-1's actual journal entry:
```
Captured:  round=1, commit="b598e43", finding="...", status="progress"
DROPPED:   question="Is start.go's...", confidence="HIGH"
Summary(): "round 1:  (progress)"  ← BLANK DESCRIPTION
```

### Template is internally inconsistent

The template uses `"question"` in the main journal format (line 120) but `"desc"` in the guidance acknowledgement format (later). The struct was designed for `"desc"` — the main example is wrong.

### Impact chain

Affects all 3 monitoring commands: `goalx status` (status.go:49), `goalx review` (review.go:53), `goalx report` (report.go) — all call `Summary()` which reads `last.Desc`, always empty.

### Fix

Change template `"question"` → `"desc"`. Add `Confidence string \`json:"confidence,omitempty"\`` to struct. Tests already use `"desc"` (journal_test.go:12-14) — no test changes needed.

### Verdict

**S1 wins.** Critical find that S2 completely missed. True P0.

---

## Summary.md Corrections

The Round 1 summary (`docs/research-round1/summary.md`) has 3 errors this debate corrects:

1. **Missing #1 bug:** Summary lists P0 as rename bug. Omits journal field mismatch entirely.
2. **False claim:** "100% template-struct field coverage for active templates" — false because `program.md.tmpl:120` has mismatched `"question"` and `"confidence"` fields.
3. **File undercount:** "18 times across 7 files" — verified as **8 files** (archive.go was missed).

---

## Consensus Priority Fix List

### P0 — Immediate user-facing bugs
| # | Issue | File | Fix |
|---|-------|------|-----|
| P0-1 | Journal field mismatch: template `"question"`→`"desc"`, add `confidence` to struct | journal.go + program.md.tmpl | 3 lines |
| P0-2 | Rename bug: `"ar report"` → `"goalx report"` | cli/report.go:23 | 1 line |

### P1 — Real bugs + high-ROI refactoring
| # | Issue | File | Fix |
|---|-------|------|-----|
| P1-1 | Session-path helpers (18 duplications across 8 files) | new cli/session_paths.go + 8 files | +20/-18 lines |
| P1-2 | Adapter `\n` cosmetic bug: `echo` → `printf` | cli/adapter.go:44 | 1 line |
| P1-3 | Master restart missing C-c (affects unresponsive case only) | templates/master.md.tmpl:106 | 1 line |

### P2 — Dead code cleanup
| # | Issue | File | Lines saved |
|---|-------|------|-------------|
| P2-1 | Remove `LoadBaseConfig` + `loadBaseConfig` | config.go:192-211 | -16 |
| P2-2 | Replace `ResolveSubagentCommand` passthrough | config.go:357-360, start.go:106, config_test.go:127-145 | -4 (+ rename 2 test callers) |
| P2-3 | Delete `report.md.tmpl` | templates/report.md.tmpl | -18 |
| P2-4 | Delete `ResolveHarness` (zero callers, zero tests) | cli/harness.go | -11 |

### P3 — Robustness improvements
| # | Issue | File | Fix |
|---|-------|------|-----|
| P3-1 | Startup guard: check HOME + cwd at main() | cmd/goalx/main.go | +5 lines |
| P3-2 | Fix nanosecond literal → `6 * time.Hour` | cli/init.go:70 | 1 line |
| P3-3 | Heartbeat floor warning when `< 30s` becomes 300s | cli/start.go:235 | +3 lines |

### P4 — Desirable improvements
| # | Issue | Fix |
|---|-------|-----|
| P4-1 | Test pure CLI functions (`parseSessionIndex`, etc.) | +30 lines |
| P4-2 | Extract `buildSessionDataList` from start.go | +36 lines |
| P4-3 | Fix CLAUDE.md "goroutine" → "process" | 1 line |

### NOT recommended (consensus)
1. Replace mergeConfig with reflection — loses type safety (BOTH AGREE)
2. Add Go-level process monitoring — violates "框架做编排，agent 做判断" (BOTH AGREE)
3. Add rollback/defer to Start() — `goalx drop` handles cleanup (S1 position, S2 concedes)
4. Interface abstraction for tmux/git — high effort, low ROI for personal tool (BOTH AGREE)

---

## Implementation Order

**File overlap:** `cli/report.go` (P0-2 + P1-1), `cli/start.go` (P1-1 + P2-1 + P3-3)

```
Batch 1 (parallel-safe, no file overlap):
  P0-1: journal.go + program.md.tmpl
  P1-2: adapter.go
  P1-3: master.md.tmpl
  P3-1: cmd/goalx/main.go
  P3-2: cli/init.go

Batch 2 (sequential, shared files):
  P2-1: config.go dead code removal    (changes API surface first)
  P1-1: session-path helpers + callers  (touches 8 files)
  P0-2: report.go rename               (same file as P1-1)
  P3-3: start.go heartbeat warning     (same file as P1-1)
```

**P1-1 test modification verification:** `grep` for session-path patterns in `*_test.go` returns 0 results. Tests use `initGitRepo(t)` with temp dirs, never construct session paths directly. Zero test modifications needed.

---

## Debate Scorecard

| Point | Winner | Resolution |
|-------|--------|------------|
| start.go god function | Draw | God function by metrics, acceptable by priority. P4. |
| mergeConfig | S1 (S2 concedes) | Keep as-is. No action. |
| Suppressed errors | Both | S1 deeper on analysis, S2 found 3 real bugs. Merge findings. |
| Dead code priority | S2 slightly | Bundle as P2. S2 more precise on report.md.tmpl. |
| Journal field mismatch | S1 | Critical P0 bug S2 missed entirely. |

**Overall:** S1 wins on depth (journal mismatch, error quantification). S2 wins on breadth (adapter, C-c, nanosecond bugs). The consensus list is stronger than either individual report.
