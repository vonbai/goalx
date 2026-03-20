---
name: goalx
description: Use when the user wants to run autonomous research, start parallel AI agents for code analysis, launch develop sessions, check agent progress, or manage the research→debate→develop lifecycle. Also use when the user mentions goalx, research runs, subagents, master/session agents, or wants to investigate/analyze/audit code autonomously. Even if they just say "research this" or "investigate that" or "look into X in the background", this skill likely applies.
allowed-tools: Bash, Read, Glob, Grep, Write, Edit
user-invocable: true
---

# GoalX — Intelligent Research Orchestrator

Orchestration layer for GoalX CLI. Understands pipeline state, automates git/config, handles routine operations, and drives the research→debate→develop→keep lifecycle.

## Core Rules

1. **Git is invisible.** Dirty worktree? Auto-commit before start/keep. Never ask the user to manage git.
2. **State-aware.** Detect pipeline state before every action. Run `goalx next` first.
3. **Suggest next.** After every action, tell the user what comes next.
4. **Act, don't narrate.** Run commands, show results. Minimize commentary.
5. **Analyze, don't dump.** For observe, read the output and tell the user what each agent is doing and whether anything is stuck — don't just paste raw tmux captures.
6. **Respect the chain: User → Master → Subagent.** When the user wants to redirect research or give instructions, send to master (via `/goalx tell`). Master distributes to subagents. Only bypass master for urgent direct intervention.

## State Detection

```bash
goalx next
```

| State | Detection | Default action |
|-------|-----------|---------------|
| RUNNING | active tmux session | observe + analyze |
| COMPLETE | run dir exists, no tmux | suggest save → next step |
| HAS_RESEARCH | `.goalx/runs/` has research | suggest debate or implement |
| IDLE | nothing | quickstart |

## Commands

### `/goalx` (no args) — Smart Dashboard

1. `goalx next` to detect state
2. **RUNNING** → run `goalx observe`, then analyze and summarize what each agent is doing
3. **COMPLETE** → `goalx review`, suggest save/debate/implement
4. **IDLE** → quickstart hint

### `/goalx auto "objective" [flags]`

Full autonomous pipeline. The skill orchestrates all stages:

1. `goalx init "objective" [flags]` + `goalx start`
2. Poll `.goalx/status.json` every 30s until phase=complete (or use `/loop 30s /goalx observe`)
3. `goalx save` + read summary → **AI decides**: needs debate?
   - Yes (disagreements found) → `goalx debate` + `goalx start` → poll → save
   - No → skip to implement
4. `goalx implement` + `goalx start` → poll → review
5. **AI picks** best session → `goalx keep` → rebuild binary if Go project

At each stage, explain the decision ("Skipping debate — findings are consistent across sessions").

### `/goalx init "objective" [flags]`

```bash
goalx init <args...>
```

Flags: `--research`/`--develop`, `--parallel N`, `--strategy <names>`, `--context <paths>`, `--name <name>`

Strategies: `depth`, `breadth`, `adversarial`, `experimental`, `comparative`, `web`, `security`, `performance`

Defaults (research, no --strategy): parallel=1→depth, 2→depth+adversarial, 3→+experimental, 4→+comparative

Config written to `.goalx/goalx.yaml`. Show contents after.

### `/goalx start [args...]`

**Before starting, handle git silently:**
```bash
git status --porcelain | grep -v '^??' | grep -v '.goalx/'
```
If dirty: ask user, then `git add -A && git commit -m "wip: snapshot before goalx run"`.

Then: `goalx start <args...>`

After success: `goalx observe`.

### `/goalx observe [NAME]`

```bash
goalx observe <NAME>
```

After CLI output, **analyze and summarize**:
- What is each agent currently doing?
- How many rounds completed?
- Any sessions stuck or idle?
- Estimated progress toward completion

### `/goalx status [NAME]`

```bash
goalx status <NAME>
```

### `/goalx stop [NAME]`

```bash
goalx stop <NAME>
```

### `/goalx review [NAME]`

```bash
goalx review <NAME>
```
After output, suggest next step: save → debate/implement.

### `/goalx save [NAME]`

```bash
goalx save <NAME>
```
Copies artifacts to `.goalx/runs/<name>/`.

### `/goalx debate`

```bash
goalx debate
```
CLI generates config from `.goalx/runs/`. Show config, on approval: start.

### `/goalx implement`

```bash
goalx implement
```
CLI generates develop config from consensus. Show config, on approval: start.

### `/goalx next`

```bash
goalx next
```

### `/goalx keep [NAME] [session]`

**Handle git first:**
```bash
git status --porcelain | grep -v '^??'
```
If dirty: `git add -A && git commit -m "chore: pre-merge cleanup"`.

Then: `goalx keep <NAME> <session>`

After keep, if the project has a build step: run the project's harness command to verify the merge didn't break anything.

### `/goalx archive [NAME] <session>`

```bash
goalx archive <NAME> <session>
```

### `/goalx add "direction" [--run NAME]`

```bash
goalx add "research direction" --run <NAME>
```
Adds new subagent mid-run. Master notified automatically.

### `/goalx tell "message"` or `/goalx tell <target> "message"`

Send a message to the running session. **Default target is master** — master decides how to distribute to subagents.

```bash
# To master (preferred — master distributes):
tmux send-keys -t <TMUX_SESSION>:master "user message here" Enter

# To a specific session (bypass master — use sparingly):
echo "guidance content" > <run-dir>/guidance/<session>.md
tmux send-keys -t <TMUX_SESSION>:<window> Enter
```

**Always prefer telling master.** Master is the director — it knows the context and will distribute appropriately. Only bypass master for urgent direct intervention.

### `/goalx diff [NAME] <a> [b]`

```bash
goalx diff <NAME> session-1 session-2
```

### `/goalx report [NAME]`

```bash
goalx report <NAME>
```

### `/goalx drop [NAME]`

```bash
goalx drop <NAME>
```

### `/goalx attach [window]`

Print both commands:
```
tmux attach -t <SESSION>:<window>      # outside tmux
tmux switch-client -t <SESSION>:<window>  # inside tmux
```

### `/goalx list`

```bash
goalx list
```

## Full Pipeline

```
goalx init → start → [observe...] → save
                                       ↓
goalx debate → start → [observe...] → save    (optional)
                                       ↓
goalx implement → start → [observe...] → keep
```

Short-circuit: skip debate, go research → implement directly.
Single step: `goalx init "obj" --develop` → start → keep.
Full auto: `/goalx auto "objective" --research --parallel 2`
