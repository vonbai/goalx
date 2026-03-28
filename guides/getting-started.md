# GoalX Getting Started

## 1. Install

```bash
git clone https://github.com/vonbai/goalx.git && cd goalx
make install
make skill-sync
```

Requirements:

- Go 1.24+
- `tmux`
- Claude Code or Codex CLI

## 2. Start With A Goal

Describe the destination, not the implementation checklist.

Good:

```bash
goalx run "the deploy works reliably in production"
goalx run "this repository has a high-quality architecture audit and an actionable fix plan"
goalx run "the authentication system is secure and verified"
```

Bad:

```bash
goalx run "1. profile the API 2. add caching 3. fix tests"
```

## 3. Watch The Run

```bash
goalx status
goalx observe
```

- `status` is the durable/control view.
- `observe` is the live transport view plus control summary.

## 4. Redirect When Needed

```bash
goalx tell "focus on the payment module first"
goalx tell --urgent "stop: production is down"
```

Use durable GoalX commands. Do not type instructions directly into tmux as the normal control path.

## 5. Get Results

```bash
goalx verify
goalx result
goalx keep
goalx save
```

## 6. Choose Intent Only When It Helps

```bash
goalx run "goal" --intent research
goalx run "goal" --intent develop
goalx run "goal" --intent evolve --budget 8h
```

- `research`: findings and reports
- `develop`: code and verification
- `evolve`: open-ended iterative improvement

## 7. Keep The Mental Model Straight

- GoalX framework = storage, execution, connectivity
- master = judgment, orchestration, closeout
- sessions = execution slices
- `goalx verify` = fact recording, not auto-completion
