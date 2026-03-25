# GoalX Run Guidance Plane Design

**Date:** 2026-03-25

## Decision

Add one read-only, run-scoped **guidance plane** for agents:

- `control/activity.json` for aggregated runtime facts
- `control/context-index.json` for durable structural anchors
- `control/affordances.json` and `control/affordances.md` for run-scoped GoalX command and path affordances

Also add two read-only CLI entry points:

- `goalx context`
- `goalx afford`

This is a hard cutover in direction. Do not add a supervisor, policy engine, semantic judge, or compatibility-shaped duplicate logic. The guidance plane is a derived read-model over existing canonical run state.

## Why This Round Exists

`main` is already moving in the right direction:

- config load and config resolution are separated
- master is explicitly the orchestrator, not the implementer
- acceptance and completion are moving toward facts-not-judgments
- launch behavior follows the current caller environment instead of a persisted shell snapshot

But the current runtime surface is still fragmented:

1. sidecar already records useful facts, but they are split across `control/run-state.json`, leases, reminders, deliveries, `liveness.json`, `worktree-snapshot.json`, `identity-fence.json`, runtime state, and tmux capture
2. `goalx status` and `goalx observe` reconstruct different views ad hoc, so they are useful for humans but not stable enough for agents
3. the master template still injects a large static operating manual instead of exposing a small run-scoped command surface
4. long-running runs preserve durable files, but not one compact orientation surface that helps Claude/Codex quickly recover context

The product gap is not "GoalX needs to decide more." The gap is that GoalX does not yet expose its existing facts and tools in the cleanest form for agents.

## Product Constraints

This design must preserve the product line:

- GoalX provides storage, execution, connectivity, and guidance surfaces
- agents do the interpretation, planning, orchestration, and completion judgment
- the framework may expose facts and available tools
- the framework may not infer whether the goal is complete, which path is best, or what the next action should be

## Goals

- Give agents one compact, run-scoped runtime fact surface.
- Give agents one compact, run-scoped command surface.
- Give long-running runs one durable structural context index.
- Improve `status` and `observe` by reading the same facts the agents read.
- Reduce static command dump in templates.
- Keep everything facts-first and read-only from the framework side.

## Non-Goals

- No supervisor that owns the master loop.
- No semantic labels such as `stale`, `idle`, `blocked`, `should_verify`, or `done`.
- No framework-generated "next step" or "recommended action".
- No raw shell environment snapshot file returning as a durable authority.
- No second source of truth for run state.
- No compatibility shims that preserve an old prompt-heavy control model beside the new one.

## Current State

GoalX already has the right raw materials:

- immutable anchors: run charter, run metadata, session identities
- control-plane facts: inbox, cursor, reminders, deliveries, leases, control run state
- sidecar-maintained facts: liveness and worktree snapshot
- runtime facts: run/session runtime state
- transport facts: tmux session and pane capture

The problem is shape, not absence. These facts exist, but they are not exposed as one stable guidance surface.

## Options

### Option 1: Patch `status` and `observe` only

Keep the current control plane as-is and improve the human CLI output.

Pros:

- smallest diff

Cons:

- helps humans more than agents
- keeps prompt injection as the main command discovery surface
- does not improve long-run orientation

### Option 2: Add a framework-owned supervisor

Wrap the master in a stronger loop that decides when to wait, wake, or continue.

Pros:

- can reduce prompt-level drift

Cons:

- pushes GoalX toward orchestration ownership
- violates the product boundary
- turns runtime control into policy instead of infrastructure

### Option 3: Add a read-only run guidance plane

Keep agent autonomy intact, but expose current run facts, structural anchors, and relevant GoalX commands through compact derived surfaces.

Pros:

- aligned with the product boundary
- improves both agent usability and human observability
- builds on the existing sidecar/control plane instead of replacing it

Cons:

- touches multiple surfaces at once: sidecar, status, observe, protocol, docs

## Decision

Choose **Option 3**.

This is the best leverage point that improves observability, command usability, and continuity without moving GoalX across the facts-not-judgments boundary.

## Design

### 1. Canonical State Does Not Change

The canonical sources remain the existing files:

- `run-spec.yaml`
- `run-charter.json`
- `state/run.json`
- `state/sessions.json`
- `control/run-identity.json`
- `control/run-state.json`
- `control/identity-fence.json`
- `control/inbox/*`
- `control/reminders.json`
- `control/deliveries.json`
- `control/liveness.json`
- `control/worktree-snapshot.json`
- session identities and journals

The guidance plane is derived from those files plus current tmux facts. It is not a second write authority.

### 2. Facts Plane: `control/activity.json`

`activity.json` is the compact runtime fact surface for one run.

It should aggregate:

- run identity facts: run id, epoch, project id, run name, tmux session
- lifecycle facts: control lifecycle state, runtime phase, active flag
- lease facts: master and sidecar lease health, pid, transport, renewal timestamps
- queue facts: unread master inbox count, urgent unread present, reminders due, delivery failures
- output facts: per-pane presence, last observed pane hash, last observed pane change time
- file freshness facts: last update times for identity fence, liveness, worktree snapshot, run state, sessions state, status
- worktree facts: root diff stat, per-session diff stat
- liveness facts: per-session pid alive, lease health, journal stale minutes

`activity.json` must not contain:

- inferred state labels such as `stuck`, `needs_nudge`, `nearly_done`
- recommended actions
- completion claims
- policy outcomes

Example shape:

```json
{
  "version": 1,
  "checked_at": "RFC3339",
  "run": {
    "project_id": "data-dev-autoresearch",
    "run_name": "harness-master-agent",
    "run_id": "run_abc123",
    "epoch": 1,
    "tmux_session": "gx-data-dev-autoresearch-harness-master-agent"
  },
  "lifecycle": {
    "control_state": "active",
    "runtime_phase": "working",
    "run_active": true
  },
  "queue": {
    "master_unread": 0,
    "urgent_unread": false,
    "reminders_due": 1,
    "deliveries_failed": 0,
    "last_master_wake_at": "RFC3339"
  },
  "actors": {
    "master": {
      "lease": "healthy",
      "pid": 12345,
      "pid_alive": true,
      "pane_present": true,
      "pane_hash": "sha256:...",
      "last_output_change_at": "RFC3339"
    }
  }
}
```

The only acceptable derivation is mechanical aggregation of already-existing facts. No semantic interpretation.

### 3. Continuity Plane: `control/context-index.json`

`context-index.json` is the durable structural orientation surface.

It should answer:

- where the run lives
- what the stable control files are
- which sessions currently exist
- what the current role/provider surface looks like
- where the key artifacts and journals are

It should include:

- project root, run dir, run worktree, reports dir
- run charter, goal, acceptance, proof, coordination, summary paths
- control file paths
- master engine/model and run mode
- current session roster with names, modes, window names, worktree paths, journal paths, inbox paths, cursor paths
- current dimensions path and routing surface paths

It may include capability facts only as observed facts, for example:

- `claude_code_available: true`
- `codex_available: true`
- `git_available: true`
- `tmux_available: true`

It must not include:

- a raw environment variable dump
- a persisted PATH snapshot treated as launch authority
- recommended commands

This file is the long-run structural anchor for reorientation after compaction, wake cycles, and resumed work.

### 4. Affordance Plane: `control/affordances.json` and `control/affordances.md`

The affordance plane is the run-scoped command surface.

It exists to answer:

- what GoalX commands are relevant right now
- what exact command strings should be used for this run
- what file paths matter for this run

It should be generated from current run facts plus the known GoalX command surface.

Each affordance entry should contain:

- `id`
- `kind`: observe, control, session, save, closeout, phase, path
- `summary`
- `command`
- `when`
- `paths`

Example:

```json
{
  "id": "observe",
  "kind": "observe",
  "summary": "Read current transport output and control facts for this run.",
  "command": "goalx observe --run harness-master-agent",
  "when": "Use when you need current run output and control state."
}
```

Important boundary:

- affordances describe available actions
- affordances do not say which action is correct

The Markdown form is optimized for Claude-style prompt consumption. The JSON form is optimized for Codex and machine-friendly inspection.

### 5. CLI Surface

Add two read-only commands:

#### `goalx context`

Purpose:

- print `context-index.json` in either human-readable or JSON form

Suggested shape:

```bash
goalx context --run NAME
goalx context --run NAME --json
```

#### `goalx afford`

Purpose:

- print the run-scoped command surface in either Markdown or JSON form

Suggested shape:

```bash
goalx afford --run NAME
goalx afford --run NAME --json
goalx afford --run NAME session-2 --json
```

These commands are read-only. They do not mutate run state and do not encode policy.

### 6. Refresh Model

Use one shared builder path. Do not create separate logic for file refresh, status rendering, observe rendering, and CLI commands.

Refresh ownership:

- `goalx start` seeds the guidance plane once after the run is created
- `goalx sidecar` refreshes the guidance plane on every tick after liveness/worktree refresh
- lifecycle commands that change the session roster or ownership refresh the guidance plane immediately after mutation:
  - `goalx add`
  - `goalx park`
  - `goalx resume`
  - `goalx replace`
  - relaunch flows

`goalx status`, `goalx observe`, `goalx context`, and `goalx afford` should all read from the same builder output.

### 7. Template Changes

`templates/master.md.tmpl` should shrink its static command narration.

Keep:

- charter-first identity
- control-loop discipline
- facts-not-judgments completion semantics

Move out:

- long command lists
- repeated path inventories
- run-specific command strings

Replace with a compact note that the agent can consult:

- `goalx context --run {{.RunName}}`
- `goalx afford --run {{.RunName}}`
- the guidance plane files under `control/`

This keeps the protocol smaller while giving the agent a better way to discover run-local tools.

### 8. Status And Observe

`goalx status` and `goalx observe` should consume the guidance plane instead of reconstructing their own independent summaries.

That means:

- one runtime fact model for humans and agents
- fewer cases where the pane is active but durable summaries look stale
- less duplication across CLI commands

`observe` should still show live tmux capture when present, but the surrounding summary should come from `activity.json` and `context-index.json`.

### 9. Facts-Not-Judgments Guardrails

The implementation must preserve these rules:

- never write semantic labels like `good`, `bad`, `done`, `stuck`, `healthy-enough`
- never compute "next action"
- never infer completion from verify output or code changes
- never infer orchestration intent from free text
- never persist raw shell env as a durable run authority

If a field reads like a recommendation, it does not belong in the guidance plane.

## Why This Fits The Product Direction

This design keeps GoalX in the role of infrastructure:

- storage: durable run facts and indexes
- execution: existing process/tmux lifecycle
- connectivity: inbox/reminder/delivery plumbing
- guidance: read-only facts and command affordances

It explicitly does **not** make GoalX the orchestrator.

## Cutover

This should be a clean cutover:

- no second prompt-heavy command surface kept alive
- no dual `status` logic with separate semantic summaries
- no compatibility branch that preserves an old "agent learns GoalX from template dump" model

One guidance builder, one guidance plane, one read-only CLI surface.
