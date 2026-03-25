# GoalX Run Guidance Plane Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a read-only run guidance plane that exposes current runtime facts, structural anchors, and run-scoped GoalX command affordances without violating facts-not-judgments.

**Architecture:** Keep all existing canonical state files, build one shared guidance builder over them, refresh the guidance files from start/sidecar/lifecycle flows, add read-only `goalx context` and `goalx afford` commands, then retarget `status`, `observe`, and the master template to the new surface.

**Tech Stack:** Go 1.24, stdlib, existing GoalX control plane, tmux capture, JSON state files, text/template, Go tests

---

### Task 1: Lock the guidance-plane contract with failing tests

**Files:**
- Create: `cli/activity_test.go`
- Create: `cli/context_index_test.go`
- Create: `cli/affordances_test.go`
- Modify: `cli/protocol_test.go`
- Modify: `cli/status_test.go`
- Modify: `cli/observe_test.go`

**Step 1: Write failing tests for the facts plane**

Cover that `activity.json`:

- aggregates lease, queue, liveness, worktree, tmux, and runtime facts
- records last observed pane change facts without semantic labels
- does not emit recommendation, completion, or next-step fields

Useful targets:

- `TestBuildActivitySnapshotAggregatesControlFacts`
- `TestBuildActivitySnapshotTracksPaneHashChanges`
- `TestActivitySnapshotContainsNoJudgmentFields`

**Step 2: Write failing tests for the continuity plane**

Cover that `context-index.json`:

- includes stable run/control/artifact paths
- includes session roster facts
- excludes raw environment dumps and persisted PATH authority

Useful targets:

- `TestBuildContextIndexIncludesRunAnchors`
- `TestContextIndexIncludesSessionRoster`
- `TestContextIndexExcludesRawEnvSnapshot`

**Step 3: Write failing tests for the affordance plane**

Cover that affordances:

- produce exact run-scoped commands
- expose both machine and markdown forms
- describe command availability without imperative guidance

Useful targets:

- `TestBuildAffordancesIncludesRunScopedCommands`
- `TestRenderAffordancesMarkdownUsesCurrentRunPaths`
- `TestAffordancesAvoidRecommendationLanguage`

**Step 4: Write failing tests for template integration**

Require that the master template points to the new guidance surfaces instead of carrying a large static command dump.

Useful targets:

- `TestRenderMasterProtocolReferencesGoalxContextAndAfford`
- `TestRenderMasterProtocolRemovesExpandedStaticCommandInventory`

**Step 5: Run focused tests**

Run:

```bash
go test ./cli -run 'Test(BuildActivitySnapshot|ContextIndex|BuildAffordances|RenderMasterProtocol|Status|Observe)' -count=1
```

Expected: FAIL on missing files/APIs and old protocol text.

**Step 6: Commit**

```bash
git add cli/activity_test.go cli/context_index_test.go cli/affordances_test.go cli/protocol_test.go cli/status_test.go cli/observe_test.go
git commit -m "test: lock run guidance plane contract"
```

### Task 2: Implement the shared guidance builders

**Files:**
- Create: `cli/activity.go`
- Create: `cli/context_index.go`
- Create: `cli/affordances.go`
- Modify: `cli/control_state.go`
- Modify: `cli/sidecar.go`

**Step 1: Add the guidance-plane structs**

Implement concrete data models for:

- activity snapshot
- context index
- affordance document

Keep them facts-only. Do not add semantic helper enums such as `stale`, `suspect`, or `needs_attention`.

**Step 2: Build `activity.json` from canonical state**

Implement one builder that reads:

- run identity/state
- leases
- inbox/cursor
- reminders/deliveries
- liveness
- worktree snapshot
- tmux pane presence/capture

Use pane content hashing plus the prior snapshot to record `last_output_change_at` mechanically.

**Step 3: Build `context-index.json` from structural state**

Implement one builder that reads:

- run metadata
- run charter
- run spec
- session identities
- session runtime state

Write structural anchors only. No raw environment snapshot file and no policy fields.

**Step 4: Build `affordances.json` and `affordances.md`**

Generate exact run-scoped command strings and path affordances from the context index and run facts.

Hard rules:

- no "you should"
- no "recommended next step"
- no completion claims

**Step 5: Add one refresh entry point**

Add one helper such as:

```go
func RefreshRunGuidance(projectRoot, runName, runDir string) error
```

This helper is the only path used by start, sidecar, lifecycle, status, observe, `context`, and `afford`.

**Step 6: Run focused tests**

Run:

```bash
go test ./cli -run 'Test(BuildActivitySnapshot|ContextIndex|BuildAffordances)' -count=1
```

Expected: PASS

**Step 7: Commit**

```bash
git add cli/activity.go cli/context_index.go cli/affordances.go cli/control_state.go cli/sidecar.go
git commit -m "feat: add run guidance builders"
```

### Task 3: Refresh guidance during run lifecycle changes

**Files:**
- Modify: `cli/start.go`
- Modify: `cli/add.go`
- Modify: `cli/lifecycle.go`
- Modify: `cli/relaunch.go`
- Modify: `cli/start_test.go`
- Modify: `cli/add_test.go`
- Modify: `cli/lifecycle_test.go`

**Step 1: Seed guidance on run creation**

After protocol render and control bootstrap, call the shared refresh helper so every new run starts with guidance files already present.

**Step 2: Refresh on roster-changing commands**

Refresh guidance after:

- add
- park
- resume
- replace
- relaunch

Do not add per-command custom rendering logic. Call the same refresh helper.

**Step 3: Refresh from the sidecar tick**

After liveness/worktree/identity refresh, have `runSidecarTick` refresh the guidance plane so the derived facts stay current during long runs.

**Step 4: Run focused tests**

Run:

```bash
go test ./cli -run 'Test(Start|Add|Resume|Park|Replace|Relaunch).*Guidance' -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add cli/start.go cli/add.go cli/lifecycle.go cli/relaunch.go cli/start_test.go cli/add_test.go cli/lifecycle_test.go
git commit -m "refactor: refresh run guidance through lifecycle flows"
```

### Task 4: Add read-only `goalx context` and `goalx afford`, then retarget `status` and `observe`

**Files:**
- Create: `cli/context.go`
- Create: `cli/afford.go`
- Modify: `cli/status.go`
- Modify: `cli/observe.go`
- Modify: `cmd/goalx/main.go`
- Modify: `cmd/goalx/main_test.go`
- Modify: `cli/serve.go`

**Step 1: Add the new CLI commands**

Implement:

- `goalx context [--run NAME] [--json]`
- `goalx afford [--run NAME] [master|session-N] [--json]`

These commands should render from the shared guidance builder output only.

**Step 2: Retarget `status`**

Make `goalx status` read the activity/context surfaces for its summary instead of independently stitching control files.

**Step 3: Retarget `observe`**

Keep live tmux capture, but source the surrounding run/control summary from the guidance plane.

**Step 4: Expose the commands in the CLI entry points**

Wire `context` and `afford` into:

- `cmd/goalx/main.go`
- `cli/serve.go` command dispatch

**Step 5: Run focused tests**

Run:

```bash
go test ./cmd/goalx ./cli -run 'Test(Main|Status|Observe|Context|Afford)' -count=1
```

Expected: PASS

**Step 6: Commit**

```bash
git add cli/context.go cli/afford.go cli/status.go cli/observe.go cmd/goalx/main.go cmd/goalx/main_test.go cli/serve.go
git commit -m "feat: add guidance commands and retarget status surfaces"
```

### Task 5: Slim the protocol and docs to the new guidance model

**Files:**
- Modify: `templates/master.md.tmpl`
- Modify: `README.md`
- Modify: `cli/protocol.go`
- Modify: `cli/protocol_test.go`

**Step 1: Reduce static command narration**

Replace the current expanded run-local command listing with a short guidance reference that points the master to:

- `goalx context --run {{.RunName}}`
- `goalx afford --run {{.RunName}}`
- the `control/` guidance files

Keep the orchestration rules and facts-not-judgments contract intact.

**Step 2: Update README**

Document:

- what the guidance plane is
- what `goalx context` and `goalx afford` do
- that the guidance plane is read-only and not a second source of truth
- that GoalX still does not make semantic judgments

**Step 3: Run focused tests**

Run:

```bash
go test ./cli -run 'TestRenderMasterProtocol|TestContext|TestAfford' -count=1
```

Expected: PASS

**Step 4: Commit**

```bash
git add templates/master.md.tmpl README.md cli/protocol.go cli/protocol_test.go
git commit -m "docs: point master protocol to run guidance plane"
```

### Task 6: Full verification

**Files:**
- Verify: `cli/*.go`
- Verify: `cmd/goalx/main.go`
- Verify: `templates/master.md.tmpl`
- Verify: `README.md`

**Step 1: Run targeted packages**

Run:

```bash
go test ./cmd/goalx ./cli -count=1
```

**Step 2: Run the full project checks**

Run:

```bash
make check
```

**Step 3: Manual CLI verification**

Create a disposable run and verify:

```bash
goalx auto "guidance-plane smoke test"
goalx context --run guidance-plane-smoke-test --json
goalx afford --run guidance-plane-smoke-test
goalx status --run guidance-plane-smoke-test
goalx observe --run guidance-plane-smoke-test
goalx stop --run guidance-plane-smoke-test
goalx drop --run guidance-plane-smoke-test
```

Confirm:

- guidance files are present under `control/`
- `status` and `observe` reflect the same underlying facts
- no command output contains framework-generated judgments or next-step recommendations

**Step 4: Review the final contract**

Before merge, explicitly check that the new surfaces:

- expose facts only
- do not revive persisted launch-env authority
- do not require a second compatibility path
- improve agent usability without moving orchestration into GoalX
