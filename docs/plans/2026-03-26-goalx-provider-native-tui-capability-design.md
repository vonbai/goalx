# GoalX Provider-Native TUI Capability Design

Date: 2026-03-26
Status: Approved
Owner: Codex

## Decision

GoalX should standardize on one canonical provider runtime:

- `tmux + interactive TUI`

Everything else is secondary:

- `claude -p`
- `codex exec`

These may still be used for capability probing or local diagnostics, but they are not the runtime contract GoalX is designed around.

Within that runtime, GoalX should adopt a provider-native capability model:

- preserve each provider's native skill / plugin / MCP surface
- expose capability facts and blocked facts
- keep routing based on task/risk, not skill inventory
- keep recovery in infrastructure, not policy

GoalX should not pursue `Claude root + bypassPermissions` as a default design target. In the current deployment model, that path is provider-blocked and pushes the framework into unnecessary complexity.

## Problem

Recent GoalX issues around skill usage, MCP prompts, and blocked sessions all share one root cause:

GoalX has been reasoning about provider capabilities too abstractly.

In practice, `claude-code` and `codex` do not expose the same runtime surface:

- Claude TUI exposes skills, plugins, slash/command surfaces, and MCP dialogs
- Codex TUI exposes skills through a native `/skills` UI and has its own MCP behavior
- Claude root sessions hard-block permission bypass modes
- Codex does not share the same root restriction

Trying to flatten these into one generic "skill" or one generic "permission model" creates fake capability surfaces.

That leads to the wrong framework incentives:

- chasing a Claude-specific bypass flag as if it were the product goal
- treating skill visibility as a routing truth
- assuming provider-native invocation flows are interchangeable
- adding framework commands where the master already has shell/tmux/worktree power

## Key Findings

### 1. GoalX runtime truth is TUI, not print/exec

Local tmux experiments showed that `-p` / `exec` are useful probes, but they are not representative enough to define GoalX runtime semantics.

The canonical runtime contract must match how GoalX actually operates:

- long-lived tmux panes
- provider-owned TUI state
- real permission / dialog / queued-input behavior

### 2. Claude root bypass is provider-blocked

On this host, under root:

- `--dangerously-skip-permissions` fails immediately
- `--permission-mode bypassPermissions` resolves to the same failure
- `--allow-dangerously-skip-permissions` does not change that outcome

The failure is explicit:

- `--dangerously-skip-permissions cannot be used with root/sudo privileges for security reasons`

Running the same Claude binary as a non-root user removes that root-specific error and advances to the next gate (`Not logged in`), which shows the restriction is bound to effective user identity, not tmux or GoalX.

### 3. Claude TUI already exposes usable native capability

In real tmux TUI tests, Claude:

- listed skills/plugins/MCP servers/MCP tools
- explicitly accepted a named skill request for `impeccable:frontend-design`
- surfaced a real Playwright MCP permission menu when asked to navigate

That means the problem is not "Claude cannot use installed capability." The problem is how GoalX models and recovers around that capability.

### 4. Codex TUI capability is real but not Claude-shaped

In real tmux TUI tests, Codex:

- booted with native context7 MCP ready
- exposed a native `/skills` menu
- advertised that skills are part of its own TUI interaction model

That means Codex does have provider-native skill capability, but GoalX must not assume the invocation shape matches Claude.

### 5. Whole-home mirroring for non-root Claude is not elegant

Local inspection of `~/.claude` shows it mixes:

- stable config
- credentials
- plugin registry
- project/session history
- todos/debug/telemetry/cache state

Mirroring or symlinking the whole home to a second user couples mutable session state, credentials, and provider cache in ways that are brittle and high-maintenance.

## Goals

- Preserve provider-native capability in GoalX TUI sessions.
- Let sessions use installed skills/MCP naturally when materially helpful.
- Make explicit user/master skill requests first-class without turning skill inventory into routing truth.
- Surface blocked provider state as infrastructure facts.
- Keep master judgment in the agent, not in framework policy.
- Avoid new duplicate command paths for direct intervention.

## Non-Goals

- Do not create a unified skill registry as the source of truth.
- Do not create a universal skill invocation language across providers.
- Do not make skill presence the primary task routing standard.
- Do not make non-root Claude worker infrastructure the default GoalX runtime.
- Do not share an entire Claude home directory between users.
- Do not add a new framework takeover command when shell/tmux already exist.

## Proposed Model

### 1. Canonical runtime: provider-native TUI

GoalX should treat provider runtime as:

- a tmux pane
- owned by the provider's interactive TUI
- sampled by GoalX only at the fact/recovery layer

Provider-native surfaces stay provider-native.

GoalX does not attempt to normalize:

- Claude skills into Codex skill menus
- Codex skill menus into Claude slash/plugin language
- one provider's permission UX into another's

### 2. Two fact families only

To avoid concept sprawl, GoalX should standardize on two fact families.

#### Capability facts

These answer: "What can this session use right now?"

Examples:

- engine = `claude-code` / `codex`
- runtime mode = `tui`
- provider-native skills visible
- provider-native plugins visible
- configured MCP servers visible
- permission bypass available = true/false
- root-guarded capability = true/false

These belong in existing read surfaces:

- `goalx context`
- `goalx afford`
- `goalx status`
- `goalx observe`

They are facts, not routing advice.

#### Blocked facts

These answer: "What is this session currently stuck on?"

Examples:

- `provider_dialog_visible`
- `permission_prompt_visible`
- `elicitation_visible`
- `skill_ui_visible`
- `capacity_picker_visible`

These are dynamic transport/runtime observations and belong with the existing sidecar / transport fact path.

### 3. Thin protocol contract, not provider reimplementation

Master and subagent protocols should only say:

- provider-native skills/plugins/MCP are allowed when materially helpful
- if the user or master explicitly names a capability and it is visible, use it before the default flow
- if the named capability is not visible, report that immediately
- do not invoke meta/orchestration capability that starts a competing control loop

This is the right amount of prompt logic:

- it exposes the execution contract
- it does not encode fake provider semantics
- it does not maintain a second skill registry in prose

### 4. Routing remains task/risk first

Routing should continue to depend on:

- task type
- autonomy risk
- expected file scope
- browser/MCP need
- provider limits

Routing should not depend primarily on skill inventory.

Skill matters as execution leverage, not as the truth source for dispatch.

This yields the practical default:

- Claude: orchestration, research, review, attended browser/MCP
- Codex: low-friction unattended execution, larger code change slices, lower permission friction work

### 5. Recovery remains infrastructure-level

Sidecar should remain the final backstop, not the primary policy engine.

Responsibilities:

- detect blocked facts
- surface urgent facts to master
- use existing recover/relaunch paths when the pane is stuck

Responsibilities it should not take:

- deciding the business fallback itself
- silently completing provider-specific interaction flows
- pretending transport success means semantic success

Claude-specific hooks should still be used where the provider offers them. Where a provider lacks equivalent hooks, GoalX may rely on pane/block heuristics as a final fallback.

### 6. Non-root Claude worker is optional advanced mode only

A non-root Claude worker remains a valid advanced configuration for users who explicitly want it.

But it is not the default design because:

- it adds user/account/bootstrap complexity
- it requires auth and plugin state under a second user
- it complicates worktree write permissions

If ever implemented, it must be done as:

- dedicated non-root provider user
- separate provider home
- explicit bootstrap of stable config
- separate login/token ownership

It must not be done as:

- symlinking the entire root Claude home
- sharing credentials and mutable session state between users

## Static Capability Surfaces

GoalX should expose provider-native capability by deriving from local truth already present on disk.

Examples:

- Claude:
  - `~/.claude.json`
  - `~/.claude/settings.json`
  - `~/.claude/skills`
  - `~/.claude/plugins/installed_plugins.json`
  - worktree `.claude/settings.local.json`
- Codex:
  - `~/.codex/config.toml`
  - `~/.codex/skills`
  - configured MCP servers

This is still "facts, not judgments." It is discovery of configured local capability, not inference about task success.

## Observe / Status Behavior

`goalx status` and `goalx observe` should become the primary human-readable surface for:

- provider capability summary
- blocked provider state
- whether a session is running under a root-guarded limitation

This gives the master and the user a shared mental model without adding a second orchestration channel.

## Rejected Alternatives

### 1. Make Claude bypass the default goal

Rejected because it is provider-blocked under the current root deployment model and pushes GoalX into account/bootstrap complexity that does not serve the product's core promise.

### 2. Add a unified skill registry/taxonomy

Rejected because it would drift from provider truth and create fake ability surfaces.

### 3. Add a new takeover/inject framework command

Rejected because it duplicates shell/tmux power and violates the current product discipline around one concern, one path.

### 4. Symlink or mirror the whole Claude home

Rejected because it mixes credentials, plugin registry, cache, project history, and session state across users.

## Rollout

### Phase 1

- standardize runtime language on `tmux + TUI`
- expose capability facts in context/afford/status/observe
- tighten protocol wording around explicit named capability usage

### Phase 2

- expand blocked fact coverage for provider dialogs and pickers
- keep sidecar as the final recovery backstop across providers

### Phase 3

- optionally add an advanced non-root Claude preset for users who explicitly want it

This phase is optional and should not block the mainline design.

## Summary

The clean design is not "make Claude root sessions bypass permissions at all costs."

The clean design is:

- TUI-first
- provider-native
- facts-first
- routing by task/risk
- skill as execution leverage
- sidecar as final blocked-state recovery

That keeps GoalX close to its product promise without building a second, less reliable version of provider runtime inside the framework.
