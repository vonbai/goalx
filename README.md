# GoalX

Autonomous research and development framework. Master/Subagent architecture powered by AI coding agents (Claude Code, Codex). GoalX starts the master, and the master orchestrates the rest through GoalX tools.

**GoalX provides tools. The master orchestrates.**

## How It Works

```
goalx auto "investigate authentication system vulnerabilities"
```

GoalX creates a run directory, launches the master transport session, and starts a run-scoped sidecar. The master decides when to call `goalx add`, keeps required goal work covered, spins up temporary research sessions when needed, challenges findings, rescues failed sessions, and synthesizes results. The sidecar renews leases, records reminders/deliveries, and keeps the durable control plane current.

```
┌─────────────────────────────────────────────────┐
│  goalx auto "objective"                         │
│                                                 │
│  master-led run                                 │
│                                                 │
│  transport session + run-sidecar:                │
│    master: starts first and reads goalx config   │
│    sidecar: renews leases and reminder delivery  │
│    session-1+: created on demand by the master   │
└─────────────────────────────────────────────────┘
```

## Install

```bash
git clone https://github.com/vonbai/goalx.git
cd goalx
go build -o /usr/local/bin/goalx ./cmd/goalx
```

### Requirements

- Go 1.24+
- tmux
- One of: [Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Codex CLI](https://github.com/openai/codex)

## Quick Start

```bash
# Give a goal, master decides approach (research/develop/hybrid)
goalx auto "audit code quality and find bugs"

# Or specify mode explicitly
goalx research "audit code quality and find bugs"
goalx develop "implement the fix"

# Continue from a saved run (inherits source run's code changes)
goalx debate --from research-audit
goalx implement --from research-audit-debate
goalx explore --from research-audit

# Watch progress
goalx observe

# Review results in run worktree, then merge to main when satisfied
goalx keep

# View saved results
goalx result
```

Default to `goalx auto`. Use `goalx research` / `goalx develop` when you want an explicit phase-specific run with role defaults. Use `goalx debate --from RUN`, `goalx implement --from RUN`, and `goalx explore --from RUN` to continue from saved runs. Only use `goalx init` / `goalx start --config PATH` when you explicitly want config-first or low-level control. GoalX now snapshots the caller environment once at run creation and reuses that same run-scoped environment for later `add` and `resume`, so actor launches do not depend on whatever env the tmux server or later shell happens to have. Use `goalx focus --run NAME` when a project has multiple active runs and you want to pin the default run. Bare `--run NAME` resolution is local-first within the current project; for cross-project targeting, use `--run <project-id>/<run>`. `--parallel` is optional: if you omit it, GoalX keeps project/preset defaults; if nothing else sets it, the initial fan-out is `1`. That value is initial planning guidance, not a permanent ceiling on later master dispatch.

## Commands

| Command | Description |
|---------|-------------|
| `goalx list` | List all runs (active, completed, archived) |
| `goalx init` | Advanced/manual path: generate a manual draft config from an objective |
| `goalx start` | Advanced/manual path: launch from explicit `--config PATH` |
| `goalx auto` | Init and start one master-led run, then exit |
| `goalx research` | Start a research run directly from CLI flags |
| `goalx develop` | Start a develop run directly from CLI flags |
| `goalx observe` | Live transport capture when available plus control-plane summary |
| `goalx status` | Progress summary plus run ID, epoch, charter health, lease health, unread inbox, reminders, and delivery failures |
| `goalx focus` | Set the default run used by commands that omit `--run` |
| `goalx add` | Add a session to a running run (`--worktree` for isolated parallel work, `--mode research` for research session) |
| `goalx tell` | Send a durable instruction to the master or a specific session (`--urgent` for priority delivery) |
| `goalx wait` | Inbox-aware blocking wait; replaces sleep for master monitoring loops |
| `goalx park` | Park an idle/blocked session for later reuse without deleting its worktree |
| `goalx resume` | Resume a parked session in its existing worktree |
| `goalx diff` | Diff session code or report outputs |
| `goalx review` | Compare all session outputs |
| `goalx keep` | Merge session worktree into run worktree, or merge run worktree into source root |
| `goalx archive` | Tag and preserve a session branch |
| `goalx save` | Save durable artifacts, `run-charter.json`, session identities, goal-boundary state, provenance, and `artifacts.json` to user-scoped durable storage |
| `goalx verify` | Run the acceptance command and record the exit code + output (master interprets the result) |
| `goalx debate` | Start a debate phase from `--from RUN` |
| `goalx implement` | Start a develop phase from `--from RUN` |
| `goalx explore` | Start a follow-up research phase from `--from RUN` |
| `goalx report` | Generate a markdown report from the run journal |
| `goalx result` | Show saved run results (`--full` prints the full research summary) |
| `goalx attach` | Attach to tmux session or window |
| `goalx serve` | Start HTTP API server |
| `goalx stop` | Graceful shutdown (kills leased processes, preserves run worktree) |
| `goalx drop` | Cleanup worktrees, branches, and leased processes; refuses runs with unsaved artifacts |
| `goalx next` | Suggest next pipeline step |

## Single-Run Flow

```
goalx auto → master-led run
           → goalx research / goalx develop when you want a direct phase-specific run
           → observe / status while it runs
           → redirect only when needed
           → verify before done / implement closeout
           → save / result when the run finishes
           → goalx debate --from RUN / goalx implement --from RUN / goalx explore --from RUN for follow-up phases
```

`goalx auto` remains the default path. `goalx research` / `goalx develop` are direct phase entry points with role defaults, and `goalx debate --from RUN` / `goalx implement --from RUN` / `goalx explore --from RUN` continue from saved runs. Phase commands default to direct run creation and start; `--write-config` stays advanced/manual only. Use `--master`, `--research-role`, and `--develop-role` when you need explicit role defaults for a run or phase.

## Run Worktree Model

Every run gets its own isolated git worktree (`goalx/<run>/root` branch). Master and sessions work in this worktree, not on the main branch. Dirty tracked files are auto-committed before worktree creation (`--no-snapshot` to skip). Gitignored files (CLAUDE.md, docs/, .claude/) are copied into the worktree for a complete project mirror.

Session worktrees are optional: `goalx add "task"` runs in the shared run worktree; `goalx add --worktree "task"` creates an isolated session worktree for parallel work. `--from RUN` inherits the source run's worktree code, not just context.

Merging to the main branch is a user decision: `goalx keep` from outside the run. Master merges sessions into the run worktree but does not merge to main.

## Advanced Control

Use `goalx init` + `goalx start --config .goalx/goalx.yaml`, direct config edits, or manual session control only when you explicitly want low-level control. The default path for both research and development is still `goalx auto`, but explicit `goalx research` / `goalx develop` / `goalx debate --from` / `goalx implement --from` / `goalx explore --from` are the preferred non-config-first alternatives. Use `--write-config` only when you explicitly want to generate `.goalx/goalx.yaml` first.

## Runtime vs Saved State

- Project scope is for shared config only: `.goalx/config.yaml`.
- Active runtime state lives under `~/.goalx/runs/{projectID}/{run}`.
- Saved runs live under `~/.goalx/runs/{projectID}/saved/{run}` after `goalx save`.
- Active protocol uses immutable `run-spec.yaml` plus `run-charter.json`, with mutable `state/run.json`, `state/sessions.json`, `control/*`, and `proof/completion.json`.
- The durable control plane is centered on run-scoped files such as `control/run-identity.json`, `control/identity-fence.json`, `control/run-state.json`, `control/inbox/master.jsonl`, `control/reminders.json`, and `control/deliveries.json`.
- `goalx save` exports immutable provenance too: `run-charter.json` plus `sessions/session-N/identity.json` for every durable worker.
- User-scoped `registry.json` and `status.json` under `~/.goalx/runs/{projectID}/` are convenience indexes and external progress summaries, not the source of truth.
- `artifacts.json` is the durable index for saved reports and other research outputs consumed by `result`, `debate`, `implement`, and `explore`.
- `run-metadata.json` tracks phase lineage, including `phase_kind`, `source_run`, and `parent_run`, so each run can inherit from its own saved input without touching shared project config.
- GoalX only auto-ignores `.goalx/goalx.yaml`, the advanced/manual scratch config. Shared `.goalx/config.yaml` stays visible to git.
- When a project has multiple active runs, pass `--run NAME` explicitly for mutating commands. Use `goalx focus --run NAME` to pin the default run for commands that omit `--run`.
- Bare `--run NAME` resolution is local-first within the current project. For cross-project targeting, use `--run <project-id>/<run>`.

## Goal Dimensions

Dimensions define how agents approach the objective — not what they do, but from what angle:

```bash
goalx research "objective" --parallel 3 --strategy depth,adversarial,evidence
```

| Dimension | Focus |
|-----------|-------|
| `depth` | Pick the most impactful area, go as deep as possible |
| `breadth` | Scan all dimensions, find blind spots |
| `creative` | Non-obvious solutions, challenge assumptions |
| `feasibility` | Implementation cost, risk, dependencies |
| `adversarial` | Find bugs, flaws, edge cases |
| `evidence` | Quantify everything with data |
| `comparative` | Compare with industry best practices |
| `user` | End-user perspective, usability |

Strategy defaults for research fan-out: parallel=2 → depth+adversarial, parallel=3 → +evidence. Custom dimensions in `~/.goalx/config.yaml`.

## Agent Composition

```bash
# Explicit agent composition with --sub
goalx research "objective" --sub claude-code/opus:2 --sub codex/gpt-5.4:1

# Override role defaults
goalx research "objective" --master codex/gpt-5.4 --research-role claude-code/opus --develop-role codex/gpt-5.4-mini

# N workers + 1 auditor pattern
goalx develop "objective" --preset claude-h --parallel 3 --auditor codex/gpt-5.4
```

## Architecture

```
goalx/
├── config.go           # Shared config loading + explicit manual draft overlay
├── strategies.go       # Built-in research strategies
├── journal.go          # JSONL journal format
├── templates/
│   ├── master.md.tmpl  # Master agent protocol
│   └── program.md.tmpl # Subagent protocol
├── cli/                # All CLI commands
│   ├── auto.go         # Init + start, then exit
│   ├── start.go        # Session launch + worktree setup
│   ├── observe.go      # Live transport capture + control summary
│   └── ...
└── cmd/goalx/main.go   # Entry point
```

### Protocol Design

GoalX is a **protocol scaffolding tool**. The Go code launches the master, exposes worker-management tools, and handles git/worktree mechanics; the orchestration logic lives in the protocol templates. The live protocol is built around immutable charter/identity files plus mutable goal/gate/proof state:

**Master** (`master.md.tmpl`): Final responsible party and lightweight dispatcher. Re-reads `run-charter.json` and `control/run-identity.json` as the durable identity anchor, maintains `goal.json` as the mutable completion boundary, appends boundary/path decisions to `goal-log.jsonl`, keeps required outcomes covered or explicitly blocked, dispatches parallel work when independent required slices remain, parks idle sessions for later reuse, resumes parked sessions before creating unnecessary new ones, keeps `acceptance.json` aligned with the current verification gate, rescues dead or stuck sessions, runs verification before `done` / `implement`, and cannot close a run without both passing acceptance and a canonical `proof/completion.json`.

**Subagent** (`program.md.tmpl`): Hypothesis-driven exploration (research) or structured TDD (develop). Resumes from `sessions/session-N/identity.json` plus `run-charter.json`, then executes the current assignment. `goal.json` remains the mutable completion boundary, `acceptance.json` remains the verification gate, and `proof/completion.json` remains the canonical closeout proof. Communicates via journal files plus the durable session inbox/cursor pair, including concise blocker and dependency hints so the master can rebalance work quickly or park/resume the session cleanly.

### Config Model

```
Built-in defaults → ~/.goalx/config.yaml → .goalx/config.yaml
                                      \
                                       + explicit manual draft: .goalx/goalx.yaml
```

`Built-in defaults` and `~/.goalx/config.yaml` provide global defaults. `.goalx/config.yaml` is the only shared project-scoped GoalX file. `.goalx/goalx.yaml` is an explicit advanced/manual draft, loaded only when you choose `goalx start --config PATH` or `--write-config`.

### Engine Presets

| Preset | Master | Research Role | Develop Role |
|--------|--------|-------------|-------------|
| hybrid | claude/opus | claude/opus | codex/gpt-5.4 |
| claude | claude/opus | claude/sonnet | codex/gpt-5.4 |
| claude-h | claude/opus | claude/opus | claude/opus |
| codex | codex/gpt-5.4 | codex/gpt-5.4 | codex/gpt-5.4 |
| mixed | codex/gpt-5.4 | claude/opus | codex/gpt-5.4 |

Custom presets in `~/.goalx/config.yaml`. Override per-run with `--preset` plus role-specific flags.
Role-specific overrides use `--master`, `--research-role`, `--develop-role`, `--parallel`, `--context`, `--strategy`, `--budget-seconds`, `--name`, and `--sub`. `--parallel` is optional initial fan-out. Master may still add or resume more durable workers later when the goal warrants it.

## HTTP API & Remote Management

GoalX includes a lightweight HTTP server for remote management:

```bash
goalx serve    # starts on configured bind address
```

Common endpoints:
- `GET /projects` — list all configured workspaces
- `GET /runs` — list runtime runs across all configured workspaces
- `POST /workspaces` — add project directory (auto git-init if needed)
- `POST /projects/:name/goalx/auto` — start one master-led run from an objective
- `POST /projects/:name/goalx/start` — start from CLI flags or from an explicit manual draft via `config_scope=draft`
- `POST /projects/:name/goalx/research` — start a research run directly from CLI flags
- `POST /projects/:name/goalx/develop` — start a develop run directly from CLI flags
- `POST /projects/:name/goalx/observe` — capture live progress
- `POST /projects/:name/goalx/status` — summarize progress
- `POST /projects/:name/goalx/add` — add a session to a running run
- `POST /projects/:name/goalx/debate` — start a debate phase from `from`
- `POST /projects/:name/goalx/implement` — start an implementation phase from `from`
- `POST /projects/:name/goalx/explore` — start a follow-up research phase from `from`
- `POST /projects/:name/goalx/keep|park|resume|save|stop|drop` — control a specific run or session
- `POST /projects/:name/goalx/tell` — send instructions to the master
- `POST /projects/:name/goalx/config` — read/write shared project config, read/write an explicit manual draft via `config_scope=draft`, or read an active run's immutable run spec

The HTTP server accepts the same action names as the local CLI for the supported routes above; see [cli/serve.go](cli/serve.go) if you need the exact request mapping.

Bearer token auth + IP binding. See [deploy/](deploy/) for config example and systemd unit.

## OpenClaw Integration

GoalX can be managed by an [OpenClaw](https://github.com/openclaw/openclaw) agent via Lark, Telegram, or Web UI:

```bash
cp -r skill/openclaw-skill /path/to/openclaw/workspace/skills/goalx
```

The OpenClaw agent calls GoalX HTTP API to start research, check progress, and manage tasks across all projects. See [deploy/README.md](deploy/README.md) for setup guide.

## Claude Code Skill

For local interactive use in Claude Code:

```bash
mkdir -p ~/.claude/skills/goalx
cp skill/SKILL.md ~/.claude/skills/goalx/SKILL.md
```

Then: `/goalx observe`, `/goalx auto "objective"`, etc.

## License

MIT
