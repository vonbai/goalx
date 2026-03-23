# GoalX Daemonless Run Control V2 Design

## Status

Approved for planning on 2026-03-23.

## Decision

Adopt a daemonless, run-scoped control architecture for GoalX.

The new control plane remains local-first and does not depend on a global daemon. Each active run owns its own durable control store plus a small run-scoped sidecar process. tmux remains a transport and UI container, not the protocol center.

## Problem Statement

Current GoalX behavior has four coupled fault lines:

1. Run resolution is global-first, so bare `--run NAME` can cross project boundaries.
2. Heartbeat, liveness, and wake delivery are coupled in the same path.
3. Delivery uses tmux pane scraping plus best-effort keystrokes, which is unsafe for Codex and fragile in general.
4. Completion proof is fragmented across goal contract, acceptance state, runtime state, and summary text, so the system can mark a run complete without a canonical proof manifest.

These faults create the symptoms seen in the investigation:

- wrong-project status lookup
- stale control summary after stop
- wake messages delivered through unsafe input semantics
- "goal complete" decisions that depend too much on writable checklist text

## Goals

- Make run targeting local-first and explicit across projects.
- Separate timer, liveness, reminder, and delivery concerns.
- Make reminders durable, deduplicated, retryable, and transport-agnostic.
- Replace tmux-screen-scrape wake semantics with explicit delivery state.
- Make `status` and `observe` derive from durable control state first, tmux second.
- Introduce a canonical proof manifest for completion.
- Preserve local, daemonless operation for the primary architecture.
- Roll out the protocol across every `ResolveRun` consumer and every user-facing read-model so no command silently keeps v1 selector or tmux-first semantics.

## Non-Goals

- Do not introduce a global always-on `goalxd` daemon.
- Do not keep long-term dual semantics for v1 and v2 protocols.
- Do not preserve "global-first bare run selector" behavior.
- Do not continue supporting bare `Enter` wake delivery for Codex.

## Architecture Summary

GoalX control v2 has five layers:

1. Identity
2. Durable control store
3. Run-scoped sidecar
4. Transport adapters
5. Proof and completion

### 1. Identity

Every run gets a stable `run_id` and a monotonic `epoch`.

- `run_id` identifies the run across CLI, HTTP, status cache, and control objects.
- `epoch` fences stale sidecars, stale deliveries, and stale events from older restarts.

Rules:

- Bare `--run NAME` is local-first.
- Cross-project lookup requires `project-id/run` or `run_id`.
- Mutating commands never use implicit global-first resolution.

### 2. Durable Control Store

All active control semantics live under `control/` in the run dir.

#### Durable Objects

- `control/run-identity.json`
  - `run_id`, `project_id`, `project_root`, `run_name`, `epoch`, `protocol_version`, `created_at`
- `control/run-state.json`
  - `lifecycle_state`, `phase`, `recommendation`, `active_session_count`, `last_event_id`, `updated_at`
- `control/events.jsonl`
  - append-only event stream
- `control/leases/master.json`
- `control/leases/sidecar.json`
- `control/leases/session-*.json`
  - `holder`, `epoch`, `renewed_at`, `expires_at`, `pid`, `transport`
- `control/inbox/master.jsonl`
  - actionable queue for the master
- `control/inbox/session-*.jsonl`
  - actionable queue for sessions; may coexist with guidance files during short migration
- `control/reminders.json`
  - `reminder_id`, `dedupe_key`, `reason`, `target`, `cooldown_until`, `attempts`, `acked_at`, `suppressed`
- `control/deliveries.json`
  - `delivery_id`, `message_id`, `target`, `adapter`, `status`, `last_error`, `attempted_at`, `acked_at`

#### Event Types

- `run_started`
- `run_stopping`
- `run_stopped`
- `run_completed`
- `lease_renewed`
- `lease_expired`
- `message_enqueued`
- `delivery_requested`
- `delivery_succeeded`
- `delivery_failed`
- `delivery_acked`
- `session_spawned`
- `session_exited`
- `session_parked`
- `session_resumed`
- `required_item_updated`
- `required_item_uncovered`
- `verification_requested`
- `verification_passed`
- `verification_failed`
- `reminder_due`
- `reminder_suppressed`

### 3. Run-Scoped Sidecar

Each active run owns one small local sidecar process.

Responsibilities:

- renew its own lease
- inspect master/session leases
- append system events
- maintain derived run health
- request deliveries through the delivery queue

Non-responsibilities:

- no screen scraping
- no direct protocol reasoning
- no contract ownership
- no hidden global state

The sidecar is supervised by run lifecycle, not by a global daemon.

### 4. Transport Adapters

Adapters deliver messages to actual agent environments.

Initial adapters:

- tmux adapter
- Codex/Claude adapter layer built on existing adapter generation

Adapter rules:

- adapters consume delivery requests
- adapters update `control/deliveries.json`
- adapters must use explicit pane ids or explicit targets
- adapters must not infer wake state by pane text
- adapters must never send a bare `Enter` because of scraped UI state

### 5. Proof And Completion

Introduce `proof/completion.json` as the canonical closeout manifest.

This manifest joins:

- goal contract coverage
- acceptance command result
- completion provenance
- semantic evidence

Each required item must map to:

- `requirement_id`
- `evidence_paths`
- `evidence_class`
- `counter_evidence`
- `semantic_match`
- `satisfaction_basis`

Completion is allowed only when the proof manifest is satisfied. Textual checklist output is not enough.

## State Machines

### Run Lifecycle

- `starting`
- `active`
- `draining`
- `stopping`
- `stopped`
- `completed`
- `degraded`

Rules:

- `completed` is a proof outcome, not a transport outcome.
- `stopped` is terminal for liveness and reminder purposes.
- `degraded` means the run is still logically active but one or more leases or transports are unhealthy.

### Lease State

- `healthy`
- `late`
- `expired`

Lease expiration degrades the run or session; it does not auto-complete or auto-stop the run.

### Reminder State

- `queued`
- `cooldown`
- `sent`
- `acked`
- `failed`
- `suppressed`

Heartbeat no longer implies reminder delivery. Reminder delivery is a separate state machine driven by dedupe and cooldown rules.

## Command Semantics

### Explicit Rollout Surface

The v2 rollout is not complete unless all of these surfaces move together:

- selector consumers: `status`, `attach`, `drop`, `diff`, `stop`, `review`, `save`, `tell`, `keep`, `archive`, `lifecycle`, `verify`, `add`, `observe`, `pulse`, `report`, `serve`
- read-model commands: `list`, `next`, `status`, `observe`, `serve /runs`
- transport-specific commands: `attach`, `stop`, `drop`
- protocol assets: master/program templates, generated adapter hooks, shipped skills, README and deploy docs

The rule is simple: no command may continue to imply that tmux session presence is the source of truth for run identity, liveness, or completion.

### `goalx start`

- creates v2 identity and control objects
- starts master
- starts run-scoped sidecar
- does not create a `heartbeat` tmux window

### `goalx stop`

- appends `run_stopping`
- marks run terminal in `control/run-state.json`
- clears reminder backlog
- expires or revokes leases
- appends `run_stopped`
- stops sidecar and transport windows
- does not leave stale wake state visible in `status`

### `goalx tell`

- writes message into target inbox
- creates delivery requests
- does not directly send best-effort keystrokes as the primary semantic path

### `goalx status`

- derives run health from control store first
- shows terminal state for stopped runs
- shows legacy notice for v1 runs without rewriting them

### `goalx observe`

- reads durable run state first
- if tmux exists, appends transport capture
- if tmux is absent, still works in control/journal mode

## Status Cache Discipline

`status.json` becomes a derived read-model only.

Rules:

- no reverse synchronization from project status cache back into run state
- no active/inactive truth derived from tmux session presence alone
- no control health derived from stale heartbeat residue

## Serve API Discipline

The HTTP API must use the same resolver and derived-state logic as CLI.

Rules:

- `/runs` derives state from run identity + run state + lease health
- `tell` returns queue or delivery identifiers, not just tmux targeting assumptions
- `observe` and `status` must support degraded transport

## Documentation And Skill Synchronization

The protocol change is user-visible and operator-visible. Documentation and shipped skills are part of the contract surface and must move in the same release train as the code.

Files that must be synchronized:

- `README.md`
- `deploy/README.md`
- `skill/SKILL.md`
- `skill/references/advanced-control.md`
- `skill/openclaw-skill/SKILL.md`
- `skill/agents/openai.yaml`

Required updates:

- remove or rewrite heartbeat-window-centered mental models
- describe sidecar-based run monitoring and degraded transport semantics
- change bare run targeting guidance from global-first to local-first
- explain explicit cross-project selectors using `project-id/run` or `run_id`
- update `status`, `observe`, and `tell` behavior descriptions to match control-v2 semantics
- update remote/OpenClaw skill examples so HTTP API consumers reason about derived run state and durable delivery, not tmux-centric liveness
- update local skill guidance so operators stop treating heartbeat lag and raw tmux intervention as the primary control path

## Compatibility Strategy

Use a short migration window:

1. read old, write new for explicit v2 runs only
2. keep legacy runs readable without implicit rewrite
3. remove old heartbeat/nudge path once v2 coverage is complete

Rules:

- v1 runs are readable in read-only compatibility mode
- no implicit upgrade during `status`, `observe`, `report`, `save`, or `verify`
- migration, if needed, must be explicit
- protocol version and per-file schema versions must be defined separately

## Protocol Changes

`templates/master.md.tmpl` and `templates/program.md.tmpl` must be updated in the same change set as the control-plane implementation.

New protocol assumptions:

- heartbeats mean lease renewal, not wake semantics
- real instructions live in inbox and delivery objects
- completion requires proof manifest, not only textual checklists
- master must react to event queues and lease health, not to `goalx-hb` pane text

## Risks

- introducing sidecar plus keeping old pulse path would create double writers
- changing run resolution without updating serve and all mutating commands would preserve hidden cross-project bugs
- leaving `status.json` bidirectional would keep split-brain behavior
- upgrading protocol templates after Go code would leave agents reasoning with old semantics

## Rollout Order

1. Identity and protocol version
2. Control store v2 objects
3. Sidecar lifecycle
4. Delivery/reminder engine
5. Derived status and observe
6. Serve API and resolver convergence
7. Proof manifest and verify tightening
8. README, deploy docs, and skill synchronization
9. Legacy compatibility cleanup

## Acceptance For This Design

The implementation is acceptable only if it:

- removes global-first bare run mutation behavior
- removes heartbeat-window control semantics
- removes bare-`Enter` Codex wake behavior
- makes stopped runs display terminal state without stale control confusion
- adds canonical proof manifest enforcement
- synchronizes README, deploy docs, and shipped skills with control-v2 semantics
- keeps legacy runs observable without implicit migration
