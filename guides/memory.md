# GoalX Memory

GoalX keeps a local, file-backed long-term working memory store under `~/.goalx/memory/`.

## Model

GoalX memory is split into:

- canonical entries
- proposals
- derived indexes
- run-local compiled context

The master reads the compiled run-local context, not the entire canonical store.

## Core Rules

- memory is facts-first
- canonical memory is evidence-gated
- secret values are never persisted
- only secret references are allowed

## Main Surfaces

- canonical entries: `~/.goalx/memory/entries/`
- proposals: `~/.goalx/memory/proposals/`
- indexes: `~/.goalx/memory/index/`
- run-local query: `control/memory-query.json`
- run-local context: `control/memory-context.json`

## Why Proposal Gating Exists

GoalX does not immediately turn one run's lesson into canonical truth.

That avoids:

- polluted long-term memory
- one-run hallucinations becoming durable
- weak procedural guesses being treated as verified facts

## Retrieval Model

GoalX retrieval is selector-first:

- project
- environment
- service
- provider
- host
- intent

Only after that does it do lighter free-text ranking.

## Operator Guidance

- do not manually manage memory files in normal operation
- let GoalX compile run-local memory context automatically
- inspect memory files only when debugging or intentionally curating them
