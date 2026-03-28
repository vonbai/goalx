# GoalX Configuration

GoalX works with zero config. Use configuration only when you need explicit control.

## Config Locations

- user-level: `~/.goalx/config.yaml`
- project-level: `.goalx/config.yaml`

## Typical Example

```yaml
master:
  engine: claude-code
  model: opus

roles:
  research: { engine: codex, model: gpt-5.4, effort: high }
  develop:  { engine: codex, model: gpt-5.4, effort: medium }

routing:
  profiles:
    research_deep: { engine: claude-code, model: opus, effort: high }
    build_fast:    { engine: codex, model: gpt-5.4-mini, effort: minimal }
  rules:
    - role: research
      any_dimensions: [depth]
      efforts: [high, max]
      profile: research_deep
    - role: develop
      efforts: [minimal, low]
      profile: build_fast

preferences:
  research:
    guidance: "Default gpt-5.4 high. Use opus for deep analysis."
  develop:
    guidance: "Default gpt-5.4 medium. Use fast for simple fixes."

local_validation:
  command: "go build ./... && go test ./... && go vet ./..."
```

## Principles

- Keep one resolver path. Do not invent alternate config entrypoints.
- Use overrides only when they clearly improve execution.
- Explicit `--engine/--model` is an override, not the default path.
- Unknown config should fail loudly, not degrade silently.

## What Config Is For

- pinning master or role defaults
- routing certain slices to different engines
- setting local validation

## What Config Is Not For

- encoding orchestration judgment in the framework
- replacing the normal `goalx run "goal"` path
- hand-editing live run state
