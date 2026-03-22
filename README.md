# GoalX

Autonomous research and development framework. Master/Subagent architecture powered by AI coding agents (Claude Code, Codex). GoalX starts the master, and the master orchestrates the rest through GoalX tools.

**GoalX provides tools. The master orchestrates.**

## How It Works

```
goalx auto "investigate authentication system vulnerabilities"
```

GoalX creates a run directory and launches a master agent in tmux. The master decides when to call `goalx add`, keeps required goal work covered, spins up temporary research sessions when needed, challenges findings, rescues failed sessions, and synthesizes results.

```
┌─────────────────────────────────────────────────┐
│  goalx auto "objective"                         │
│                                                 │
│  master-led run                                 │
│                                                 │
│  tmux session:                                  │
│    master: starts first and reads goalx config   │
│    master: calls goalx add to launch workers     │
│    session-1+: created on demand by the master   │
└─────────────────────────────────────────────────┘
```

## Install

```bash
go install github.com/vonbai/goalx/cmd/goalx@latest
```

Or build from source:

```bash
git clone https://github.com/vonbai/goalx.git
cd goalx
go build -o bin/goalx ./cmd/goalx
```

### Requirements

- Go 1.21+
- tmux
- One of: [Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Codex CLI](https://github.com/openai/codex)

## Quick Start

```bash
# Give a goal, master handles everything
goalx auto "audit code quality and find bugs"

# Watch progress
goalx observe

# View results
goalx result
```

Default to `goalx auto`. Only use `goalx init` / `goalx start` when you explicitly want config-first or low-level control.

## Commands

| Command | Description |
|---------|-------------|
| `goalx init` | Advanced/manual path: generate config from objective without starting |
| `goalx start` | Advanced/manual path: launch tmux session from existing config |
| `goalx auto` | Init and start one master-led run, then exit |
| `goalx observe` | Live tmux capture from all agents |
| `goalx status` | Journal-based progress summary |
| `goalx add` | Add a session to a running run (`--mode research` launches a temporary research session) |
| `goalx park` | Park an idle/blocked session for later reuse without deleting its worktree |
| `goalx resume` | Resume a parked session in its existing worktree |
| `goalx save` | Save durable artifacts and `artifacts.json` to `.goalx/runs/` |
| `goalx verify` | Run the active run's acceptance command and record the result |
| `goalx debate` | Generate debate config from prior research |
| `goalx implement` | Generate develop config from consensus |
| `goalx keep` | Merge session branch into main |
| `goalx next` | Suggest next pipeline step |
| `goalx result` | Show saved run results (`--full` prints the full research summary) |
| `goalx review` | Compare all session outputs |
| `goalx attach` | Attach to tmux session or window |
| `goalx serve` | Start HTTP API server |
| `goalx stop` | Graceful shutdown |
| `goalx drop` | Cleanup worktrees and branches; refuses runs with unsaved artifacts |

## Single-Run Flow

```
goalx auto → master-led run
           → observe / status while it runs
           → redirect only when needed
           → save / result when the run finishes
```

`goalx debate` and `goalx implement` still exist as explicit commands, but `goalx auto` no longer routes between phases on the framework side.

## Advanced Control

Use `goalx init` + `goalx start`, direct config edits, or manual session control only when you explicitly want low-level control. The default path for both research and development is still `goalx auto`.

## Runtime vs Saved State

- Active runtime state lives under `~/.goalx/runs/{projectID}/{run}`.
- Durable project artifacts live under `<project>/.goalx/runs/{run}` after `goalx save`.
- `artifacts.json` is the durable index for saved reports and other research outputs consumed by `result`, `debate`, and `implement`.
- GoalX bootstraps `.goalx/` into `.git/info/exclude` for local repos so project-scoped run state stays out of git by default.

## Goal Dimensions

Dimensions define how agents approach the objective — not what they do, but from what angle:

```bash
goalx init "objective" --research --parallel 3 --strategy depth,adversarial,evidence
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

Defaults: parallel=2 → depth+adversarial, parallel=3 → +evidence. Custom dimensions in `~/.goalx/config.yaml`.

## Agent Composition

```bash
# Explicit agent composition with --sub
goalx init "objective" --research --sub claude-code/opus:2 --sub codex/gpt-5.4:1

# Override master engine
goalx init "objective" --master codex/gpt-5.4 --parallel 2

# N workers + 1 auditor pattern
goalx init "objective" --preset claude-h --parallel 3 --auditor codex/gpt-5.4
```

## Architecture

```
goalx/
├── config.go           # Config hierarchy (4 layers)
├── strategies.go       # Built-in research strategies
├── journal.go          # JSONL journal format
├── templates/
│   ├── master.md.tmpl  # Master agent protocol
│   └── program.md.tmpl # Subagent protocol
├── cli/                # All CLI commands
│   ├── auto.go         # Init + start, then exit
│   ├── start.go        # Session launch + worktree setup
│   ├── observe.go      # Live tmux capture
│   └── ...
└── cmd/goalx/main.go   # Entry point
```

### Protocol Design

GoalX is a **protocol scaffolding tool**. The Go code launches the master, exposes worker-management tools, and handles git/worktree mechanics; the orchestration logic lives in the protocol templates:

**Master** (`master.md.tmpl`): Final responsible party and lightweight dispatcher. Maintains a machine-readable `goal-contract.json`, records required-item completion provenance (`preexisting|run_change|mixed`), keeps required items covered or explicitly blocked, dispatches parallel work when independent required slices remain, parks idle sessions for later reuse, resumes parked sessions before creating unnecessary new ones, keeps the acceptance checklist aligned as proof against that contract, rescues dead or stuck sessions, runs verification before `done` / `implement`, and cannot close a run without both passing acceptance and internally consistent completion provenance.

**Subagent** (`program.md.tmpl`): Hypothesis-driven exploration (research) or structured TDD (develop). Executes the current assignment, but the goal contract remains the run-level completion boundary. Communicates via journal files and guidance files, including concise blocker and dependency hints so the master can rebalance work quickly or park/resume the session cleanly.

### Config Hierarchy

```
Built-in defaults → ~/.goalx/config.yaml → .goalx/config.yaml → .goalx/goalx.yaml
```

### Engine Presets

| Preset | Master | Research Sub | Develop Sub |
|--------|--------|-------------|-------------|
| hybrid | claude/opus | claude/opus | codex/gpt-5.4 |
| claude | claude/opus | claude/sonnet | codex/gpt-5.4 |
| claude-h | claude/opus | claude/opus | claude/opus |
| codex | codex/gpt-5.4 | codex/gpt-5.4 | codex/gpt-5.4 |
| mixed | codex/gpt-5.4 | claude/opus | codex/gpt-5.4 |

Custom presets in `~/.goalx/config.yaml`. Override per-run with `--master`, `--sub`, `--auditor`.

## HTTP API & Remote Management

GoalX includes a lightweight HTTP server for remote management:

```bash
goalx serve    # starts on configured bind address
```

API endpoints:
- `GET /projects` — list all configured workspaces
- `POST /projects/:name/goalx/start` — start a run
- `POST /projects/:name/goalx/observe` — check agent progress
- `POST /projects/:name/goalx/tell` — send instructions to master
- `POST /projects/:name/goalx/config` — read or modify project/run configuration
- `POST /workspaces` — add project directory (auto git-init if needed)
- `GET /runs` — all active runs across all projects

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
