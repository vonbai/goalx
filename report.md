# Shared Context for GoalX Subagents: Research Report

## Finding 1: Subagents are team-blind at startup — 1-5 minute gap before first orientation

- **Confidence**: HIGH
- **Evidence**:
  - `cli/start.go:236-253` populates `ProtocolData` for each subagent with **only individual fields**: Objective, Mode, Target, Harness, Context, Budget, SessionName, JournalPath, GuidancePath, WorktreePath, DiversityHint
  - **Missing from ProtocolData** (not populated for subagents): Sessions list, TmuxSession, ProjectRoot, AcceptancePath, SummaryPath, MasterJournalPath, StatusPath — all zeroed out (`cli/start.go:237-253` vs `cli/start.go:214-231` for master)
  - Master must complete 4-5 "Before First Heartbeat" steps (`master.md.tmpl:32-44`) before writing first guidance — including scope assessment, dimension/ownership planning, acceptance checklist creation
  - Default heartbeat interval: `2 * time.Minute` (`config.go:170`), minimum enforced: 30s (`cli/start.go` normalizeHeartbeatInterval)
  - Guidance files initialized as empty (`cli/start.go:175-177`: `os.WriteFile(guidancePath, nil, 0644)`)
  - Subagents only check guidance after commits (`program.md.tmpl:181`), creating additional latency
- **Counter-evidence**: Subagents DO have the objective and diversity_hint, so they can start meaningful work immediately on research tasks. The gap matters more for develop mode (where file ownership conflicts are destructive) than research mode (where redundant exploration is merely wasteful).
- **Implication**: Subagents spend 1-5 minutes working without team awareness. In develop mode, this can cause merge conflicts from overlapping file edits. In research mode, this causes redundant investigation of the same angles.

### What subagents HAS vs MISSING at startup

| HAS at startup | MISSING at startup |
|---|---|
| Objective | Team size (total sessions) |
| Description | Other sessions' existence |
| Mode (research/develop) | Other sessions' diversity hints |
| Own SessionName (e.g. "session-1") | File ownership assignments |
| Own JournalPath, GuidancePath, WorktreePath | Acceptance criteria |
| Target files + readonly files | Master agent's existence/path |
| Harness (gate command) | TmuxSession name |
| Context (external refs) | ProjectRoot |
| Budget (max rounds, duration) | Global progress state |
| DiversityHint (if configured) | |

---

## Finding 2: The master protocol already has full team context — the information exists but isn't shared

- **Confidence**: HIGH
- **Evidence**:
  - `cli/start.go:214-231` builds `masterData` with ALL fields populated, including `Sessions []SessionData` containing every subagent's Name, WindowName, WorktreePath, JournalPath, GuidancePath
  - `master.md.tmpl:19-23` renders a full session roster:
    ```
    {{range .Sessions}}
    - **{{.Name}}**: tmux `{{$.TmuxSession}}:{{.WindowName}}`, worktree `{{.WorktreePath}}`, journal `{{.JournalPath}}`, guidance `{{.GuidancePath}}`
    {{end}}
    ```
  - The master also gets AcceptancePath, SummaryPath, MasterJournalPath, StatusPath, ProjectRoot
  - Yet NONE of this is passed to `subData` in `cli/start.go:237-253` — the information is simply not forwarded
- **Counter-evidence**: The design is intentional — subagents are meant to be "guided workers" who receive direction via the guidance file. Giving them full team context adds protocol complexity and potentially increases distraction.
- **Implication**: The simplest implementation path is to populate additional fields in `ProtocolData` and render them in `program.md.tmpl`. The data already exists at render time — it's a wiring problem, not a data problem.

---

## Finding 3: Industry comparison — GoalX's proposed shared context would be ahead of all mainstream frameworks

- **Confidence**: HIGH
- **Evidence**: Surveyed 7 major multi-agent frameworks:

  | Framework | Team Context Mechanism | Agent-Readable? | Structured? |
  |-----------|----------------------|-----------------|-------------|
  | **CrewAI** | `Crew` object + delegation tools | No (orchestrator-mediated) | Partial |
  | **AutoGen** | System message free-text listing teammates | Yes (hand-written in prompt) | No |
  | **LangGraph** | `create_supervisor([agents])` | No (graph-level only) | No |
  | **MetaGPT** | `Team.hire(roles)` + pub-sub message pool | Partially (via message filtering) | Partial |
  | **Google ADK** | `sub_agents=[...]` list | No (parent-mediated) | No |
  | **OpenAI Agents SDK** | Transfer functions in tool list | Implicitly (via tool availability) | No |
  | **Claude Code Agent Teams** | `config.json` in `~/.claude/teams/` | **Yes** | **Yes** |

  - **Claude Code Agent Teams** is the closest analogue — uses a JSON file as a discoverable roster, plus shared task lists for self-coordination
  - **AutoGen** manually embeds team roster into system messages (exactly what GoalX's guidance mechanism does, but delayed)
  - **MetaGPT** uses pub-sub over a global message pool — agents discover peers by message type subscription, not explicit roster
  - No framework provides all four components (roster + ownership + acceptance + progress) as structured, agent-readable startup context

- **Counter-evidence**: Most frameworks avoid giving agents full team context, citing simplicity and reduced prompt bloat. In CrewAI and LangGraph, the orchestrator intentionally mediates all cross-agent awareness.
- **Implication**: The proposed `.goalx/shared/` design (roster.md + ownership.md + acceptance.md + progress.md) would make GoalX the most team-aware multi-agent framework. The question is whether the added context helps agents self-orient or just adds noise.

---

## Finding 4: Two feasible implementation paths — template injection vs. shared files

- **Confidence**: MEDIUM
- **Evidence**: Analysis of the codebase reveals two architectural options:

  ### Option A: Template Injection (add team context to program.md.tmpl)

  **Mechanism**: Populate `Sessions`, `AcceptancePath`, and other fields in `subData` (`start.go:237-253`), then render them in `program.md.tmpl`.

  **Pros**:
  - Minimal code change — just wire existing fields from `ProtocolData` into the subagent template
  - Information available at first token — zero startup latency
  - No new files to manage or coordinate
  - Works for `goalx add` (dynamic sessions) by re-rendering protocols

  **Cons**:
  - Protocol grows larger (~20-30 lines added → ~170-180 line rendered program)
  - Static snapshot — if master changes ownership mid-run, subagent's protocol is stale
  - Roster info is duplicated across N protocol files
  - `goalx add` problem: existing subagents' protocols don't update when a new session is added

  ### Option B: Shared Files (the proposed .goalx/shared/ design)

  **Mechanism**: Create structured files in the run directory that subagents read at startup and periodically.

  ```
  {runDir}/shared/
    roster.md        # Identity + team structure
    ownership.md     # File ownership map
    acceptance.md    # Master's acceptance criteria (already exists as {runDir}/acceptance.md)
    progress.md      # Global progress updates
  ```

  **Pros**:
  - Dynamic — master can update ownership/progress mid-run
  - Single source of truth — no duplication across protocols
  - Naturally extends current file-based IPC pattern (journals + guidance)
  - Scales to `goalx add` — new session appears in roster.md
  - Acceptance.md already exists at `{runDir}/acceptance.md` — just needs to be referenced in subagent protocol

  **Cons**:
  - Requires master to write these files (more "Before First Heartbeat" work, potentially worsening the gap)
  - Framework must generate roster.md at launch time (before master starts)
  - Read latency — subagent must `cat` files vs. having them in-protocol
  - More complex Resume logic in program.md.tmpl

  ### Option C: Hybrid (framework generates static, master updates dynamic)

  **Mechanism**: The Go framework (`cli/start.go`) generates `roster.md` at launch time with session info. Master writes `ownership.md`, `acceptance.md`, and `progress.md`. Protocol template tells subagents to read shared files at startup.

  **Pros**:
  - Roster available at T+0 (framework-generated, no master dependency)
  - Dynamic ownership/acceptance updated by master as today
  - Clean separation: framework handles identity, master handles assignments
  - `goalx add` updates roster.md programmatically

  **Cons**:
  - Two systems maintain shared state (framework + master)
  - Must handle consistency (what if master writes ownership.md BEFORE roster.md is read?)

- **Counter-evidence**: The current guidance-only approach works. Teams with diversity_hints already self-differentiate. The value of shared context may be marginal for research mode runs where agents explore independently.
- **Implication**: Option C (Hybrid) appears optimal — the framework generates roster.md at launch time (solving the cold-start identity problem), while master continues to write ownership/acceptance/progress through the existing guidance mechanism.

---

## Finding 5: Quantitative analysis of the program.md.tmpl overhead budget

- **Confidence**: HIGH
- **Evidence**:
  - Raw template: 215 lines, 8,084 bytes
  - Typical rendered program (research mode): ~120-150 lines, ~5,000-6,000 chars
  - Typical rendered program (develop mode): ~130-160 lines, ~5,500-6,500 chars
  - Adding a "Team Context" section would add ~20-30 lines:
    ```
    ## Team Context
    - You are session-1 of 3 total sessions
    - Sessions: session-1 (you), session-2, session-3
    - Acceptance criteria: read `{runDir}/acceptance.md`
    - Shared roster: read `{runDir}/shared/roster.md`
    ```
  - This represents a ~15-20% increase in rendered protocol size
  - For comparison, the "Resume First" section is 8 lines, "Context" section is 4-6 lines, "Journal" section is 6 lines
  - Industry data: AutoGen system messages with team rosters are typically 200-500 tokens; CrewAI agent backstories average 100-300 tokens
- **Counter-evidence**: Protocol size matters less than quality — an LLM reading 150 vs 180 lines shows no measurable behavior difference. The risk is more about distraction (subagent fixating on team coordination instead of doing its job).
- **Implication**: The protocol budget can easily accommodate a team context section. The design should be minimal and factual — "you are X of Y" — not prescriptive about cross-agent coordination.

---

## Finding 6: The `goalx add` dynamic session problem requires roster.md to be framework-managed

- **Confidence**: HIGH
- **Evidence**:
  - `cli/add.go:132-149` creates a new `ProtocolData` for the added session but does NOT update existing sessions' protocols
  - `cli/add.go:162-166` notifies master via tmux SendKeys but has no mechanism to notify existing subagents
  - If roster is embedded in protocol (Option A), existing subagents never learn about the new peer
  - If roster is a shared file (Option B/C), `cli/add.go` can append to roster.md and existing subagents will see it on next read
  - Current workaround: master writes guidance to existing sessions about the new peer, but this is manual and may be forgotten
- **Counter-evidence**: Dynamic session addition is relatively rare — most runs start with all sessions. The `goalx add` case may not justify the added complexity.
- **Implication**: A file-based roster (not template-embedded) is necessary for `goalx add` correctness. The framework should update `roster.md` in both `Start()` and `Add()`.

---

## Finding 7: acceptance.md already exists — the cold-start fix is primarily about roster and identity

- **Confidence**: HIGH
- **Evidence**:
  - `cli/start.go:203,263-264`: `acceptancePath := filepath.Join(runDir, "acceptance.md")` is created and initialized as empty
  - `master.md.tmpl:37,43`: Master writes acceptance criteria to this file as step 4 of "Before First Heartbeat"
  - BUT `program.md.tmpl` never references `acceptancePath` — subagents don't know it exists
  - Similarly, `summary.md` exists (`start.go:226`) but is master-only
  - The minimal fix is adding `AcceptancePath` to subagent `ProtocolData` and referencing it in the template
- **Counter-evidence**: Acceptance.md is empty at launch time (master hasn't written it yet). Pointing subagents to an empty file might confuse them. The protocol should say "check acceptance.md once available" or integrate it with the guidance-check loop.
- **Implication**: Of the four proposed shared files, acceptance.md is already 80% implemented — it just needs a reference in the subagent protocol. roster.md is the truly missing piece that requires new framework code.

---

## Finding 8: Codex subagents face a worse cold-start than Claude Code subagents

- **Confidence**: MEDIUM
- **Evidence**:
  - Claude Code subagents (`program.md.tmpl:149-156`) have Agent tool access — they can spawn parallel subagents to explore multiple angles simultaneously even without team context
  - Codex subagents (`program.md.tmpl:157-163`) lack subagent spawning capability and are told to check guidance file manually ("Re-check every few minutes")
  - Codex guidance delivery hook (`cli/adapter.go`) triggers on tool stops, not on file change — passive polling
  - Codex's `--full-auto` flag (per memory: `project_autoresearch_design.md`) gives full filesystem access but no inter-agent messaging
  - In hybrid preset runs (research=opus/claude-code, develop=gpt-5.4/codex), Codex develop sessions start modifying files without ownership info — the worst-case conflict scenario
- **Counter-evidence**: Codex sessions in develop mode receive the full target file list and harness command — they have enough to start working correctly on many objectives. The risk is primarily with multi-session develop runs where file boundaries matter.
- **Implication**: The cold-start fix disproportionately benefits Codex subagents in develop mode, which is arguably the highest-stakes scenario (conflicting file edits are destructive, not just wasteful).

---

## Next: Round 2 will investigate the precise implementation — what should roster.md contain, how should the protocol template reference it, and what code changes are needed in start.go and add.go.
