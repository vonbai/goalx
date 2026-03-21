---
name: goalx
description: Use when the user wants to run autonomous research or develop sessions, monitor GoalX progress, add subagents, or manage the research -> debate -> implement pipeline.
allowed-tools: Bash, Read, Glob, Grep, Write, Edit
user-invocable: true
---

# GoalX

GoalX is an orchestration CLI for unattended research and develop runs. Prefer the simplest path that matches the user's intent.
## Operating Rules
1. Run `goalx next` before acting on an existing workspace.
2. Keep Git hygiene invisible. Explain the effect of a dirty tree, then handle it before `start` or `keep`.
3. Route direction changes through the master. Do not type ad-hoc instructions into subagent panes unless the user explicitly asks.
4. Interpret `goalx observe` output. Report what matters instead of dumping raw tmux noise.
## Quick Start
### Default path: autonomous research
```bash
goalx auto "objective"
```
- `goalx auto` defaults to research mode when no mode flag is provided.
- Add `--context /abs/path/a,/abs/path/b` when local files or repos matter.
- Add `--parallel` or `--strategy` only when the user clearly wants control.
### Explicit control
```bash
goalx init "objective" --research
goalx start
```
- Use `init` + `start` when the user wants to inspect or edit `.goalx/goalx.yaml` first.
- Use `--develop` when code changes are the primary goal.
### Develop mode checklist
Before `goalx start` or `goalx auto ... --develop`, make sure `.goalx/goalx.yaml` scopes writable files and defines a real gate:
```yaml
target:
  files: [cli/, config.go]
harness:
  command: "go build ./... && go test ./... -count=1 && go vet ./..."
```
## Scenario Guide
- Research, investigate, audit: `goalx auto "objective"` or `goalx init "objective" --research`
- Fix, implement, refactor: `goalx init "objective" --develop`, review config, then `goalx start`
- Full unattended loop: `goalx auto "objective" --develop` or `goalx auto "objective"`
- Compare against another repo, doc set, or local artifact: add `--context /abs/path1,/abs/path2`
- Add another angle mid-run: `goalx add "direction" --run NAME`
- Need a different engine for the new session: `goalx add --engine codex --model fast "direction" --run NAME`
- Check progress: `goalx observe [NAME]`, `goalx status [NAME]`, `goalx attach [NAME] [window]`
## Flags and Config
`init`, `start "objective"`, and `auto` accept `--research`, `--develop`, `--parallel N`, `--name NAME`, `--context path1,path2`, `--strategy a,b`, `--preset claude|claude-h|codex|mixed`, `--master engine/model`, `--auditor engine/model`, and repeatable `--sub engine/model[:count]`.
`add` accepts `--run NAME`, `--engine ENGINE`, and `--model MODEL`.
Built-in presets: `claude`, `claude-h`, `codex`, `mixed`.
Built-in strategy names: `depth`, `breadth`, `creative`, `feasibility`, `adversarial`, `evidence`, `comparative`, `user`.

Common config fields:
```yaml
objective: "clear goal"
description: "optional extra context"
mode: research
preset: claude
parallel: 2
diversity_hints:
  - "custom session hint"
context:
  files: [/abs/path]
  refs: ["https://example.com/spec"]
target:
  files: [cli/]
  readonly: [docs/]
harness:
  command: "go test ./..."
budget:
  max_duration: 2h
  max_rounds: 6
master:
  engine: claude-code
  model: opus
  check_interval: 2m
sessions:
  - hint: "depth pass"
```
`preset` is usually enough. Reach for top-level `engine`, `model`, `master`, `auditor`, or `sub` only when the user asks for explicit composition.
## Command Reference
- `goalx init "obj" [flags]`, `goalx start`, `goalx start "obj" [flags]`, `goalx auto "obj" [flags]`: create and launch runs
- `goalx observe [NAME]`, `goalx status [NAME] [session-N]`, `goalx attach [NAME] [window]`: inspect live progress
- `goalx add "direction" [--run NAME]`, `goalx review [NAME]`, `goalx diff [NAME] <a> [b]`: expand or compare session work
- `goalx save [NAME]`, `goalx debate`, `goalx implement`, `goalx next`: move the pipeline forward
- `goalx keep [NAME] <session>`, `goalx stop [NAME]`, `goalx drop [NAME]`: merge, stop, or clean up a run
- `goalx list`, `goalx archive`, `goalx report`, `goalx serve`: supporting management commands
## Observe and React
- Healthy run: summarize progress and wait.
- Stuck or shallow run: check `goalx status`, then redirect the master with a concise instruction or add a new angle.
- Wrong direction: steer the master; avoid direct subagent interruption unless necessary.
- `phase":"complete"` in status: run `goalx next`, then usually `save`, `debate`, `implement`, or `keep`.
- Multiple active runs: use `goalx observe NAME` or `goalx attach NAME master`.
## Useful Defaults
- Preset `claude`: master `claude-code/opus`, research `claude-code/sonnet`, develop `codex/codex`
- Preset `claude-h`: all `claude-code/opus`
- Preset `codex`: all `codex/codex`
- Preset `mixed`: master `codex/codex`, research `claude-code/opus`, develop `codex/codex`
- Default strategies by parallel: `1 -> depth`, `2 -> depth, adversarial`, `3 -> depth, adversarial, evidence`, `4+ -> depth, adversarial, evidence, comparative`
