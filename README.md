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
make install        # builds to /usr/local/bin/goalx
make skill-sync     # copies GoalX skill files to ~/.claude/skills/goalx/ and ~/.codex/skills/goalx/
```

### Requirements

- Go 1.24+
- tmux
- One of: [Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Codex CLI](https://github.com/openai/codex)

### Zero-Config Experience

GoalX auto-detects installed engines and picks the best preset:

| Available Engines | Auto Preset | Master | Sessions |
|-------------------|------------|--------|----------|
| Claude Code + Codex | hybrid | claude-code/opus | codex/gpt-5.4 |
| Claude Code only | claude | claude-code/opus | claude-code/sonnet |
| Codex only | codex | codex/gpt-5.4 | codex/gpt-5.4 |

No config file needed. Just install and run.

### Optional: Custom Config

Override defaults in `~/.goalx/config.yaml` (user-level) or `.goalx/config.yaml` (project-level):

```yaml
# ~/.goalx/config.yaml — only needed to override auto-detected defaults
master:
  engine: claude-code
  model: opus
roles:
  research: { engine: codex, model: gpt-5.4, effort: high }
  develop:  { engine: codex, model: gpt-5.4, effort: medium }
```

Project-level config is for shared project defaults, not just launch-time engine choices. A common use is an optional local validation command:
```yaml
# .goalx/config.yaml — project-specific
harness:
  command: "go build ./... && go test ./... && go vet ./..."
```

## Quick Start

```bash
# Give a goal, master decides approach (research/develop/hybrid)
goalx auto "audit code quality and find bugs"

# Or enter a phase directly with explicit effort
goalx research "audit code quality and find bugs" --effort high
goalx develop "implement the fix" --effort medium

# Continue from a saved run (inherits source run's code changes)
goalx debate --from research-audit
goalx implement --from research-audit-debate
goalx explore --from research-audit

# Adjust runtime dimensions after launch
goalx dimension --run research-audit session-2 --set adversarial,evidence

# Watch progress
goalx observe

# Review results in run worktree, then merge to main when satisfied
goalx keep

# View saved results
goalx result
```

Default to `goalx auto`. Use `goalx research` / `goalx develop` when you want an explicit phase-specific run with role defaults, optional `--effort`, or role-specific engine/model overrides. Use `goalx debate --from RUN`, `goalx implement --from RUN`, and `goalx explore --from RUN` to continue from saved runs. Use `--dimension SPEC` only to seed launch-time viewpoints; the canonical runtime control is `goalx dimension`, and `goalx replace` is the handoff path when the routed owner must change. Only use `goalx init` / `goalx start --config PATH` when you explicitly want config-first or low-level control. GoalX now launches master and sessions from the current process environment at the moment of each start/add/resume/relaunch, so runtime behavior follows the active caller environment instead of a persisted snapshot file. Starting a new run also makes it the focused default run for later commands in the same project. Use `goalx focus --run NAME` when a project has multiple active runs and you want to pin a different default run. Bare `--run NAME` resolution is local-first within the current project; for cross-project targeting, use `--run <project-id>/<run>`. `--parallel` is optional: if you omit it, GoalX keeps project/preset defaults; if nothing else sets it, the initial fan-out is `1`. That value is initial planning guidance, not a permanent ceiling on later master dispatch.

## Commands

| Command | Description |
|---------|-------------|
| `goalx list` | List all runs (active, completed, archived) |
| `goalx init` | Advanced/manual path: generate a manual draft config from an objective |
| `goalx start` | Advanced/manual path: launch from explicit `--config PATH` |
| `goalx auto` | Init and start one master-led run, then exit |
| `goalx research` | Start a research run directly from CLI flags (`--effort`, `--dimension`, role overrides) |
| `goalx develop` | Start a develop run directly from CLI flags (`--effort`, `--dimension`, role overrides) |
| `goalx context` | Show the run-scoped context index: stable paths, roster, roles, and capability facts |
| `goalx afford` | Show the run-scoped GoalX command/path affordances in markdown or JSON |
| `goalx observe` | Live transport capture when available plus control-plane summary |
| `goalx status` | Progress summary plus run ID, epoch, charter health, lease health, unread inbox, reminders, and delivery failures |
| `goalx focus` | Set the default run used by commands that omit `--run` |
| `goalx add` | Add a session to a running run (`--worktree` for isolated parallel work, `--mode research` for research session) |
| `goalx dimension` | Change runtime dimension assignments for one session or all sessions in a run |
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
           → verify when you need fresh acceptance evidence
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
- GoalX also writes read-only guidance surfaces under `control/`: `activity.json`, `context-index.json`, and `affordances.{json,md}`. These are derived run facts and command surfaces for humans and agents, not a second source of truth.
- `goalx save` exports immutable provenance too: `run-charter.json` plus `sessions/session-N/identity.json` for every durable worker.
- User-scoped `registry.json` and `status.json` under `~/.goalx/runs/{projectID}/` are convenience indexes and external progress summaries, not the source of truth.
- `artifacts.json` is the durable index for saved reports and other research outputs consumed by `result`, `debate`, `implement`, and `explore`.
- `run-metadata.json` tracks phase lineage, including `phase_kind`, `source_run`, and `parent_run`, so each run can inherit from its own saved input without touching shared project config.
- GoalX only auto-ignores `.goalx/goalx.yaml`, the advanced/manual scratch config. Shared `.goalx/config.yaml` stays visible to git.
- When a project has multiple active runs, pass `--run NAME` explicitly for mutating commands. Use `goalx focus --run NAME` to pin the default run for commands that omit `--run`.
- Bare `--run NAME` resolution is local-first within the current project. For cross-project targeting, use `--run <project-id>/<run>`.

## Effort Levels

Effort is first-class. GoalX stores the requested effort in session identity, resolves it through the selected engine, and records the provider-specific effective effort.

| Level | Meaning | Claude Code | Codex |
|-------|---------|-------------|-------|
| `auto` | Engine or routing profile chooses the provider-specific default | provider default | provider default |
| `minimal` | Cheapest acceptable pass for lightweight validation or triage | `low` | `low` |
| `low` | Fast iteration with basic reasoning depth | `low` | `low` |
| `medium` | Default balanced execution for most work | `medium` | `medium` |
| `high` | Deeper reasoning for hard implementation or research slices | `high` | `high` |
| `max` | Highest-value, hardest slices only | `max` | `xhigh` |

## Goal Dimensions

Dimensions define how agents approach the objective, and they now live in runtime state. Seed them at launch with `--dimension NAMES` if needed, then change them during the run with `goalx dimension`:

```bash
goalx dimension --run my-run session-2 --set depth,adversarial
goalx dimension --run my-run session-2 --add evidence
goalx dimension --run my-run session-2 --remove depth
goalx dimension --run my-run all --set breadth,comparative
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

Runtime assignments are stored in `control/dimensions.json` inside the run directory. Custom dimensions still come from `~/.goalx/config.yaml` or `.goalx/config.yaml`.

## Agent Composition

```bash
# Explicit agent composition with --sub
goalx research "objective" --sub claude-code/opus:2 --sub codex/gpt-5.4:1 --effort high

# Override role defaults
goalx research "objective" --master codex/gpt-5.4 --research-role claude-code/opus --develop-role codex/gpt-5.4-mini --master-effort medium --research-effort high --develop-effort medium

# N workers + routed review pattern
goalx develop "objective" --preset claude-h --parallel 3 --dimension feasibility --dimension adversarial --route-role develop --effort medium
goalx replace --run demo session-2 --route-profile research_deep
```

## Architecture

```
goalx/
├── config.go           # Shared config loading + explicit manual draft overlay
├── dimensions.go       # Built-in goal dimensions
├── journal.go          # JSONL journal format
├── templates/
│   ├── master.md.tmpl  # Master agent protocol
│   └── program.md.tmpl # Subagent protocol
├── cli/                # All CLI commands
│   ├── auto.go         # Init + start, then exit
│   ├── dimension.go    # Runtime dimension mutation command
│   ├── session_identity.go # Requested/effective effort + route/dimension identity
│   ├── start.go        # Session launch + worktree setup
│   ├── observe.go      # Live transport capture + control summary
│   └── ...
└── cmd/goalx/main.go   # Entry point
```

### Protocol Design

GoalX is a **protocol scaffolding tool**. The Go code launches the master, exposes worker-management tools, and handles git/worktree mechanics; the orchestration logic lives in the protocol templates. The live protocol is built around immutable charter/identity files plus mutable goal/acceptance/proof state and read-only guidance surfaces under `control/`:

**Master** (`master.md.tmpl`): Final responsible party and lightweight dispatcher. Re-reads `run-charter.json` as the durable structural anchor, maintains `goal.json` as the mutable completion boundary, appends boundary/path decisions to `goal-log.jsonl`, keeps required outcomes covered or explicitly blocked, dispatches parallel work when independent required slices remain, parks idle sessions for later reuse, resumes parked sessions before creating unnecessary new ones, rescues dead or stuck sessions, reads `goalx context` / `goalx afford` as the canonical run guidance surfaces, and treats `goalx verify` as evidence recording only. The master interprets acceptance results and builds `proof/completion.json` itself.

**Subagent** (`program.md.tmpl`): Hypothesis-driven exploration (research) or structured TDD (develop). Resumes from `sessions/session-N/identity.json`, `run-charter.json`, and the run-scoped guidance surfaces, then executes the current assignment. `goal.json` remains the mutable completion boundary, `acceptance.json` remains verification-only state, and `proof/completion.json` remains the canonical closeout proof. A session-local validation command may exist, but it is not the run acceptance contract. Communicates via journal files plus the durable session inbox/cursor pair, including concise blocker and dependency hints so the master can rebalance work quickly or park/resume the session cleanly.

### Config Model

```
Built-in defaults → ~/.goalx/config.yaml → .goalx/config.yaml
                                      \
                                       + explicit manual draft: .goalx/goalx.yaml
```

`Built-in defaults` and `~/.goalx/config.yaml` provide global defaults. `.goalx/config.yaml` is the only shared project-scoped GoalX file. `.goalx/goalx.yaml` is an explicit advanced/manual draft, loaded only when you choose `goalx start --config PATH` or `--write-config`.

Example shared config:

```yaml
preset: codex
master:
  engine: claude-code
  model: opus
roles:
  research:
    engine: codex
    model: gpt-5.4
    effort: high
  develop:
    engine: codex
    model: gpt-5.4
    effort: medium
routing:
  profiles:
    research_deep:   { engine: claude-code, model: opus, effort: high }
    research_max:    { engine: claude-code, model: opus, effort: max }
    build_balanced:  { engine: codex, model: gpt-5.4, effort: medium }
    build_fast:      { engine: codex, model: gpt-5.4-mini, effort: minimal }
    fallback_safe:   { engine: claude-code, model: sonnet, effort: low }
  rules:
    - role: simple
      efforts: [minimal, low, medium]
      profile: build_fast
    - role: simple
      efforts: [high, max]
      profile: build_balanced
    - role: develop
      efforts: [medium]
      profile: build_balanced
    - role: develop
      any_dimensions: [audit, adversarial]
      efforts: [high, max]
      profile: research_deep
    - role: research
      efforts: [medium]
      profile: research_deep
    - role: research
      any_dimensions: [depth, comparative]
      efforts: [high, max]
      profile: research_max
preferences:
  research:
    guidance: "默认 gpt-5.4 high。深度分析/架构设计用 opus。简单信息收集用 fast。"
  develop:
    guidance: "主力 gpt-5.4 medium。简单修复用 fast。"
  simple:
    guidance: "轻量任务用 fast。"
```

`routing.profiles` defines reusable engine/model/effort bundles. `routing.rules` is an ordered ruleset over `route_role + dimensions + effort`. `preferences.*.guidance` is human guidance for the agent, not a routing alias.

### Engine Presets

| Preset | Master | Research Role | Develop Role |
|--------|--------|-------------|-------------|
| hybrid | claude/opus | claude/opus | codex/gpt-5.4 |
| claude | claude/opus | claude/sonnet | codex/gpt-5.4 |
| claude-h | claude/opus | claude/opus | claude/opus |
| codex | codex/gpt-5.4 | codex/gpt-5.4 | codex/gpt-5.4 |
| mixed | codex/gpt-5.4 | claude/opus | codex/gpt-5.4 |

Custom presets in `~/.goalx/config.yaml`. Override per-run with `--preset` plus role-specific flags.
Role-specific overrides use `--master`, `--research-role`, `--develop-role`, `--effort`, `--master-effort`, `--research-effort`, `--develop-effort`, `--parallel`, `--context`, `--dimension`, `--name`, and `--sub`. `--parallel` is optional initial fan-out. Master may still add or resume more durable workers later when the goal warrants it.

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
- `POST /projects/:name/goalx/replace` — replace a session with a new routed owner
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
mkdir -p ~/.claude/skills/goalx/{references,agents} ~/.codex/skills/goalx/{references,agents}
cp skill/SKILL.md ~/.claude/skills/goalx/SKILL.md
cp -R skill/references/. ~/.claude/skills/goalx/references/
cp -R skill/agents/. ~/.claude/skills/goalx/agents/
cp skill/SKILL.md ~/.codex/skills/goalx/SKILL.md
cp -R skill/references/. ~/.codex/skills/goalx/references/
cp -R skill/agents/. ~/.codex/skills/goalx/agents/
```

Copy the whole directory so `references/advanced-control.md` stays available. Then ask Claude to use the GoalX skill for tasks such as `goalx observe` or `goalx auto "objective"`.

## License

MIT
