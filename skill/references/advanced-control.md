# Advanced Control

Use this only when the user explicitly wants operator-level GoalX control instead of the default skill loop.

## Default Commands

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

## Config-First Launch

Only when the user explicitly wants to inspect config before launch:

```bash
goalx init "goal"
goalx start --config .goalx/goalx.yaml
```

Do not choose this path by default.

Fresh-run startup guidance:

- a new run can briefly show `launching` while bootstrap settles
- during that window use `goalx status`, `goalx observe`, or `goalx wait --run RUN master --timeout 30s`
- do not treat early missing lease facts as a recover trigger

## Intent And Continuation

```bash
goalx run "goal" --intent explore
goalx run "goal" --intent explore --readonly
goalx run "goal" --intent evolve --budget 8h

goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
goalx run --from RUN --intent explore --readonly
```

Rules:

- `recover` is same-run relaunch
- `save + run --from` is new-phase continuation
- saved-run continuation fails fast when canonical surfaces are missing

## Budget

```bash
goalx budget --run NAME
goalx budget --run NAME --extend 2h
goalx budget --run NAME --set-total 10h
goalx budget --run NAME --clear
```

## Selection Policy

Normal engine/model policy lives in `~/.goalx/config.yaml`:

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

Use explicit `--engine/--model` only when the user wants a one-off override.

## Optional Repo Cognition

GoalX always has builtin `repo-native` cognition. GitNexus is optional.

- binary install is preferred
- install: `npm install -g gitnexus@1.5.0`
- verify: `gitnexus status`
- pinned `npx gitnexus@1.5.0` is exposed only when a real probe succeeds
- GoalX does not auto-install GitNexus
- GoalX records GitNexus per worktree scope with `index_state=missing|fresh|stale|unknown`
- lifecycle refresh can best-effort run `gitnexus analyze` when the provider is available but the selected scope is missing or stale
- `goalx afford [--run NAME] [master|session-N]` can expose runnable GitNexus `status`, `query`, `context`, and `impact` commands for the selected scope
- MCP setup is optional and user-owned:
  - `codex mcp add gitnexus -- npx -y gitnexus@1.5.0 mcp`
  - `claude mcp add gitnexus -- npx -y gitnexus@1.5.0 mcp`
- GoalX is MCP-aware for GitNexus, but it does not auto-install GitNexus or mutate user MCP configs

## Public Command Matrix

### Inspect

- `goalx list`
- `goalx status [--run NAME]`
- `goalx observe [--run NAME]`
- `goalx context [--run NAME]`
- `goalx afford [--run NAME] [target]`
- `goalx attach [--run NAME] [master|session-N]`
- `goalx wait [--run NAME] [target] --timeout ...`

### Launch And Continue

- `goalx run "goal"`
- `goalx run --objective "goal"`
- `goalx run --objective-file /abs/path/to/objective.txt`
- `goalx init "goal"` then `goalx start --config .goalx/goalx.yaml`
- `goalx run --from RUN --intent debate|implement|explore`

### Control

- `goalx tell`
- `goalx add`
- `goalx replace`
- `goalx dimension`
- `goalx budget`
- `goalx focus`

### Review And Merge

- `goalx review`
- `goalx diff`
- `goalx keep --run NAME session-N`
- `goalx integrate --run NAME --method ... --from ...`
- `goalx keep --run NAME`

### Lifecycle

- `goalx park`
- `goalx resume`
- `goalx stop`
- `goalx recover`
- `goalx save`
- `goalx archive`
- `goalx drop`

### Results And Durable Surfaces

- `goalx verify`
- `goalx result`
- `goalx report`
- `goalx schema <surface>`
- `goalx durable write <surface> ...`

Canonical authoring centers:

- `obligation-model`
- `assurance-plan`
- `obligation-log`
