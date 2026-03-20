# Debate Summary — AutoResearch Architecture Review Consensus

**Run**: debate | **Sessions**: 2 (S1 depth-first, S2 breadth+adversarial) | **Rounds**: S1×9, S2×15 | **Outcome**: Full consensus on all 5 debate points

## Consensus P0-P4 Fix List

### P0 — Must Fix Now (bugs producing wrong output)
| # | Issue | File:Line | Effort |
|---|-------|-----------|--------|
| P0-1 | **Journal field mismatch**: template says `"question"`+`"confidence"`, struct has `"desc"` and no confidence. `goalx status/review/report` all show blank descriptions. Live-proven in this run. | journal.go:16 + program.md.tmpl:120 | 3 lines |
| P0-2 | **Rename bug**: `"ar report"` → `"goalx report"` | cli/report.go:21-23 | 1 line |

### P1 — Should Fix Next (real bugs + high-ROI refactoring)
| # | Issue | File:Line | Effort |
|---|-------|-----------|--------|
| P1-1 | **Session-path helpers**: 18 duplicated formulas across 8 files → 4 helper functions | new cli/session_paths.go + 8 callers | +20/-18 |
| P1-2 | **Adapter `\n` bug**: `echo '\n...'` outputs literal `\n` → use `printf` | cli/adapter.go:44 | 1 line |
| P1-3 | **Master restart missing C-c**: restart into stuck agent produces garbled input | templates/master.md.tmpl:106 | 1 line |

### P2 — Dead Code Cleanup (bundle as single commit)
| # | Issue | File:Line | Lines saved |
|---|-------|-----------|-------------|
| P2-1 | Remove `LoadBaseConfig` + `loadBaseConfig` (zero callers) | config.go:192-211 | -16 |
| P2-2 | Replace `ResolveSubagentCommand` passthrough + rename 2 tests | config.go:357-360 | -4 |
| P2-3 | Delete `report.md.tmpl` (never rendered, wrong fields) | templates/report.md.tmpl | -18 |

### P3 — Robustness
| # | Issue | File:Line | Effort |
|---|-------|-----------|--------|
| P3-1 | **Startup guard**: check HOME + cwd at main(), fail fast | cmd/goalx/main.go | 5 lines |
| P3-2 | **Nanosecond literal**: `6 * 3600_000_000_000` → `6 * time.Hour` | cli/init.go:70 | 1 line |
| P3-3 | **Heartbeat floor warning**: silent 300s override when < 30s | cli/start.go:235 | 3 lines |

### P4 — Desirable
| # | Issue | Effort |
|---|-------|--------|
| P4-1 | Test pure CLI functions (parseSessionIndex, etc.) | 30 lines |
| P4-2 | Extract `buildSessionDataList` from start.go for testability | 36 lines |
| P4-3 | Fix CLAUDE.md "heartbeat goroutine" → "heartbeat process" | 1 line |

### NOT Recommended (both sessions agree)
1. Replace mergeConfig with reflection — loses type safety, 3 distinct strategies can't be unified
2. Add Go-level process monitoring — violates "框架做编排，agent 做判断"
3. Add rollback/defer to Start() — `goalx drop` handles cleanup
4. Interface abstraction for tmux/git — high effort, low ROI for personal tool

## Debate Scorecard

| Point | Winner | Resolution |
|-------|--------|------------|
| start.go god function | **Draw** | S2 right on classification (38 side-effects, 8 categories), S1 right on priority (10 logic branches = low essential complexity). P4. |
| mergeConfig | **S1** (S2 concedes) | Both independently concluded explicit is correct. No action. |
| Suppressed errors | **Both** | S1 classified all 13 sites with severity. S2 found 3 bugs S1 missed (adapter, C-c, nanoseconds). |
| Dead code priority | **S2 slightly** | Same inventory, S2 more precise on report.md.tmpl field analysis. Bundle as P2. |
| Journal field mismatch | **S1** | Most impactful bug in codebase. S2 missed it entirely. |

**Overall**: S1 won on depth (journal mismatch, error quantification). S2 won on breadth (3 additional bugs, implementation batching). The consensus list is stronger than either report alone.

## Implementation Order (from S2)

```
Batch 1 (parallel-safe):
  P0-1: journal.go + program.md.tmpl
  P1-2: adapter.go
  P1-3: master.md.tmpl
  P3-1: cmd/goalx/main.go
  P3-2: cli/init.go

Batch 2 (sequential, shared files):
  P2-1→P2-3: config.go dead code + report.md.tmpl
  P1-1: session-path helpers (8 files)
  P0-2: report.go rename
  P3-3: start.go heartbeat warning
```

## Minimal Viable Fix (7 lines, 5 minutes)
P0-1 (journal mismatch, 3 lines) + P1-3 (C-c, 1 line) + P1-2 (adapter printf, 1 line) + P0-2 (rename, 1 line) + P3-2 (nanoseconds, 1 line)
