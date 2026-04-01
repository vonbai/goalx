# GoalX

## One Goal In. Continuous Software Evolution Out.

GoalX is a durable local framework for autonomous investigation, implementation, verification, and continuous software improvement.

```bash
goalx run "this product is a working investor-ready demo someone can open and immediately understand"
# go to sleep
# wake up to progress, reports, and mergeable work
```

GoalX is not a one-shot prompt wrapper. It gives one master agent a durable run identity, isolated worktrees, runtime control, saved artifacts, and the ability to continue or recover later without relying on chat history luck.

Master decomposes the objective into typed obligations and dispatches parallel sessions:

![goalx run](guides/goalx-auto.png)

Runs accumulate across projects:

![goalx list](guides/goalx-list.png)

## Why GoalX

- Autonomous execution across investigation and implementation: one master reads the repo, chooses the path, dispatches durable sessions, reviews results, and keeps moving.
- Durable local control: runs, journals, reports, leases, saved artifacts, and runtime state survive restarts.
- Worktree-isolated parallelism: workers can branch into dedicated worktrees and merge back through explicit `keep` boundaries.
- Evidence-first closeout: GoalX records assurance evidence, but the master still owns judgment.
- Continuous evolution: `--intent evolve` lets the run keep finding and executing the next best improvement until the budget boundary says stop.

## Start Here

GoalX has two normal entry paths.

### 1. Skill-First Operator Path

If Claude or Codex is your operator:

```bash
make install
make skill-sync
```

`make skill-sync` installs the same GoalX skill to:

- `~/.claude/skills/goalx`
- `~/.codex/skills/goalx`

Use it like this:

- Claude Code: ask Claude to use GoalX for this repo.
- Codex: tell the assistant to use `$goalx`.

This is the preferred operator path when you want the assistant to launch, observe, redirect, recover, verify, save, and continue runs correctly instead of improvising raw tmux sequences.

### 2. Direct CLI Path

If you are operating GoalX yourself:

```bash
goalx run "the dashboard feels production-ready, fast, and clear on desktop and mobile"
goalx run "the onboarding feels polished and credible for first-time users"
goalx run --objective-file /abs/path/to/objective.txt
```

`goalx run` is the canonical public entrypoint. Fresh runs always materialize intake and compile the success plane before the master starts working.
Use `--objective-file` for long or multi-line objectives so shell quoting does not silently corrupt the launch command.

## Canonical Model

GoalX now centers these durable surfaces:

- `objective-contract`: immutable extracted user-clause contract
- `obligation-model`: canonical mutable boundary of what must be true
- `assurance-plan`: scenario-based verification strategy
- `evidence-log`: recorded assurance evidence
- `cognition-state`: repo cognition provider facts
- `impact-state`: changed files and impact facts
- `freshness-state`: whether evidence or cognition is stale

Legacy `goal` and `acceptance` runtime surfaces are gone from the normal runtime path. Phase continuation now fails fast if a saved run is missing canonical surfaces.

## Default Operator Loop

The normal loop is:

```bash
goalx run "goal"
goalx status
goalx observe
goalx context
goalx afford
goalx tell "redirect"
goalx verify
goalx result
goalx save
```

Meaning:

- `goalx status`: durable control summary
- `goalx observe`: transport plus current run facts
- `goalx context`: canonical identity, paths, cognition, and assurance facts
- `goalx afford`: current run-scoped command surface
- `goalx schema <surface>`: inspect the canonical authoring contract before writing machine-consumed durable state
- `goalx durable write <surface> ...`: write canonical durable state after inspecting the contract
- `goalx tell`: durable redirect to master or a worker
- `goalx verify`: record assurance evidence, not a completion verdict
- `goalx result`: read the current result surfaces
- `goalx save`: export a saved run for later continuation

Fresh runs can briefly show `launching` while bootstrap settles. In that window, prefer:

```bash
goalx status
goalx observe
goalx wait --run RUN master --timeout 30s
```

Do not jump to `goalx recover` just because `master` or `runtime-host` has not published stable lease facts yet.

## Write Better Goals

Write the end state, not the route.

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

When the user is vague, keep the goal outcome-focused:

- "make deploy work" -> `goalx run "the project deploys cleanly to the target host in one step"`
- "audit this repo" -> `goalx run "full audit of this project with an actionable improvement plan"`
- "keep improving this" -> `goalx run "this project keeps getting better until the budget runs out" --intent evolve --budget 8h`

## Intent Routing

Intent biases master behavior without creating separate runtime engines.

```bash
goalx run "goal"
goalx run --objective "goal"
goalx run --objective-file /abs/path/to/objective.txt
goalx run "goal" --intent explore
goalx run "goal" --intent explore --readonly
goalx run "goal" --intent evolve --budget 8h
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
goalx run --from RUN --intent explore --readonly
```

Intent meanings:

- `deliver`: the default shipped-result path
- `explore`: fresh evidence-first investigation or saved-run follow-up exploration
- `evolve`: open-ended iterative improvement
- `debate`: challenge and refine prior findings from a saved run
- `implement`: build from prior evidence or debate output

Boundary flag:

- `--readonly`: declare a no-edit run contract in `target.readonly`

Context injection:

```bash
goalx run "audit auth flow" --intent explore --context README.md,docs/architecture,https://example.com/spec,ref:ticket-123,note:reproduce the rejection path
```

- file and directory paths go to `context.files`
- URLs and `ref:` / `note:` items go to `context.refs`
- use one comma-delimited `--context` value

## Recover vs Phase Continuation

Do not mix these up.

Same run, same run directory:

```bash
goalx recover --run RUN
```

New phase from saved artifacts:

```bash
goalx save --run RUN
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
```

Budget control for the same run:

```bash
goalx budget --run RUN
goalx budget --run RUN --extend 2h
goalx budget --run RUN --set-total 10h
goalx budget --run RUN --clear
```

Rules:

- `recover` relaunches the same stopped or stranded run in place
- a fresh run that still shows `launching` is not a recover case; wait for bootstrap to settle first
- `save + run --from` creates a new phase from saved artifacts
- exhausted-budget recovery requires a budget change first
- saved-run continuation now requires canonical saved surfaces and fails fast when they are missing

## Engine Selection

GoalX auto-detects available engines and builds a candidate pool.

User-scoped long-term policy lives in `~/.goalx/config.yaml`:

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

- `selection` is user-scoped only
- project config and manual drafts do not persist `selection`
- explicit `--engine/--model` is an override, not the normal path

## Optional Repo Cognition

GoalX always has builtin `repo-native` cognition:

- file inventory
- file search
- file read
- git diff

GoalX can also expose `GitNexus` as an optional graph cognition provider.

Current behavior:

- binary install is preferred
- install: `npm install -g gitnexus@1.5.0`
- verify: `gitnexus status`
- pinned `npx gitnexus@1.5.0` is only marked available if a real probe succeeds
- GoalX does not auto-install GitNexus
- GoalX records provider facts per worktree scope, not just one global availability boolean
- `available` does not mean `indexed` or `fresh`
- GoalX can best-effort refresh a missing or stale GitNexus index during run lifecycle transitions
- both master and worker scopes can receive runnable GitNexus cognition affordances when the provider is available

Optional MCP setup for runtimes that support it:

```bash
codex mcp add gitnexus -- npx -y gitnexus@1.5.0 mcp
claude mcp add gitnexus -- npx -y gitnexus@1.5.0 mcp
```

GoalX documents these commands, but does not run them for you or mutate your user-level MCP configuration.

Typical outcomes in `goalx context --json`:

- no GitNexus: only `repo-native`
- working pinned `npx`: `gitnexus available=true invocation_kind="npx" index_state=missing|fresh|stale`
- working global install: `gitnexus available=true invocation_kind="binary" index_state=missing|fresh|stale`

## Worktree Architecture

GoalX uses explicit merge boundaries:

```text
source root
   |
   \u2514\u2500 run root worktree
         |
         \u251c\u2500 session-1 worktree
         \u251c\u2500 session-2 worktree
         \u2514\u2500 shared sessions (optional)
```

Key commands:

```bash
goalx add --run NAME --worktree "task"
goalx keep --run NAME session-1
goalx integrate --run NAME --method partial_adopt --from run-root,session-2
goalx keep --run NAME
```

Meaning:

- `goalx keep --run NAME session-N`: merge a reviewed session branch into the run root
- `goalx integrate --run NAME --method ... --from ...`: record manual run-root integration lineage
- `goalx keep --run NAME`: merge run root into the source root

## Public Command Surface

Normal operator commands:

- `run`, `status`, `observe`, `context`, `afford`, `attach`, `wait`
- `tell`, `add`, `replace`, `dimension`, `budget`, `focus`
- `review`, `diff`, `keep`, `integrate`
- `park`, `resume`, `stop`, `recover`, `archive`, `save`, `drop`
- `verify`, `result`, `report`, `schema`, `durable`

See:

- [guides/cli-reference.md](guides/cli-reference.md)
- [skill/references/advanced-control.md](skill/references/advanced-control.md)

## Build And Install

```bash
make install
make skill-sync
make check
```

Equivalent verification:

```bash
go build ./...
go test ./... -count=1
go vet ./...
```
