# GoalX Deployment Guide

GoalX is local-first in this release. Install the binary, point it at a project, and run goals from the CLI.

## Install

```bash
go install github.com/vonbai/goalx/cmd/goalx@latest
# or build from source:
git clone https://github.com/vonbai/goalx.git && cd goalx
go build -o /usr/local/bin/goalx ./cmd/goalx
```

## Configure

```bash
mkdir -p ~/.goalx
cp deploy/config.example.yaml ~/.goalx/config.yaml
```

Edit the config to match your local defaults, engine presets, routing rules, and validation command.

## Run

```bash
cd /your/project
goalx run "objective"
goalx observe
goalx status
goalx verify --run your-run
goalx save --run your-run
```

`goalx status` and `goalx observe` surface `run_id`, `epoch`, and charter health alongside lease state. `goalx save` exports durable run metadata for later inspection.
