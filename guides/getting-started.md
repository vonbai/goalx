# GoalX Getting Started

## 1. Install

```bash
git clone https://github.com/vonbai/goalx.git && cd goalx
make install
make skill-sync
```

Requirements:

- Go 1.24+
- `tmux`
- Claude Code or Codex CLI

## 2. Choose The Entry Path

Recommended for assistant-driven operation:

- sync the skill with `make skill-sync`
- in Claude Code, ask Claude to use GoalX or the GoalX skill
- in Codex, tell the assistant to use `$goalx`

Recommended for direct terminal use:

```bash
goalx run "goal"
```

## 3. Start With A Goal

Describe the destination, not the implementation checklist.

Good:

```bash
goalx run "the product deploys cleanly to the target host and is reachable without manual repair steps"
goalx run "this repository has a high-quality architecture audit and an actionable fix plan"
goalx run "users can complete the core workflow reliably in production"
goalx run "the dashboard feels production-ready on desktop and mobile"
```

Bad:

```bash
goalx run "1. profile the API 2. add caching 3. fix tests"
```

## 4. Watch The Run

```bash
goalx status
goalx observe
```

- `status` is the durable/control view.
- `observe` is the live transport view plus control summary.

## 5. Redirect When Needed

```bash
goalx tell "focus on the payment module first"
goalx tell --urgent "stop: production is down"
```

Use durable GoalX commands. Do not type instructions directly into tmux as the normal control path.

## 6. Recover Or Continue

```bash
goalx recover --run NAME
goalx save --run NAME
goalx run --from NAME --intent implement
```

- `recover` relaunches the same stopped or stranded run in place
- `save` plus `run --from` starts a new phase from durable artifacts

## 7. Get Results

```bash
goalx verify
goalx result
goalx integrate --run NAME --method partial_adopt --from run-root,session-2
goalx keep
goalx save
```

## 8. Choose Intent Only When It Helps

```bash
goalx run "goal" --intent research
goalx run "goal" --intent develop
goalx run "goal" --intent evolve --budget 8h
```

- `research`: findings and reports
- `develop`: code and verification
- `evolve`: open-ended iterative improvement

## 9. Understand The Worktree Boundary

- run root worktree = the integration boundary for the run
- session worktree = an isolated worker boundary
- project config can set `worktree_root: .worktrees` if you want those boundaries to live under the repo instead of `~/.goalx/runs/.../worktrees/`
- `goalx keep --run NAME session-N` merges session work into the run root
- `goalx integrate --run NAME --method ... --from ...` records a manual run-root integration after master merged or cherry-picked work there
- `goalx keep --run NAME` merges the run root back into your source root

## 10. Keep The Mental Model Straight

- GoalX framework = storage, execution, connectivity
- master = judgment, orchestration, closeout
- sessions = execution slices
- `goalx verify` = fact recording, not auto-completion
