---
name: goalx
description: Use when a human developer wants GoalX to autonomously investigate, build, verify, keep iterating on a goal in the current project, or when they want to observe, redirect, recover, save, or continue a GoalX run.
---

# GoalX

GoalX is the default operating system for durable autonomous investigation, implementation, and continuous evolution in this project. Use it when the user wants more than a one-shot answer: a real run, real artifacts, durable control, and the ability to continue the work later.

## Use GoalX By Default

If the user wants autonomous progress in this repo, prefer the GoalX skill / `goalx run` over ad-hoc manual orchestration.

Use GoalX when the user wants any of these:

- autonomous investigation
- autonomous implementation
- a project audit with follow-through
- continuous improvement over time
- durable observation and redirection
- a stopped or stranded run recovered in place
- a saved run that can continue into the next phase

Do not default to manual config or raw tmux operations. For machine-consumed durable surfaces, inspect the authoring contract with `goalx schema <surface>` before writing with `goalx durable write`.

When operating through a host assistant:

- in Claude Code, use the installed GoalX skill from `~/.claude/skills/goalx`
- in Codex, use the installed GoalX skill from `~/.codex/skills/goalx`

## Write Better Goals

The goal should describe the end state, not the route.

Good:

```bash
goalx run "users can complete the core workflow reliably in production"
goalx run "the codebase has a high-quality architecture audit and a prioritized fix plan"
goalx run "the product keeps getting better until the budget runs out" --intent evolve --budget 8h
```

Bad:

```bash
goalx run "1. inspect auth 2. patch middleware 3. update tests"
```

When the user is vague, keep the goal outcome-focused:

- "make deploy work" -> `goalx run "the project deploys cleanly to the target host in one step"`
- "audit this repo" -> `goalx run "full audit of this project with an actionable improvement plan"`
- "keep improving this" -> `goalx run "this project keeps getting better until the budget runs out" --intent evolve --budget 8h`

## Default Operator Loop

The normal loop is:

```bash
goalx run "goal"
goalx status
goalx observe
goalx schema status
goalx tell "redirect"
goalx recover --run NAME
goalx verify
goalx result
goalx save
```

Use this by default unless the user explicitly asks for config-first control.

- `goalx status` = durable control/state view
- `goalx observe` = live transport view plus control summary
- `goalx schema <surface>` = authoring contract view for machine-consumed durable surfaces
- `goalx tell` = durable redirect to master or session
- `goalx recover --run NAME` = relaunch the same stopped or stranded run in place
- `goalx verify` = record acceptance facts, not completion verdict
- `goalx result` = read the current result surfaces
- `goalx save` = export a durable saved run for later continuation

## Selection Policy

GoalX now defaults to auto-detected candidate pools rather than preset-first launch.

- use `~/.goalx/config.yaml` `selection.*` for long-term engine/model preferences
- do not suggest project-scoped `selection`; project config is for shared repo facts
- use explicit `--engine/--model` only when the user clearly wants a one-off override
- if the user wants help configuring engine choices, help them write or edit `~/.goalx/config.yaml` directly instead of improvising a custom config shape

When helping with selection config, stay inside this shape:

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

Recommended guidance when a human asks for engine/model setup:

- ask which engines or targets should be disabled because of quota, stability, or preference
- ask which target should bootstrap master by default
- ask whether the worker pool should bias toward one target or stay balanced across multiple targets
- keep the config user-scoped in `~/.goalx/config.yaml`
- avoid `preset`, `routing`, or project-scoped policy unless the human explicitly asks for legacy compatibility

## Intent Routing

Use intent to express the kind of outcome the user wants.

```bash
goalx run "goal"
goalx run "goal" --intent explore
goalx run "goal" --intent explore --readonly
goalx run "goal" --intent evolve --budget 8h
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
goalx run --from RUN --intent explore --readonly
```

Intent mapping:

- **default / deliver**: the user wants the goal achieved
- **explore**: the user wants a fresh evidence-first investigation or a follow-up evidence expansion
- **evolve**: the user wants open-ended iterative improvement
- **debate**: challenge and refine prior findings from a saved run
- **implement**: build from prior evidence or debate output from a saved run

Boundary flag:

- **--readonly**: declare a no-edit execution boundary in `target.readonly` for report-first or investigation-only runs; GoalX exposes it in protocol/context/affordances instead of pretending it is an OS sandbox

## Evolve

`evolve` is not just "run again." It is GoalX's continuous improvement mode.

Use it when the user wants the system to keep finding and executing the next best improvement until a boundary is reached.

```bash
goalx run "this project keeps getting better until the budget runs out" --intent evolve --budget 8h
```

Important facts:

- the master chooses each next iteration
- the run records an iteration trail in `experiments.jsonl`
- budget is a fact the master sees and manages against
- the framework does not force-stop on meaning; the agents decide when to consolidate, continue, or stop

## Saved Runs And Phase Continuation

Saved runs are first-class inputs to later work. Do not improvise this flow.

```bash
goalx save --run RUN
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
goalx run --from RUN --intent explore --readonly
```

Rules:

- `debate` and `implement` phase continuation require a saved run
- `explore` supports both fresh runs and saved-run continuation
- add `--readonly` when the continuation should carry a declared no-edit boundary
- the saved run must contain report or summary context
- use `goalx save` before telling the user to continue from a prior run

## Recovery vs Phase Continuation

Keep these two paths separate:

```bash
goalx recover --run RUN
goalx save --run RUN
goalx run --from RUN --intent implement
```

- `goalx recover --run RUN` relaunches the same stopped or stranded run in place
- `goalx save --run RUN` plus `goalx run --from RUN --intent ...` creates a new phase from saved artifacts
- do not suggest `save + run --from` when the user wants to continue the same run after `stop`, tmux loss, or a stranded state

## Worktree And Merge Boundaries

GoalX uses explicit worktree boundaries so parallel work stays mergeable.

```bash
goalx add --run NAME --worktree "task"
goalx keep --run NAME session-1
goalx integrate --run NAME --method partial_adopt --from run-root,session-2
goalx keep --run NAME
```

Meaning:

- run root worktree = the integration boundary for the run
- session worktree = an isolated worker boundary
- `goalx keep --run NAME session-N` = merge session branch into the run root
- `goalx integrate --run NAME --method ... --from ...` = record the current run-root result after master manually integrated work there
- `goalx keep --run NAME` = merge run root back into the source root

Do not describe `keep` as a generic "save my work" command. It is a merge boundary command.
Do not describe `integrate` as a merge command. It records lineage for a run-root state that master already produced.

## Effort, Dimensions, And Control

Use these when the user wants to shape the run, not as the default first move.

```bash
goalx run "goal" --effort high --dimension adversarial,evidence
goalx dimension --run NAME session-2 --set depth,creative
goalx replace --run NAME session-2 --engine claude-code --model opus --effort high
```

Prefer the simplest viable control surface first:

1. `goalx tell`
2. `goalx dimension`
3. `goalx replace`
4. `goalx add`

## Results And Truth

GoalX keeps the public result in `summary.md` and supporting material in `reports/`.

```bash
goalx result
goalx result --full
goalx save
```

Keep these truths straight:

- GoalX records facts; agents make judgments
- `goalx verify` records exit code and output; it does not auto-declare success
- transport delivery is not the same thing as task completion
- local-first control and durable state are the public release contract
- active docs explain the workflow, but `goalx schema` is the canonical durable authoring-contract authority

## What Not To Do

- Do not turn the user's goal into a checklist unless they explicitly need planning rather than execution.
- Do not default to `goalx init` / `goalx start --config` unless the user wants config-first control.
- Do not hand-edit GoalX machine-consumed runtime files.
- Do not recommend raw tmux interaction as the primary control path.
- Do not suggest `goalx save` plus `goalx run --from ...` as same-run recovery; use `goalx recover --run NAME`.
- Do not suggest `goalx run --from ...` unless there is already a saved run with usable context.
- Do not blur worktree merge boundaries: session keep and run keep are different operations.

## Advanced Control

For config-first launch, manual session control, explicit engine overrides, or transport-level intervention: [references/advanced-control.md](references/advanced-control.md).
