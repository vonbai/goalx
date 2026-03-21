---
name: goalx
description: Use when the user wants to run autonomous research, start parallel AI agents for code analysis, launch develop sessions, check agent progress, or manage the research→debate→develop lifecycle. Also use when the user mentions goalx, research runs, subagents, master/session agents, or wants to investigate/analyze/audit code autonomously. Even if they just say "research this" or "investigate that" or "look into X in the background", this skill likely applies.
allowed-tools: Bash, Read, Glob, Grep, Write, Edit
user-invocable: true
---

# GoalX — Intelligent Research Orchestrator

GoalX launches parallel AI agents to research, debate, and implement — supervised by a master agent that drives the objective to completion. Your job as the skill user is to translate what the user wants into the right goalx configuration and commands.

## Core Rules

1. **Git is invisible.** Handle dirty worktree silently before start/keep.
2. **State-aware.** Run `goalx next` before every action.
3. **Respect the chain: User → Master → Subagent.** Redirect instructions via `/goalx tell`, not directly to sessions.
4. **Configure intelligently.** Don't just pass through user words — pick the right mode, strategy, parallel count, and context based on what they actually need.
5. **Analyze, don't dump.** Interpret observe output; tell the user what's happening, not raw tmux captures.

## Scenario Guide — What to Do When

### "Research this" / "Investigate X" / "Look into Y"
```bash
goalx init "objective" --research --parallel 2
```
- Pure investigation, no code changes
- Pick dimensions based on objective:
  - Code audit → `depth,adversarial`
  - Market/tech research → `comparative,creative`
  - Architecture review → `depth,evidence`
  - Explore alternatives → `creative,feasibility`

### "Fix this" / "Implement X" / "Refactor Y"
```bash
goalx init "objective" --develop --parallel 2
```
- Code changes expected. Set `target.files` and `harness.command` in `.goalx/goalx.yaml`

### "Compare this project with X" / "Reference another codebase"
```bash
goalx init "objective" --research --context /path/to/other-project
```
- `--context` accepts directories (auto-discovers key files) and file paths
- After init, edit `.goalx/goalx.yaml` to add URLs or notes:
  ```yaml
  context:
    files:
      - /path/to/other-project/
    refs:
      - https://github.com/xxx/yyy
      - "This project uses event-sourcing, compare with CQRS approach"
  ```

### "Just research, don't write code"
- Use `--research` mode. Master will `recommendation: done` when findings are complete
- No implement phase needed

### "Run it fully automatic"
```bash
goalx auto "objective" --research --parallel 2
```
- Runs research → debate (if needed) → implement → keep automatically
- Master decides each transition

### "I want to redirect the research / change direction"
```bash
/goalx tell "focus on security aspects instead of performance"
```
- Sends to master, who redistributes to subagents
- For urgent direct control: `/goalx tell session-1 "stop current direction, investigate X"`

### "Add another research angle mid-run"
```bash
goalx add "investigate from security perspective" --run <NAME>
# With specific engine/model:
goalx add --engine codex --model gpt-5.4 "audit the implementation" --run <NAME>
```

## Configuration Guide

After `goalx init`, the config is at `.goalx/goalx.yaml`. You should edit it when the defaults don't match the user's needs.

### Key fields to configure:

```yaml
# Objective — what master evaluates against
objective: "clear, specific objective"
description: "optional supplementary context for subagents"

# Mode
mode: research    # or develop

# Engine + Model
preset: claude    # claude (default), claude-h (all opus), codex (all gpt-5.4), mixed (codex master + claude sub)
engine: claude-code  # or codex
model: sonnet     # or opus

# Strategies (research mode)
# Auto-assigned if omitted. Override when you know the right approach:
diversity_hints:
  - "Depth-first: pick most impactful area, go deep"
  - "Adversarial: find bugs, flaws, challenge assumptions"

# Context — any reference material (files, URLs, notes)
context:
  files:
    - /path/to/local/file.md
    - /path/to/other-project/
  refs:
    - https://github.com/xxx/relevant-project
    - "Background: the system processes 10k events/sec"

# Target (develop mode)
target:
  files: [cli/, config.go]     # what subagents can modify
  readonly: [docs/, go.mod]    # read but don't modify

# Gate (develop mode)
harness:
  command: "go build ./... && go test ./... -count=1"

# Parallel sessions
parallel: 2       # 1-4 subagents

# Heartbeat
master:
  check_interval: 2m0s   # how often master checks progress
```

### When to edit config vs use CLI flags:
- **CLI flags**: quick settings (`--parallel`, `--strategy`, `--master`, `--auditor`, `--sub`)
- **Edit yaml**: multi-line objective, context refs/URLs, custom diversity hints, harness, target files, per-session hints

### Session Orchestration

Use `--sub engine/model:N` to explicitly compose agents:

```bash
# 2 opus exploring + 1 codex auditing
goalx init "obj" --research --sub claude-code/opus:2 --auditor codex/gpt-5.4

# 3 codex dev + 1 opus reviewing
goalx init "obj" --develop --master codex/gpt-5.4 --sub codex/gpt-5.4:3 --auditor claude-code/opus

# mixed team
goalx init "obj" --research --sub claude-code/opus:1 --sub codex/gpt-5.4:1
```

`--sub` overrides `--parallel`. Count defaults to 1 if omitted. Combine with `--auditor` for N workers + 1 reviewer.

For per-session hints, edit `sessions:` in `.goalx/goalx.yaml` after init.

Quick patterns with presets:
- "all opus" → `--preset claude-h --parallel 3`
- "all codex" → `--preset codex --parallel 2`
- "codex master + claude sub" → `--preset mixed --parallel 2`

Presets: `claude` (default), `claude-h` (all opus), `codex` (all gpt-5.4), `mixed` (codex master + claude sub)

## Commands Reference

| Command | Use when |
|---------|----------|
| `goalx init "obj" [flags]` | Start new research/develop |
| `goalx start` | Launch after config is ready |
| `goalx auto "obj" [flags]` | Full auto pipeline |
| `goalx observe [NAME]` | Check what agents are doing |
| `goalx status [NAME]` | Journal-based progress |
| `goalx tell "msg"` | Redirect master (default) or session |
| `goalx add "direction"` | Add subagent mid-run |
| `goalx save [NAME]` | Save artifacts to .goalx/runs/ |
| `goalx debate` | Generate debate config from prior research |
| `goalx implement` | Generate develop config from consensus |
| `goalx keep [NAME] [session]` | Merge session branch to main |
| `goalx review [NAME]` | Compare session outputs |
| `goalx next` | What should I do next? |
| `goalx stop [NAME]` | Graceful shutdown |
| `goalx drop [NAME]` | Cleanup worktrees + branches |
| `goalx list` | List all runs |
| `goalx diff [NAME] <a> [b]` | Compare sessions |
| `goalx attach [window]` | Print tmux attach commands |

## Observe — How to React

After `goalx observe`, analyze and decide:

| What you see | What to do |
|-------------|------------|
| All agents working normally | Report progress, wait |
| Agent stuck/idle for 2+ heartbeats | `/goalx tell "session-N seems stuck, check on it"` |
| Wrong direction | `/goalx tell "redirect: focus on X instead of Y"` |
| Need more angles | `goalx add "new research direction"` |
| Research quality too shallow | `/goalx tell "push deeper on finding X, need evidence"` |
| phase=complete | `goalx save` → suggest next step |

## Pipeline

```
init → start → [observe...] → save
                                 ↓
debate → start → [observe...] → save    (if disagreements)
                                 ↓
implement → start → [observe...] → keep  (if code changes needed)
```

`goalx auto` runs this entire loop. Master decides transitions.
Pure research ends at `save` with `recommendation: done`.
