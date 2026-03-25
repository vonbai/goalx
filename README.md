# GoalX

```bash
goalx auto "audit the authentication system and fix every vulnerability"
# go to sleep
# wake up to results
```

You give a goal. GoalX launches a master agent that decomposes it, spins up parallel workers across AI engines, challenges findings, rescues stuck sessions, and synthesizes everything into a final result. You come back to a research report or a passing test suite — depending on what the goal needed.

## What Actually Happens

```
$ goalx auto "find and fix all N+1 query issues in the API layer"

GoalX ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Run:      n1-query-audit
  Master:   claude-code/opus
  Worktree: goalx/n1-query-audit/root

  The master is now in control.
  It will read the codebase, define acceptance criteria,
  dispatch workers, verify results, and close out the run.

  goalx observe    — watch live progress
  goalx status     — check control plane health
  goalx tell       — redirect the master if needed
  goalx keep       — merge results when satisfied

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

Behind the scenes:

```
                        goalx auto "goal"
                              │
                    ┌─────────┴─────────┐
                    │     Master        │  opus — decomposes goal,
                    │  (orchestrator)   │  dispatches work, verifies results
                    └────┬────┬────┬────┘
                         │    │    │
                   ┌─────┘    │    └─────┐
                   ▼          ▼          ▼
              session-1   session-2   session-3
              codex/5.4   codex/5.4   claude/sonnet
              (research)  (develop)   (adversarial)
                   │          │          │
                   └──────────┴──────────┘
                              │
                    ┌─────────┴─────────┐
                    │   Master merges,  │
                    │   verifies, and   │
                    │   closes the run  │
                    └───────────────────┘
                              │
                        goalx keep
                     (you merge to main)
```

## Install

```bash
git clone https://github.com/vonbai/goalx.git && cd goalx
make install        # /usr/local/bin/goalx
make skill-sync     # installs Claude Code + Codex skills
```

Requires: Go 1.24+, tmux, and at least one of [Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Codex CLI](https://github.com/openai/codex).

Zero config needed — GoalX detects your engines and picks the best preset:

| Engines Installed | Master | Workers |
|-------------------|--------|---------|
| Claude Code + Codex | opus | gpt-5.4 |
| Claude Code only | opus | sonnet |
| Codex only | gpt-5.4 | gpt-5.4 |

## Quick Start

```bash
# The default. Master decides everything.
goalx auto "goal"

# Watch it work
goalx observe

# Nudge the master mid-run
goalx tell "focus on the payment module first"

# Merge results to main when satisfied
goalx keep
```

That's the entire workflow for most goals. Everything below is optional control.

## Phase Runs

When you know what kind of work you want:

```bash
goalx research "map all API endpoints and their auth requirements" --effort high
goalx develop "implement rate limiting on all public endpoints" --effort medium
```

Chain phases from saved results:

```bash
goalx debate --from auth-audit          # challenge research findings
goalx implement --from auth-audit       # build fixes from research
goalx explore --from auth-audit         # dig deeper into one area
```

## Runtime Control

```bash
# Adjust how agents think about the problem
goalx dimension session-2 --set adversarial,evidence

# Deeper reasoning for a specific session
goalx replace session-2 --route-profile research_deep

# Urgent redirect — interrupts the master immediately
goalx tell --urgent "stop: the payments API is down, pivot to incident response"

# Park a session for later, resume when needed
goalx park session-3
goalx resume session-3
```

## Effort Levels

```bash
goalx research "goal" --effort high     # deep reasoning
goalx develop "goal" --effort minimal   # fast cheap pass
```

| Level | When to Use |
|-------|-------------|
| `minimal` | Triage, validation, lightweight checks |
| `low` | Fast iteration |
| `medium` | Default balanced execution |
| `high` | Hard problems, deep analysis |
| `max` | The hardest, highest-value slices only |

## Goal Dimensions

Dimensions shape how agents approach the objective:

```bash
goalx auto "audit security" --dimension adversarial,evidence
goalx dimension session-2 --add creative    # change live
```

| Dimension | Focus |
|-----------|-------|
| `depth` | Go deep on the most impactful area |
| `breadth` | Scan everything, find blind spots |
| `creative` | Non-obvious solutions, challenge assumptions |
| `adversarial` | Find bugs, flaws, edge cases |
| `evidence` | Quantify everything with data |
| `comparative` | Benchmark against industry best practices |
| `feasibility` | Implementation cost, risk, dependencies |
| `user` | End-user perspective, usability |

## Config

Override defaults only when you need to. User-level `~/.goalx/config.yaml`, project-level `.goalx/config.yaml`:

```yaml
master:
  engine: claude-code
  model: opus
roles:
  research: { engine: codex, model: gpt-5.4, effort: high }
  develop:  { engine: codex, model: gpt-5.4, effort: medium }

# Route sessions to different engines based on role + dimensions + effort
routing:
  profiles:
    research_deep:  { engine: claude-code, model: opus, effort: high }
    build_fast:     { engine: codex, model: gpt-5.4-mini, effort: minimal }
  rules:
    - role: research
      any_dimensions: [depth]
      efforts: [high, max]
      profile: research_deep
    - role: develop
      efforts: [minimal, low]
      profile: build_fast

# Semantic guidance for the master (not routing aliases)
preferences:
  research:
    guidance: "Default gpt-5.4 high. Use opus for deep analysis."
  develop:
    guidance: "Default gpt-5.4 medium. Use fast for simple fixes."
```

Project-level config can also define a local validation command that sessions run before handoff:

```yaml
local_validation:
  command: "go build ./... && go test ./... && go vet ./..."
```

## All Commands

| Command | What It Does |
|---------|-------------|
| **Start** | |
| `goalx auto` | One master-led run. The default path. |
| `goalx research` | Direct research run with role defaults |
| `goalx develop` | Direct develop run with role defaults |
| `goalx debate --from RUN` | Challenge findings from a saved run |
| `goalx implement --from RUN` | Build from a saved run's research |
| `goalx explore --from RUN` | Dig deeper from a saved run |
| **Observe** | |
| `goalx observe` | Live transport capture + control summary |
| `goalx status` | Progress, lease health, inbox, reminders |
| `goalx list` | All runs across states |
| `goalx context` | Run-scoped paths, roster, roles |
| `goalx afford` | Available commands and paths |
| **Control** | |
| `goalx tell` | Durable instruction to master/session |
| `goalx add` | Add a session to a running run |
| `goalx dimension` | Change runtime dimension assignments |
| `goalx replace` | Hand a slice to a new routed owner |
| `goalx focus` | Pin the default run for bare commands |
| **Session Lifecycle** | |
| `goalx park` | Pause a session, keep its worktree |
| `goalx resume` | Restart a parked session |
| `goalx keep` | Merge worktree (session→run or run→main) |
| `goalx archive` | Tag and preserve a session branch |
| **Closeout** | |
| `goalx verify` | Run acceptance command, record result |
| `goalx save` | Export artifacts to durable storage |
| `goalx result` | Show saved results (`--full` for raw report) |
| `goalx diff` | Diff session code or report outputs |
| `goalx review` | Compare all session outputs |
| `goalx report` | Generate markdown report from journal |
| **Cleanup** | |
| `goalx stop` | Kill processes, preserve worktree |
| `goalx drop` | Kill processes, remove everything |
| **Advanced** | |
| `goalx init` | Generate manual draft config |
| `goalx start --config PATH` | Launch from explicit config |
| `goalx serve` | HTTP API server |
| `goalx attach` | Attach to tmux session |
| `goalx next` | Suggest next pipeline step |

## Architecture

GoalX is **protocol scaffolding infrastructure**. The Go code provides storage (file I/O), execution (process management), and connectivity (git/worktree, inbox/outbox, lease renewal). All interpretation, orchestration, and proof construction belongs to agents.

```
goalx/
├── config.go             # Config loading (one resolver, one path)
├── dimensions.go         # Built-in goal dimensions
├── templates/
│   ├── master.md.tmpl    # Master protocol — the orchestration brain
│   └── program.md.tmpl   # Subagent protocol — hypothesis-driven work
├── cli/                  # All commands
└── cmd/goalx/main.go     # Entry point
```

**Master** reads the immutable `run-charter.json`, maintains `goal.json` as the mutable completion boundary, dispatches parallel work, rescues stuck sessions, and builds `proof/completion.json`.

**Subagent** resumes from durable identity + charter, executes hypothesis-driven research or structured TDD, and communicates via journal + inbox.

**Sidecar** renews leases, delivers reminders, and handles urgent interrupt escalation (Escape → relaunch).

## Run Isolation

Every run gets its own git worktree. Sessions can optionally get sub-worktrees for parallel work. Nobody touches main — `goalx keep` is an explicit user decision.

```
~/.goalx/runs/{project}/{run}/     # all runtime state
├── run-charter.json               # immutable doctrine
├── state/                         # mutable run + session state
├── control/                       # inbox, dimensions, guidance
├── proof/                         # completion evidence
└── reports/                       # research outputs
```

## HTTP API

```bash
goalx serve    # starts on configured bind address
```

Full REST API for remote management. Bearer token auth + IP binding. See [deploy/](deploy/) for systemd unit and config.

Key endpoints: `POST /projects/:name/goalx/auto`, `/observe`, `/status`, `/tell`, `/keep`, `/stop`. Same action names as the CLI.

## OpenClaw Integration

Manage GoalX from Lark, Telegram, or Web UI via [OpenClaw](https://github.com/openclaw/openclaw):

```bash
cp -r skill/openclaw-skill /path/to/openclaw/workspace/skills/goalx
```

## License

MIT
