---
name: goalx
description: "Use when a human wants durable autonomous progress in the current project: investigation, implementation, recovery, runtime control, saved-run continuation, or continuous improvement."
---

# GoalX

GoalX is the default operating system for durable autonomous work in this project. Use it when the user wants a real run with durable state, not a one-shot answer.

## Use GoalX By Default

Prefer GoalX when the user wants:

- autonomous investigation
- autonomous implementation
- a repo audit with follow-through
- durable observation and redirection
- a stopped or stranded run recovered in place
- a saved run continued into the next phase
- continuous improvement over time

Do not default to manual tmux orchestration. Use GoalX commands and durable surfaces.

## Write Better Goals

Write the desired end state, not a checklist.

Good:

```bash
goalx run "users can complete the core workflow reliably in production"
goalx run "the codebase has a high-quality architecture audit and a prioritized fix plan"
goalx run "this project keeps getting better until the budget runs out" --intent evolve --budget 8h
```

Bad:

```bash
goalx run "1. inspect auth 2. patch middleware 3. update tests"
```

When the user is vague, keep it outcome-focused:

- "make deploy work" -> `goalx run "the project deploys cleanly to the target host in one step"`
- "audit this repo" -> `goalx run "full audit of this project with an actionable improvement plan"`

## Default Operator Loop

Use this by default:

```bash
goalx run "goal"
goalx run --objective "goal"
goalx run --objective-file /abs/path/to/objective.txt
goalx status
goalx observe
goalx context
goalx afford
goalx tell "redirect"
goalx verify
goalx result
goalx save
```

Quick meanings:

- `status`: durable control summary
- `observe`: live transport plus run facts
- `context`: canonical identity, paths, cognition, and assurance facts
- `afford`: current run-scoped command surface
- `tell`: durable redirect to master or a worker
- `verify`: record assurance evidence, not completion
- `save`: export a saved run for continuation

Use `goalx schema <surface>` before `goalx durable write <surface> ...` when authoring machine-consumed state.

Operator guidance:

- use `--objective-file` for long or multi-line objectives
- fresh runs can briefly show `launching` while bootstrap settles
- in that startup window, prefer `status`, `observe`, or `goalx wait --run RUN master --timeout 30s`
- do not default to `recover` unless the run is actually stopped or stranded

## Canonical Surfaces

GoalX now centers these durable surfaces:

- `objective-contract`
- `obligation-model`
- `assurance-plan`
- `evidence-log`
- `cognition-state`
- `impact-state`
- `freshness-state`

The old `goal` and `acceptance` runtime path is gone. Saved-run continuation now fails fast if canonical surfaces are missing.

## Intent Routing

```bash
goalx run "goal"
goalx run "goal" --intent explore
goalx run "goal" --intent explore --readonly
goalx run "goal" --intent evolve --budget 8h
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
```

Intent meanings:

- `deliver`: default shipped-result path
- `explore`: evidence-first investigation
- `evolve`: continuous improvement
- `debate`: challenge and refine prior findings
- `implement`: build from prior evidence

`--readonly` declares a no-edit GoalX boundary in `target.readonly`.

## Recovery vs Saved Continuation

Same run:

```bash
goalx recover --run RUN
goalx budget --run RUN --extend 2h
```

New phase:

```bash
goalx save --run RUN
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
```

Rules:

- `recover` relaunches the same run in place
- `recover` is for stopped or stranded runs, not a fresh run still showing `launching`
- `save + run --from` creates a new phase
- exhausted-budget recovery requires changing budget first

## Selection Policy

Normal engine/model policy lives in `~/.goalx/config.yaml` under `selection`.

```yaml
selection:
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

Guidance:

- keep `selection` user-scoped
- do not invent project-scoped engine policy
- use explicit `--engine/--model` only for one-off overrides

## Optional Repo Cognition

GoalX always has builtin `repo-native` cognition.  
GitNexus is optional.

Current GitNexus behavior:

- binary install is preferred
- install with `npm install -g gitnexus@1.5.0`
- verify with `gitnexus status`
- pinned `npx gitnexus@1.5.0` is only exposed when a real probe succeeds
- GoalX does not auto-install it
- GoalX records provider facts per worktree scope
- `available` does not mean `indexed` or `fresh`
- GoalX can best-effort refresh missing or stale GitNexus indexes during lifecycle transitions
- both master and worker scopes can receive runnable GitNexus cognition commands through `goalx afford`

If the current runtime already supports MCP, GitNexus MCP can be configured separately and then preferred for graph reads when freshness is trusted.

## Worktree And Merge Boundaries

```bash
goalx add --run NAME --worktree "task"
goalx keep --run NAME session-1
goalx integrate --run NAME --method partial_adopt --from run-root,session-2
goalx keep --run NAME
```

Meaning:

- `keep session-N`: merge a reviewed session branch into the run root
- `integrate`: record run-root integration lineage after master already merged
- `keep` without `session-N`: merge run root into source root

## Advanced Control

Use [references/advanced-control.md](references/advanced-control.md) when the user explicitly wants the full command matrix or operator-level control.
