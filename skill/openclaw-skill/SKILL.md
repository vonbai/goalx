---
name: goalx-remote
description: Use when the user wants to manage GoalX research/develop runs on a remote dev server via HTTP API. Triggers when the user mentions goalx, research tasks, code investigation, starting/checking/stopping agent runs, managing project workspaces, or wants autonomous research on a remote machine. Even casual requests like "look into X on the dev server" or "start a research task" should trigger this skill.
---

# GoalX Remote — HTTP API Skill

Manage GoalX runs on a remote server via HTTP API. Browse projects, start research, check progress, change direction, manage configs — all through curl.

## Setup

The skill needs three environment variables. Set them in your OpenClaw agent config or export before use:

```bash
export GOALX_URL="http://YOUR_SERVER_IP:9800"
export GOALX_TOKEN="your-bearer-token"
```

Helper functions (define once per session):

```bash
gx_get()  { curl -fsS -H "Authorization: Bearer $GOALX_TOKEN" "$@"; }
gx_post() { curl -fsS -X POST -H "Authorization: Bearer $GOALX_TOKEN" -H "Content-Type: application/json" "$@"; }
gx_put()  { curl -fsS -X PUT -H "Authorization: Bearer $GOALX_TOKEN" -H "Content-Type: application/json" "$@"; }
gx_del()  { curl -fsS -X DELETE -H "Authorization: Bearer $GOALX_TOKEN" "$@"; }
pj()      { python3 -m json.tool 2>/dev/null; }
```

## What to Do When

### "Show me all projects" / "What's running?"
```bash
gx_get "$GOALX_URL/projects" | pj
gx_get "$GOALX_URL/runs" | pj
gx_get "$GOALX_URL/workspaces" | pj
```

### "Add this directory as a project"
```bash
gx_post "$GOALX_URL/workspaces" -d '{"name":"my-project","path":"/data/dev/my-project"}' | pj
```
Auto git-inits if the directory isn't a git repo.

### "Research X on project Y" / "Investigate..."
```bash
gx_post "$GOALX_URL/projects/Y/goalx/auto" \
  -d '{"objective":"X","mode":"research","parallel":2}' | pj
```

### "Fix X on project Y" / "Implement..."
```bash
gx_post "$GOALX_URL/projects/Y/goalx/init" \
  -d '{"objective":"X","mode":"develop","parallel":2}' | pj
# Then configure target files and harness:
gx_post "$GOALX_URL/projects/Y/goalx/config" \
  -d '{"target":{"files":["src/"]},"harness":{"command":"make test"}}' | pj
gx_post "$GOALX_URL/projects/Y/goalx/start" | pj
```

### "Use 2 opus + 1 codex" / Custom agent composition
```bash
# Configure mixed model sessions
gx_post "$GOALX_URL/projects/Y/goalx/config" \
  -d '{"sessions":[
    {"engine":"claude-code","model":"opus","hint":"deep exploration"},
    {"engine":"claude-code","model":"opus","hint":"creative alternatives"},
    {"engine":"codex","model":"gpt-5.4","hint":"audit and challenge"}
  ]}' | pj
gx_post "$GOALX_URL/projects/Y/goalx/start" | pj
```
Translate user requests like "use opus for thinking, codex for auditing" into the right session config.

### "How's the research going?" / "Check progress"
```bash
gx_get "$GOALX_URL/projects/Y/goalx/observe?run=NAME" | pj
```

### "Change direction" / "Focus on Z instead"
```bash
gx_post "$GOALX_URL/projects/Y/goalx/tell" \
  -d '{"message":"Focus on Z instead of current direction"}' | pj
```

### "Add another research angle"
```bash
gx_post "$GOALX_URL/projects/Y/goalx/add" \
  -d '{"direction":"Investigate from security perspective"}' | pj
# With specific engine/model:
gx_post "$GOALX_URL/projects/Y/goalx/add" \
  -d '{"direction":"Audit the implementation","engine":"codex","model":"gpt-5.4"}' | pj
```

### "Read or modify the config"
```bash
# Read (POST with empty body):
gx_post "$GOALX_URL/projects/Y/goalx/config" -d '{}' | pj
# Write (POST with content field):
gx_post "$GOALX_URL/projects/Y/goalx/config" \
  -d '{"content":"name: my-run\nobjective: ...\nmode: research\nparallel: 2"}' | pj
```

### "Save / Stop / Clean up"
```bash
gx_post "$GOALX_URL/projects/Y/goalx/save" | pj
gx_post "$GOALX_URL/projects/Y/goalx/stop" | pj
gx_post "$GOALX_URL/projects/Y/goalx/keep" -d '{"session":"session-1"}' | pj
gx_post "$GOALX_URL/projects/Y/goalx/drop" | pj
```

### "Remove a workspace"
```bash
gx_del "$GOALX_URL/workspaces/old-project" | pj
```

## Output Guidelines

After each action, summarize concisely:

```
Status: started / in progress / complete / stopped / failed
Key info: run name, current phase, active sessions
Next step: observe later / redirect / save / keep / drop
```

## Safety

- Always include Bearer token
- `save` before `drop` if results matter
- Use `tell` to redirect — don't `stop` a healthy run unless asked
- `observe` before making decisions about a run's direction
