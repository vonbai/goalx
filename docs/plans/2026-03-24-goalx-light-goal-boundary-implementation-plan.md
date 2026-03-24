# GoalX Lightweight Goal Boundary Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace GoalX's heavy `goal-contract + acceptance + proof` model with a lighter boundary model built around `goal.json`, simplified `acceptance.json`, audit-only `goal-log.jsonl`, and canonical `proof/completion.json`.

**Architecture:** Keep the immutable user objective in run metadata, make `goal.json` the only mutable boundary snapshot, make `acceptance.json` the only mutable gate definition, make `goal-log.jsonl` audit-only, and make `goalx verify` the only closeout writer. Cut over bootstrap, prompts, verify, stop-gate, runtime/read models, and saved artifacts in a single hard-cut path with no dual-write protocol compatibility.

**Tech Stack:** Go 1.21, tmux, JSON/JSONL run artifacts, Go CLI tests, protocol templates, README and skill docs.

---

### Task 1: Freeze The New Goal-Boundary Vocabulary

**Files:**
- Create: `docs/plans/2026-03-24-goalx-light-goal-boundary-design.md`
- Modify: `cli/protocol_test.go`
- Modify: `cli/start_test.go`
- Modify: `README.md`
- Modify: `skill/SKILL.md`

**Step 1: Write the failing tests**

Add or update tests to assert:
- master protocol references `goal.json`, `goal-log.jsonl`, and simplified `acceptance.json`
- master protocol does not reference `goal-contract.json`
- non-trivial goals must compare at least 2-3 paths before committing
- master may switch paths autonomously without extra user authorization
- startup no longer requires `acceptance.md` as a protocol-critical artifact

**Step 2: Run the targeted tests to confirm they fail**

Run: `go test ./cli -run 'TestRenderMasterProtocol|TestRenderSubagentProtocol|TestStart' -count=1`

Expected: FAIL because templates and startup still pin the old model.

**Step 3: Lock the public vocabulary**

Update docs/tests so the intended new protocol terms are explicit:
- `goal.json`
- `goal-log.jsonl`
- simplified `acceptance.json`
- canonical `proof/completion.json`
- autonomous path compare/select/switch behavior

**Step 4: Re-run the targeted tests**

Run: `go test ./cli -run 'TestRenderMasterProtocol|TestRenderSubagentProtocol|TestStart' -count=1`

Expected: still FAIL until implementation lands, but test expectations now point at the new model.

**Step 5: Commit**

```bash
git add docs/plans/2026-03-24-goalx-light-goal-boundary-design.md cli/protocol_test.go cli/start_test.go README.md skill/SKILL.md
git commit -m "test: freeze goalx lightweight goal-boundary vocabulary"
```

### Task 2: Introduce Goal Boundary Storage And Paths

**Files:**
- Create: `cli/goal.go`
- Create: `cli/goal_log.go`
- Create: `cli/goal_test.go`
- Modify: `cli/storage_paths.go`
- Modify: `cli/start.go`
- Delete: `cli/goal_contract.go`
- Delete: `cli/goal_contract_test.go`

**Step 1: Write the failing tests**

Cover:
- `GoalPath(runDir)` returns `goal.json`
- `GoalLogPath(runDir)` returns `goal-log.jsonl`
- new runs initialize `goal.json`
- new runs initialize `goal-log.jsonl`
- `goal-contract.json` is no longer created on new runs

**Step 2: Run the targeted tests**

Run: `go test ./cli -run 'TestGoal|TestStart' -count=1`

Expected: FAIL because new goal-boundary storage does not exist.

**Step 3: Implement the new storage layer**

Add:
- `GoalState`
- `GoalItem`
- `GoalLogEntry`
- path helpers and loaders/savers

Keep the model small:
- required/optional items
- `open|claimed|waived`
- `evidence_paths`
- `note`
- `user_approved`

Do not add owner/execution/proof-quality fields.

**Step 4: Wire startup**

Change `start` so new runs initialize:
- `goal.json`
- `goal-log.jsonl`
- simplified `acceptance.json`
- `proof/completion.json` only when verify writes it

Do not create `goal-contract.json` on new runs.

**Step 5: Re-run the targeted tests**

Run: `go test ./cli -run 'TestGoal|TestStart' -count=1`

Expected: PASS.

**Step 6: Commit**

```bash
git add cli/goal.go cli/goal_log.go cli/goal_test.go cli/storage_paths.go cli/start.go
git rm cli/goal_contract.go cli/goal_contract_test.go
git commit -m "refactor: introduce lightweight goal boundary storage"
```

### Task 3: Simplify Acceptance State Without Losing Gate Safety

**Files:**
- Modify: `cli/acceptance.go`
- Modify: `cli/verify_test.go`
- Modify: `cli/start_test.go`
- Modify: `cli/protocol_test.go`

**Step 1: Write the failing tests**

Cover:
- `acceptance.json` uses `default_command`, `effective_command`, `change_kind`, `change_reason`, `goal_version`, `user_approved`, and `last_result`
- `narrowed` gate requires explicit user approval
- gate changes invalidate stale results when `goal_version` changes
- `acceptance.md` is optional and not required for verification logic

**Step 2: Run the targeted tests**

Run: `go test ./cli -run 'TestVerify|TestStart|TestRenderMasterProtocol' -count=1`

Expected: FAIL because current acceptance semantics are broader and tied to the old contract.

**Step 3: Implement the simplified gate model**

Remove or replace:
- `baseline_source`
- `command_source`
- `scope_type`
- `scope_reason`
- `contract_version`

Keep:
- default/effective command
- explicit gate-change reason
- goal version binding
- last verification result

**Step 4: Re-run the targeted tests**

Run: `go test ./cli -run 'TestVerify|TestStart|TestRenderMasterProtocol' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add cli/acceptance.go cli/verify_test.go cli/start_test.go cli/protocol_test.go
git commit -m "refactor: simplify goalx acceptance gate state"
```

### Task 4: Rewrite Verify, Proof, And Stop-Gate As One Atomic Slice

**Files:**
- Modify: `cli/verify.go`
- Modify: `cli/completion.go`
- Modify: `cli/proof.go`
- Modify: `cli/adapter.go`
- Modify: `cli/proof_test.go`
- Modify: `cli/verify_test.go`
- Modify: `cli/adapter_test.go`

**Step 1: Write the failing tests**

Cover:
- `verify` reads `goal.json`, not `goal-contract.json`
- required `open` items fail verification even if the eval command passes
- required `claimed` items require concrete evidence paths
- required `waived` items require explicit user approval
- `proof/completion.json` is generated with `goal_satisfied`, `required_total`, `required_satisfied`, `required_remaining`
- Claude master stop hook reads the new completion summary fields and blocks/permits stop correctly

**Step 2: Run the targeted tests**

Run: `go test ./cli -run 'TestVerify|TestProof|TestGenerateMasterAdapter' -count=1`

Expected: FAIL because verify and the stop-gate still depend on the old contract/proof fields.

**Step 3: Implement the atomic cutover**

In one batch:
- stop reading `goal-contract.json`
- make `verify` evaluate current `goal.json` + current `acceptance.json`
- generate canonical `proof/completion.json`
- make `GenerateMasterAdapter` gate on the new completion summary

Do not leave any intermediate state where verify and the stop-hook read different schemas.

**Step 4: Re-run the targeted tests**

Run: `go test ./cli -run 'TestVerify|TestProof|TestGenerateMasterAdapter' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add cli/verify.go cli/completion.go cli/proof.go cli/adapter.go cli/proof_test.go cli/verify_test.go cli/adapter_test.go
git commit -m "refactor: cut goalx verify and closeout to lightweight boundary model"
```

### Task 5: Cut Over Master And Subagent Protocols

**Files:**
- Modify: `templates/master.md.tmpl`
- Modify: `templates/program.md.tmpl`
- Modify: `cli/protocol.go`
- Modify: `cli/protocol_test.go`

**Step 1: Write the failing tests**

Cover:
- master protocol requires path comparison for non-trivial goals
- master protocol explicitly allows autonomous path switching
- master protocol references `goal.json` and `goal-log.jsonl`
- master protocol no longer teaches `goal-contract.json`
- subagent protocol tells sessions to surface better paths without self-closing scope
- `acceptance.md` is optional notes, not required protocol maintenance

**Step 2: Run the targeted tests**

Run: `go test ./cli -run 'TestRenderMasterProtocol|TestRenderSubagentProtocol' -count=1`

Expected: FAIL because the rendered protocol still teaches the old model.

**Step 3: Update the templates and render wiring**

Make the master protocol say:
- compare paths first on non-trivial goals
- switch paths autonomously when evidence warrants it
- keep `goal.json` small
- treat `goal-log.jsonl` as audit-only
- use `goalx verify` as the only closeout writer

Make the subagent protocol say:
- report superior paths to the master in actionable form
- do not rely on old goal-contract or checklist semantics

**Step 4: Re-run the targeted tests**

Run: `go test ./cli -run 'TestRenderMasterProtocol|TestRenderSubagentProtocol' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add templates/master.md.tmpl templates/program.md.tmpl cli/protocol.go cli/protocol_test.go
git commit -m "refactor: make goalx master autonomy default in lightweight boundary protocol"
```

### Task 6: Rewire Runtime, Status, Save, And Read Models

**Files:**
- Modify: `cli/runtime_state.go`
- Modify: `cli/status.go`
- Modify: `cli/save.go`
- Modify: `cli/read_only_test.go`
- Modify: `cli/save_test.go`
- Modify: `cli/verify_test.go`

**Step 1: Write the failing tests**

Cover:
- runtime state derives goal summary from `goal.json` + `proof/completion.json`
- project `status.json` no longer writes `goal_contract_status`
- `save` exports `goal.json`, `goal-log.jsonl`, simplified `acceptance.json`, and new `proof/completion.json`
- `save` does not export `goal-contract.json`
- read-only paths do not backfill deleted legacy files

**Step 2: Run the targeted tests**

Run: `go test ./cli -run 'TestSave|TestReadOnly|TestVerify|TestStatus' -count=1`

Expected: FAIL because runtime and save still expose the old vocabulary.

**Step 3: Implement the read-model cutover**

Update:
- runtime summary fields
- project `status.json`
- saved artifact layout
- any save/read-only expectations tied to `goal-contract.json`

Keep derived summaries useful, but do not make them protocol truth.

**Step 4: Re-run the targeted tests**

Run: `go test ./cli -run 'TestSave|TestReadOnly|TestVerify|TestStatus' -count=1`

Expected: PASS.

**Step 5: Commit**

```bash
git add cli/runtime_state.go cli/status.go cli/save.go cli/read_only_test.go cli/save_test.go cli/verify_test.go
git commit -m "refactor: rewire goalx runtime and saved artifacts to lightweight boundary model"
```

### Task 7: Update README, Skill, And Operator-Facing Wording

**Files:**
- Modify: `README.md`
- Modify: `skill/SKILL.md`
- Modify: `deploy/README.md`
- Modify: `skill/openclaw-skill/SKILL.md`

**Step 1: Write the failing checks**

Use `rg` assertions or doc tests for:
- no `goal-contract.json` in operator docs
- no wording that implies the master needs extra user authorization to explore better paths
- docs describe `goal.json`, `goal-log.jsonl`, simplified `acceptance.json`, and canonical completion proof

**Step 2: Run the checks**

Run: `rg -n 'goal-contract\\.json|extra authorization|ask the user to choose among paths by default' README.md deploy/README.md skill/SKILL.md skill/openclaw-skill/SKILL.md`

Expected: matches still exist.

**Step 3: Update the docs**

Make the docs say:
- GoalX defaults to master-led compare/select/switch behavior
- user objective stays anchored
- the master may expand or strengthen, but not silently narrow, the goal
- `goalx verify` validates the new lightweight boundary model

**Step 4: Re-run the checks**

Run: `rg -n 'goal-contract\\.json|extra authorization|ask the user to choose among paths by default' README.md deploy/README.md skill/SKILL.md skill/openclaw-skill/SKILL.md`

Expected: no matches for obsolete wording.

**Step 5: Commit**

```bash
git add README.md deploy/README.md skill/SKILL.md skill/openclaw-skill/SKILL.md
git commit -m "docs: update goalx operator guidance for lightweight goal boundary"
```

### Task 8: Delete Legacy Semantics And Run Full Regression

**Files:**
- Delete: any remaining `goal_contract` helpers/tests not already removed
- Modify: any failing call sites discovered by the full regression

**Step 1: Run the full regression suite**

Run: `go test ./... -count=1`

Expected: FAIL on any remaining legacy references.

**Step 2: Remove the leftovers**

Delete or replace:
- `goal_contract` symbols
- `goal_contract_status` status/cache fields
- render/tests/docs still pinned to old terms

Do not leave aliases or compatibility shims unless they are explicitly read-only and not part of the writable protocol.

**Step 3: Re-run the full regression suite**

Run: `go test ./... -count=1`

Expected: PASS.

**Step 4: Final audit sweep**

Run:
- `rg -n 'goal-contract|goal_contract|contract_version|scope_type|scope_reason|semantic_match|counter_evidence' cli templates README.md skill docs/plans`
- `rg -n 'goal\\.json|goal-log\\.jsonl|acceptance\\.json|proof/completion\\.json' cli templates README.md skill docs/plans`

Expected:
- first command only matches historical archived docs or explicit rejection tests
- second command matches the new active protocol surface

**Step 5: Commit**

```bash
git add -A
git commit -m "refactor: remove legacy goal contract semantics from goalx"
```

Plan complete and saved to `docs/plans/2026-03-24-goalx-light-goal-boundary-implementation-plan.md`. Two execution options:

1. Subagent-Driven (this session) - I dispatch fresh subagent per task, review between tasks, fast iteration
2. Parallel Session (separate) - Open new session with executing-plans, batch execution with checkpoints

Which approach?
