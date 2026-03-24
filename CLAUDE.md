# GoalX Framework — Development Guidelines

## Product Vision

```bash
goalx auto "目标"
# 去睡觉
# 醒来看结果
```

用户只给 goal，其他全部 master 自主决定：要不要 subagent、几个、什么引擎、什么 effort / routing、什么时候切阶段、运行时 dimension 怎么调、怎么验收。研究看报告，开发看代码，全自动看最终结果。每次架构决策问"这让用户离'给 goal 看结果'更近了吗？"

## Core Design Principle: Facts, Not Judgments

GoalX is a **protocol scaffolding infrastructure**. The framework provides three things only:

1. **Storage** — file I/O for durable state (goal.json, acceptance.json, proof/, control/)
2. **Execution** — process management (run shell commands, manage tmux sessions)
3. **Connectivity** — transport plumbing (git/worktree, inbox/outbox, lease renewal, nudge delivery)

**All interpretation, judgment, orchestration, and proof construction belongs to agents (master or subagent).** The framework never:

- Derives completion mode, goal satisfaction, or result classification from raw data
- Labels evidence provenance (e.g. "preexisting" vs "run_change")
- Blocks agent exit via hooks encoding policy (inbox sync hooks for transport reliability are acceptable)
- Auto-maps mode to execution parameters (target, harness, role)
- Silently normalizes, truncates, or deduplicates agent-written data
- Encodes governance policy in charter struct fields (completion standard, approval gates, exploration doctrine belong in master template as semantic guidance, not in Go structs)

### Verify = Record, Not Judge

`goalx verify` runs the acceptance command and records the exit code + output. It does not detect completion state, build proof items, derive verdicts, or update caches. Master reads the recorded result and decides what it means.

### Hooks = Transport Reliability, Not Policy Gates

Hooks ensure transport reliability (e.g. "check inbox before sleeping" for message delivery). Hooks must never encode completion criteria, proof validation, or exit conditions that override agent judgment.

### Charter = Structure, Not Strategy

Charter records immutable structural facts: run IDs, paths, objective text, mode, role names. Strategy and governance (completion standard, path comparison rules, scope protection) live in the master template as semantic guidance the agent reasons over.

## Architecture

- **Language**: Go (1.24+)
- **Build + Test**: `go build ./... && go test ./... -count=1 && go vet ./...`
- **Install**: `go build -o /usr/local/bin/goalx ./cmd/goalx` (always use this path, never `go install`)
- **Templates**: `templates/*.md.tmpl` — protocol contracts rendered at launch
- **State**: `~/.goalx/runs/<project>/<run>/` — all durable run state (user-scoped, not in project dir)
- **Engines**: claude-code, codex, aider — each with transport adapters and provider-aware effort mapping
- **Routing**: config-driven `routing.profiles` + `routing.table`; session identity records `requested_effort`, `effective_effort`, and `route_profile`
- **Dimensions**: launch-time `--dimension` seeds hints; runtime `goalx dimension` mutates `control/dimensions.json`
- **Cleanup**: `goalx stop` / `goalx drop` must kill leased process trees before tmux teardown or run-dir removal

## Key Files

| Layer | Files |
|-------|-------|
| Master protocol | `templates/master.md.tmpl` |
| Subagent protocol | `templates/program.md.tmpl` |
| Completion | `cli/completion.go`, `cli/proof.go`, `cli/verify.go` |
| Control plane | `cli/control_state.go`, `cli/sidecar.go`, `cli/adapter.go` |
| Routing + dimensions | `config.go`, `dimensions.go`, `cli/dimension.go`, `cli/launch_options.go`, `cli/phase_options.go` |
| Session lifecycle | `cli/add.go`, `cli/keep.go`, `cli/park.go`, `cli/resume.go` |
| Session identity | `cli/session_identity.go`, `cli/next_config.go` |
| State | `cli/runtime_state.go`, `cli/derived_run.go`, `cli/coordination.go` |
| Charter | `cli/charter.go` |

## Testing

- Run all tests: `go test ./... -count=1`
- Run specific: `go test ./cli -run TestName -v`
- Tests use real filesystem fixtures, no mocks for state files
