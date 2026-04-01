# GoalX

## One Goal In. Continuous Software Evolution Out.

GoalX is an autonomous engineering framework that turns a codebase goal into durable investigation, implementation, verification, and iterative improvement across isolated worktrees.

```bash
goalx run "this product is a working investor-ready demo someone can open and immediately understand"
# go to sleep
# wake up to progress, reports, and mergeable work
```

Master decomposes the objective into typed requirements and dispatches parallel sessions:

![goalx run](guides/goalx-auto.png)

Runs accumulate across projects:

![goalx list](guides/goalx-list.png)

## Why GoalX

- **Autonomous execution across investigation and implementation**: one master agent reads the codebase, chooses the path, dispatches durable sessions, reviews results, and keeps moving.
- **Continuous improvement, not one-shot output**: GoalX can stop at a result, or keep iterating with `--intent evolve` until the budget boundary is reached.
- **Durable execution instead of chat-state luck**: runs, reports, journals, leases, and saved artifacts survive restarts; stopped or stranded runs can recover in place, and saved artifacts can continue into the next phase.
- **Worktree-isolated parallelism**: GoalX can split work into isolated session worktrees, then merge cleanly back through explicit `keep` boundaries.

## Start Here

GoalX has two recommended entry paths.

### 1. Skill-First Operator Path

If you use Codex or Claude as your operator, sync the GoalX skill first:

```bash
make install
make skill-sync
```

`make skill-sync` installs the same GoalX skill to `~/.claude/skills/goalx` and `~/.codex/skills/goalx`.

Use it like this:

- **Claude Code**: ask Claude to use GoalX or the GoalX skill for this repo. Claude discovers it from `~/.claude/skills/goalx`.
- **Codex**: tell the assistant to use `$goalx`. Codex discovers it from `~/.codex/skills/goalx`.

This is the preferred path when you want the assistant to launch, observe, redirect, recover, verify, save, and continue runs correctly instead of improvising raw tmux or ad-hoc command sequences.

### 2. Direct CLI Path

If you operate GoalX yourself from the terminal:

```bash
goalx run "the dashboard feels production-ready, fast, and clear on desktop and mobile"
goalx run "the onboarding feels polished and credible for first-time users"
```

`goalx run` is the canonical public entrypoint.

Fresh `goalx run` always materializes launch intake before the run compiles its success plane.

## Engine Selection

GoalX auto-detects available engines at launch and builds a candidate pool.

Default behavior:

- if `codex` and `claude` are both available:
  - bootstrap master defaults to `codex/gpt-5.4`
  - worker selection comes from the configured worker candidate pool
- if only one supported engine is available, GoalX uses that engine
- if no supported engine is on `PATH`, launch fails loudly

User-level long-term selection policy lives in `~/.goalx/config.yaml`:

```yaml
selection:
  disabled_engines:
    - aider

  disabled_targets:
    - claude-code/sonnet

  master_candidates:
    - codex/gpt-5.4
    - claude-code/opus

  worker_candidates:
    - codex/gpt-5.4
    - claude-code/opus

  master_effort: high
  worker_effort: high
```

Important boundaries:

- `selection` is user-scoped only; project config and manual drafts do not persist it
- explicit `--engine/--model` is an intentional override, not the default path
- master sees the resulting candidate pools as facts and chooses workers from them at runtime

How to shape `selection` in practice:

- put the engine/model you want to bootstrap master with first in `master_candidates`
- put the worker targets you want master to choose from in `worker_candidates`
- use `disabled_engines` when a whole provider is currently off-limits
- use `disabled_targets` when the engine is fine but a specific model is not
- prefer 2-3 candidates per lane, not a long list

Common example:

```yaml
selection:
  master_candidates:
    - codex/gpt-5.4
    - claude-code/opus
  worker_candidates:
    - codex/gpt-5.4
    - claude-code/opus
  worker_effort: high
  disabled_targets:
    - claude-code/sonnet
```

If you use GoalX through Claude or Codex, the skill should help you edit this user config instead of inventing ad-hoc engine policy. Tell it what you want:

- "default master to codex, but keep opus available as a worker option"
- "disable sonnet for now"
- "keep gpt-5.4-mini in the worker pool for cheap implementation slices"

## Project Config

Shared repo facts live in `.goalx/config.yaml`.

Typical example:

```yaml
worktree_root: .worktrees
run_root: .goalx/runs
saved_run_root: .goalx/saved

master:
  check_interval: 2m

preferences:
  worker:
    guidance: "Prefer broad evidence before proposing a fix plan."
  simple:
    guidance: "Bias toward small, mergeable implementation slices."

local_validation:
  command: "go build ./... && go test ./... && go vet ./..."
```

### Project-Local Storage

GoalX can store all run artifacts inside your project:

| Config Key | Purpose | Default |
|------------|---------|---------|
| `worktree_root` | Git worktrees | `~/.goalx/runs/<project>/<run>/worktrees/` |
| `run_root` | Active run state | `~/.goalx/runs/<project>/<run>/` |
| `saved_run_root` | Saved runs | `~/.goalx/runs/<project>/saved/<run>/` |

When configured, relative paths resolve from the project root. Configured values are snapshotted into the run spec at launch, so existing runs keep their original layout.

What `worktree_root` does:

- relocates the run root worktree and dedicated session worktrees under the given directory
- accepts a project-relative path like `.worktrees` or an absolute path
- keeps durable run state in `~/.goalx/runs/...`; only worktree placement changes
- is captured into the run spec at launch, so existing runs keep their original layout
- adds the configured project-local worktree directory to `.git/info/exclude` automatically

With `worktree_root: .worktrees`, a run named `demo` looks like this:

```text
project-root/
  .worktrees/
    demo
    demo-1
    demo-2
```

## Core Workflows

### Default Deliver Workflow

Use this when the desired outcome is a shipped result.

```bash
goalx run "users can complete the core workflow reliably in production"
goalx status
goalx observe
goalx tell "focus on onboarding first"
goalx verify
goalx result
goalx keep
goalx save
```

What happens:

- the master reads the repo and decides whether it needs investigation, implementation, review, or more sessions
- sessions execute concrete slices of work
- the master reviews, verifies, and closes out

### Intent-Guided Workflow

Use `intent` to bias master behavior and output shape without creating separate runtime modes.

```bash
goalx run "we understand why ranking quality regressed and have an evidence-backed recovery plan" --intent explore
goalx run "we understand why ranking quality regressed and have an evidence-backed recovery plan" --intent explore --readonly
goalx run "the product feels investor-ready and the success bar is explicit before implementation starts"
```

- `deliver` stays the default fresh-run path when the user wants the goal achieved.
- `explore` is the fresh evidence-first path when the user wants investigation, alternatives, and reports before implementation.
- fresh `explore` tells the master to start with evidence expansion and path comparison; implementation should follow only when current-run evidence clearly justifies it.
- `--readonly` declares a no-edit execution boundary in `target.readonly` and surfaces it to workers through GoalX protocol/context/affordances. It is a GoalX contract boundary, not an OS sandbox.
- fresh `goalx run` always writes a launch-time intake artifact and feeds it into the success compiler as additional run-context input.

### Extra Context

Use `--context` when the run should start with extra evidence beyond the repo.

```bash
goalx run "audit auth flow" --intent explore --context README.md,docs/architecture,https://example.com/spec,ref:ticket-123,note:reproduce the rejection path
```

- existing files and directories go to `context.files`
- URLs and explicit `ref:` / `note:` items go to `context.refs`
- use one comma-delimited `--context` value; escape literal commas inside one item as `\,`
- phase runs keep saved-run boundary/evidence surfaces and merge any extra `--context` items on top

### Evolve Workflow

Use this when the right behavior is "keep improving until the budget says stop."

```bash
goalx run "this project keeps getting better until the budget runs out" --intent evolve --budget 8h
```

`evolve` is the open-ended improvement mode:

- the master chooses the highest-value next iteration
- GoalX records the iteration trail in `experiments.jsonl`
- the run can continue, pivot, consolidate, or stop when budget or diminishing returns say so

### Recover A Stopped Or Stranded Run

Use this when you want to relaunch the same run in place.

```bash
goalx recover --run auth-audit
```

- `recover` restarts tmux/master/runtime-host for the existing run
- it preserves the same run identity and run directory
- use it after `goalx stop`, tmux loss, or a stranded run with no live master
- if recover is blocked by exhausted budget, change budget first with `goalx budget --run auth-audit --extend 2h` (or `--clear`), then rerun `goalx recover --run auth-audit`

### Runtime Budget Control

Use this when the same run should keep going with a different time boundary.

```bash
goalx budget --run auth-audit
goalx budget --run auth-audit --extend 2h
goalx budget --run auth-audit --set-total 10h
goalx budget --run auth-audit --clear
```

- `goalx budget` is the canonical same-run budget control surface
- budget exhaustion blocks new work creation and recovery-style continuation, but it does not auto-drop the run
- the master should review outputs, keep/adopt if needed, save if continuation matters, then stop explicitly

### Continue A Saved Run Into The Next Phase

Saved runs are first-class inputs to the next phase. This is not the same thing as recovering a stopped run.

```bash
goalx save auth-audit
goalx run --from auth-audit --intent debate
goalx run --from auth-audit --intent implement
goalx run --from auth-audit --intent explore
goalx run --from auth-audit --intent explore --readonly
```

- `save + run --from` creates a new phase run from saved artifacts
- `debate`: challenge and refine prior findings
- `implement`: build from prior evidence or debate output
- `explore`: either start fresh as an evidence-first run or extend the evidence base from saved artifacts
- `--readonly`: carry a declared no-edit boundary into the next phase when it should stay investigative

## Worktree Architecture

This is one of GoalX's core differentiators. Work does not have to pile up in one dirty tree.

```text
source root
   │
   └── run root worktree
         │
         ├── session-1 worktree   (isolated worker)
         ├── session-2 worktree   (isolated worker)
         └── shared sessions      (optional, no dedicated worktree)
```

The merge boundaries are explicit:

- `goalx keep --run NAME session-N` merges a reviewed worker session branch into the run root worktree
- `goalx integrate --run NAME --method partial_adopt --from session-1,session-2` records a master-owned run-root result after manual merge, cherry-pick, or partial adoption
- `goalx keep --run NAME` merges the run root worktree back into your source root

This matters because GoalX is built for parallel investigation and implementation without losing merge discipline.

Default placement keeps worktrees under the run directory in `~/.goalx/runs/<project>/<run>/worktrees/`.
If project config sets `worktree_root`, GoalX places git worktrees at the configured path instead.

## Run Architecture

```text
goalx run "goal"
      │
      ▼
   master
      │
      ├── dispatches durable sessions
      ├── compares paths
      ├── verifies evidence
      └── writes closeout
             │
             ├── summary.md
             ├── reports/
             └── saved run artifacts
```

Runtime state lives under `~/.goalx/runs/<project>/<run>/` by default, or under the configured `run_root` directory if set. Saved runs go to `~/.goalx/runs/<project>/saved/<run>/` by default, or under `saved_run_root` if configured.

GoalX the framework is intentionally narrow:

- **Storage**: durable run state and artifacts
- **Execution**: processes, tmux windows, worktrees, acceptance commands
- **Connectivity**: inbox/outbox, leases, nudges, saved-run continuation

The framework records facts. The agents make judgments.

## Durable Control And Closeout

GoalX is not "start and hope." It has a durable control plane.

```bash
goalx status
goalx observe
goalx schema status
goalx tell "focus on payments first"
goalx tell --urgent "stop: production is down"
goalx recover --run NAME
goalx verify
goalx result
goalx save
```

- `goalx status` shows durable run/control facts
- `goalx observe` shows live transport plus control summary
- `goalx schema <surface>` shows the canonical contract for a machine-consumed durable surface
- `goalx tell` redirects the master or a session durably
- `goalx recover --run NAME` relaunches a stopped or stranded run in place
- `goalx verify` records acceptance facts; it does not declare completion
- `goalx result` reads the canonical final result surface in `summary.md`; if `summary.md` is missing, the run is not closed out yet
- `goalx save` exports the run into durable saved-run storage for later phase continuation

Active operator docs explain the workflow. `goalx schema` is the canonical place to inspect durable protocol shape.

## Truthful Constraints

- **Local-first release**: this public release is the local CLI and skill flow, not a remote HTTP product surface.
- **No silent fallback**: invalid machine-consumed inputs should fail loudly.
- **Engine availability is explicit**: at least one supported engine must be on `PATH`, or launch fails.
- **Verify records, not judges**: GoalX records acceptance exit code and output; the master interprets what that means.
- **Auto-snapshot is real**: launch may commit tracked changes before creating the run unless you pass `--no-snapshot`.

## Install

```bash
git clone https://github.com/vonbai/goalx.git && cd goalx
make install
make skill-sync
```

Requires:

- Go 1.24+
- `tmux`
- at least one of [Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Codex CLI](https://github.com/openai/codex)

## Learn More

Deep-reference material lives under [`guides/`](guides):

- [Getting Started](guides/getting-started.md)
- [Runtime Control](guides/runtime-control.md)
- [Configuration](guides/configuration.md)
- [Memory](guides/memory.md)
- [CLI Reference](guides/cli-reference.md)

Related material:

- [Deployment Notes](deploy/README.md)
- [GoalX skill](skill/SKILL.md)
- [Skill advanced control reference](skill/references/advanced-control.md)

## License
