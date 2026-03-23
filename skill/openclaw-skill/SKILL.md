---
name: goalx-remote
description: Use when the user wants to manage GoalX research/develop runs on a remote dev server via HTTP API. Triggers when the user mentions goalx, research tasks, code investigation, starting/checking/stopping agent runs, managing project workspaces, or wants autonomous research on a remote machine. Even casual requests like "look into X on the dev server" or "start a research task" should trigger this skill.
---

# GoalX Remote — HTTP API Skill

Manage GoalX runs on a remote server via HTTP API. Browse projects, start autonomous work, check progress, redirect runs, verify closeout, and manage configs through curl.

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
gx_post "$GOALX_URL/projects/Y/goalx/develop" \
  -d '{"objective":"X","parallel":2,"target_files":["src/"],"harness_command":"make test"}' | pj
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
gx_post "$GOALX_URL/projects/Y/goalx/observe" -d '{"run":"NAME"}' | pj
gx_post "$GOALX_URL/projects/Y/goalx/status" -d '{"run":"NAME"}' | pj
```

### "Change direction" / "Focus on Z instead"
```bash
gx_post "$GOALX_URL/projects/Y/goalx/tell" \
  -d '{"run":"NAME","message":"Focus on Z instead of current direction"}' | pj
```

### "Add another research angle"
```bash
gx_post "$GOALX_URL/projects/Y/goalx/add" \
  -d '{"run":"NAME","direction":"Investigate from security perspective"}' | pj
# With specific engine/model:
gx_post "$GOALX_URL/projects/Y/goalx/add" \
  -d '{"run":"NAME","direction":"Audit the implementation","engine":"codex","model":"gpt-5.4"}' | pj
```

### "Read or modify the config"
```bash
# Read project config:
gx_post "$GOALX_URL/projects/Y/goalx/config" -d '{}' | pj
# Read active run spec:
gx_post "$GOALX_URL/projects/Y/goalx/config" -d '{"run":"NAME"}' | pj
# Write shared project config:
gx_post "$GOALX_URL/projects/Y/goalx/config" \
  -d '{"content":"engines:\n  ..."}' | pj
```

### "Verify / Save / Stop / Clean up"
```bash
gx_post "$GOALX_URL/projects/Y/goalx/verify" -d '{"run":"NAME"}' | pj
gx_post "$GOALX_URL/projects/Y/goalx/save" -d '{"run":"NAME"}' | pj
gx_post "$GOALX_URL/projects/Y/goalx/stop" -d '{"run":"NAME"}' | pj
gx_post "$GOALX_URL/projects/Y/goalx/keep" -d '{"run":"NAME","session":"session-1"}' | pj
gx_post "$GOALX_URL/projects/Y/goalx/drop" -d '{"run":"NAME"}' | pj
```

### "Remove a workspace"
```bash
gx_del "$GOALX_URL/workspaces/old-project" | pj
```

## Output Guidelines

After each action, summarize concisely:

```
Status: started / in progress / complete / stopped / failed
Key info: run name, lifecycle status, lease health, active sessions
Next step: observe later / redirect / verify / save / keep / drop
```

## Safety

- Always include Bearer token
- `save` before `drop` if results matter
- Use `tell` to redirect — don't `stop` a healthy run unless asked
- Use `observe` and `status` before making decisions about a run's direction
- Run `verify` before calling a develop run complete
