// Package goalx provides an autonomous research and development framework.
//
// Master/Subagent architecture. Claude does brain (judgment, research, guidance),
// Codex does hands (coding, review). GoalX launches the master and exposes the
// worktree/tmux/journal tools it uses to orchestrate the run.
//
// # Design Philosophy
//
//   - GoalX provides tools, the master orchestrates: Go code manages worktree/tmux/journal, AI decides how to use them
//   - Protocol is the engine: master.md + program.md are the soul, not Go code
//   - Guide first, restart last: file-based guidance preserves subagent context
//   - Master-driven cadence: the master runs its own check cycle and updates status.json
//
// # Inspiration
//
//   - lidangzzz/goal-driven: master/subagent + criteria verification + persistence
//   - OpenClaw: remote control for long-running autonomous coding workflows
package goalx
