#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REAL_PATH="${PATH}"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/goalx-release-validation.XXXXXX")"
BIN_DIR="${TMP_ROOT}/bin"
EMPTY_BIN_DIR="${TMP_ROOT}/no-engine-bin"
HOME_DIR="${TMP_ROOT}/home"
PROJECT_DIR="${TMP_ROOT}/p"
TMUX_DIR="${TMP_ROOT}/tmux"
REAL_TMUX="$(command -v tmux)"
GO_BIN="$(command -v go)"
TMUX_LABEL="goalx-release-validation"
TMUX_LOG="${TMP_ROOT}/tmux.log"
SMOKE_LOG="${TMP_ROOT}/fake-codex.log"
GOALX_BIN="${GOALX_BIN:-${TMP_ROOT}/goalx}"
ACTIVE_RUNS=()

step() {
  printf '\n==> %s\n' "$1"
}

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

expect_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    printf 'ERROR: expected output to contain %q\n' "${needle}" >&2
    printf '%s\n' "${haystack}" >&2
    exit 1
  fi
}

assert_file() {
  local path="$1"
  [[ -f "${path}" ]] || fail "expected file to exist: ${path}"
}

assert_dir() {
  local path="$1"
  [[ -d "${path}" ]] || fail "expected directory to exist: ${path}"
}

run_dir_for() {
  if [[ ! -d "${HOME_DIR}/.goalx/runs" ]]; then
    return 0
  fi
  find "${HOME_DIR}/.goalx/runs" -type d -path "*/$1" 2>/dev/null | head -n 1
}

saved_dir_for() {
  if [[ ! -d "${HOME_DIR}/.goalx/runs" ]]; then
    return 0
  fi
  find "${HOME_DIR}/.goalx/runs" -type d -path "*/saved/$1" 2>/dev/null | head -n 1
}

wait_for_path() {
  local path="$1"
  local tries="${2:-30}"
  local i
  for ((i = 0; i < tries; i++)); do
    if [[ -e "${path}" ]]; then
      return 0
    fi
    sleep 1
  done
  fail "timed out waiting for ${path}"
}

cleanup() {
  local status=$?
  set +e
  export HOME="${HOME_DIR}"
  export PATH="${BIN_DIR}:${REAL_PATH}"
  export TMUX_TMPDIR="${TMUX_DIR}"
  unset TMUX
  for run_name in "${ACTIVE_RUNS[@]}"; do
    "${GOALX_BIN}" stop --run "${run_name}" >/dev/null 2>&1 || true
    "${GOALX_BIN}" drop --run "${run_name}" >/dev/null 2>&1 || true
  done
  tmux kill-server >/dev/null 2>&1 || true
  if [[ "${KEEP_TMP:-0}" == "1" ]]; then
    printf '\nkept validation workspace: %s\n' "${TMP_ROOT}"
  else
    rm -rf "${TMP_ROOT}"
  fi
  exit "${status}"
}

trap cleanup EXIT

seed_run_artifacts() {
  local run_name="$1"
  local summary_title="$2"
  local report_title="$3"
  local run_dir

  run_dir="$(run_dir_for "${run_name}")"
  [[ -n "${run_dir}" ]] || fail "unable to locate run dir for ${run_name}"

  mkdir -p "${run_dir}/reports"
  cat > "${run_dir}/summary.md" <<EOF
# ${summary_title}

- seeded by release validation
- verifies result/save/export plumbing without depending on real model output
EOF
  cat > "${run_dir}/reports/session-1-report.md" <<EOF
# ${report_title}

- seeded report artifact for ${run_name}
- used by save/result/phase-continuation smoke coverage
EOF
}

step "Build goalx binary"
mkdir -p "${BIN_DIR}" "${EMPTY_BIN_DIR}" "${HOME_DIR}" "${PROJECT_DIR}" "${TMUX_DIR}"
(cd "${ROOT_DIR}" && go build -o "${GOALX_BIN}" ./cmd/goalx)
assert_file "${GOALX_BIN}"

step "Install fake codex shim"
cat > "${BIN_DIR}/codex" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log_file="${GOALX_FAKE_CODEX_LOG:?}"
printf '%s argv=%s\n' "$(date -u +%FT%TZ)" "$*" >> "${log_file}"
prompt="${*: -1}"
printf 'fake-codex started\n'
printf 'prompt=%s\n' "${prompt}"
if [[ -f "${prompt}" ]]; then
  head -n 3 "${prompt}" | sed 's/^/prompt-head: /'
fi
trap 'printf "fake-codex stopping\n"; exit 0' TERM INT HUP
while :; do
  sleep 5
done
EOF
chmod +x "${BIN_DIR}/codex"
cat > "${BIN_DIR}/tmux" <<EOF
#!/usr/bin/env bash
set -euo pipefail
printf '%s argv=%s\n' "\$(date -u +%FT%TZ)" "\$*" >> "${TMUX_LOG}"
set +e
"${REAL_TMUX}" -L "${TMUX_LABEL}" "\$@"
status=\$?
set -e
printf '%s exit=%s argv=%s\n' "\$(date -u +%FT%TZ)" "\${status}" "\$*" >> "${TMUX_LOG}"
exit "\${status}"
EOF
chmod +x "${BIN_DIR}/tmux"

export HOME="${HOME_DIR}"
export PATH="${BIN_DIR}:${REAL_PATH}"
export TMUX_TMPDIR="${TMUX_DIR}"
export GOALX_FAKE_CODEX_LOG="${SMOKE_LOG}"
unset TMUX

step "Create temporary project"
cat > "${PROJECT_DIR}/go.mod" <<'EOF'
module example.com/goalxrelease

go 1.24
EOF
cat > "${PROJECT_DIR}/smoke.go" <<'EOF'
package goalxrelease

func Smoke() string {
	return "ok"
}
EOF
cat > "${PROJECT_DIR}/smoke_test.go" <<'EOF'
package goalxrelease

import "testing"

func TestSmoke(t *testing.T) {
	if Smoke() != "ok" {
		t.Fatalf("Smoke() returned %q", Smoke())
	}
}
EOF
cat > "${PROJECT_DIR}/README.md" <<'EOF'
# GoalX Release Smoke

Temporary project used to validate GoalX release flows.
EOF
mkdir -p "${PROJECT_DIR}/.goalx"
cat > "${PROJECT_DIR}/.goalx/config.yaml" <<'EOF'
preset: codex
target:
  files: ["README.md"]
local_validation:
  command: "__GO_BIN__ test ./..."
acceptance:
  command: "__GO_BIN__ test ./..."
master:
  check_interval: 2s
memory:
  llm_extract: off
EOF
sed -i "s|__GO_BIN__|${GO_BIN}|g" "${PROJECT_DIR}/.goalx/config.yaml"
git -C "${PROJECT_DIR}" init --initial-branch=main >/dev/null
git -C "${PROJECT_DIR}" config user.name "GoalX Smoke"
git -C "${PROJECT_DIR}" config user.email "goalx-smoke@example.com"
git -C "${PROJECT_DIR}" add .
git -C "${PROJECT_DIR}" commit -m "initial smoke project" >/dev/null

cd "${PROJECT_DIR}"

step "Verify no-engine fail-fast"
no_engine_home="${TMP_ROOT}/no-engine-home"
no_engine_project="${TMP_ROOT}/no-engine-project"
mkdir -p "${no_engine_home}"
mkdir -p "${no_engine_project}"
no_engine_output="$(cd "${no_engine_project}" && HOME="${no_engine_home}" PATH="${EMPTY_BIN_DIR}" TMUX_TMPDIR="${TMUX_DIR}" "${GOALX_BIN}" run "fail when no engine exists" 2>&1 || true)"
expect_contains "${no_engine_output}" "no supported engines found in PATH; install claude or codex"

step "Scenario 0: config-first init/start"
init_output="$("${GOALX_BIN}" init "validate the config-first launch path" --develop --name manual-start 2>&1)"
expect_contains "${init_output}" "Generated"
assert_file "${PROJECT_DIR}/.goalx/goalx.yaml"
start_output="$("${GOALX_BIN}" start --config .goalx/goalx.yaml 2>&1)"
expect_contains "${start_output}" "Run 'manual-start' started"
ACTIVE_RUNS+=("manual-start")
sleep 3
status_output="$("${GOALX_BIN}" status --run manual-start 2>&1)"
expect_contains "${status_output}" "manual-start"
"${GOALX_BIN}" stop --run manual-start >/dev/null
"${GOALX_BIN}" drop --run manual-start >/dev/null

step "Scenario A: direct deliver run + lifecycle control"
deliver_output="$("${GOALX_BIN}" run "ship the smoke project with durable local control" --name deliver-smoke 2>&1)"
expect_contains "${deliver_output}" "Run started."
ACTIVE_RUNS+=("deliver-smoke")
sleep 3
deliver_run_dir="$(run_dir_for deliver-smoke)"
[[ -n "${deliver_run_dir}" ]] || fail "unable to locate run dir for deliver-smoke"
context_output="$("${GOALX_BIN}" context --run deliver-smoke --json 2>&1)"
expect_contains "${context_output}" "deliver-smoke"
"${GOALX_BIN}" afford --run deliver-smoke --json >/dev/null
observe_output="$("${GOALX_BIN}" observe --run deliver-smoke 2>&1)"
expect_contains "${observe_output}" "fake-codex started"
"${GOALX_BIN}" tell --run deliver-smoke master "smoke validation ping" >/dev/null
"${GOALX_BIN}" add --run deliver-smoke --mode develop --worktree "prepare a mergeable smoke session" >/dev/null
sleep 3
deliver_status="$("${GOALX_BIN}" status --run deliver-smoke 2>&1)"
expect_contains "${deliver_status}" "session-1"
deliver_worker_dir="${deliver_run_dir}/worktrees/deliver-smoke-1"
assert_dir "${deliver_worker_dir}"
cat > "${deliver_worker_dir}/worker-note.txt" <<'EOF'
session-1 change kept into the run worktree
EOF
git -C "${deliver_worker_dir}" add worker-note.txt
git -C "${deliver_worker_dir}" commit -m "smoke worker change" >/dev/null
"${GOALX_BIN}" keep --run deliver-smoke session-1 >/dev/null
assert_file "${deliver_run_dir}/worktrees/root/worker-note.txt"
"${GOALX_BIN}" add --run deliver-smoke --mode research "exercise park and resume" >/dev/null
sleep 2
"${GOALX_BIN}" park --run deliver-smoke session-2 >/dev/null
"${GOALX_BIN}" resume --run deliver-smoke session-2 >/dev/null
"${GOALX_BIN}" dimension --run deliver-smoke session-2 --set depth,evidence >/dev/null
"${GOALX_BIN}" verify --run deliver-smoke >/dev/null
seed_run_artifacts "deliver-smoke" "Deliver Smoke Summary" "Deliver Smoke Report"
"${GOALX_BIN}" save deliver-smoke >/dev/null
deliver_save_dir="$(saved_dir_for deliver-smoke)"
[[ -n "${deliver_save_dir}" ]] || fail "unable to locate saved deliver run"
assert_file "${deliver_save_dir}/run-charter.json"
assert_file "${deliver_save_dir}/summary.md"
assert_file "${deliver_save_dir}/sessions/session-1/identity.json"
deliver_result="$("${GOALX_BIN}" result deliver-smoke 2>&1)"
expect_contains "${deliver_result}" "Deliver Smoke Summary"
"${GOALX_BIN}" stop --run deliver-smoke >/dev/null
"${GOALX_BIN}" drop --run deliver-smoke >/dev/null

step "Scenario B: research run + saved result surfaces"
research_output="$("${GOALX_BIN}" run "produce an evidence-backed audit for the smoke project" --intent research --name research-smoke 2>&1)"
expect_contains "${research_output}" "Run 'research-smoke' started"
ACTIVE_RUNS+=("research-smoke")
sleep 3
research_status="$("${GOALX_BIN}" status --run research-smoke 2>&1)"
expect_contains "${research_status}" "research-smoke"
research_observe="$("${GOALX_BIN}" observe --run research-smoke 2>&1)"
expect_contains "${research_observe}" "fake-codex started"
"${GOALX_BIN}" verify --run research-smoke >/dev/null
seed_run_artifacts "research-smoke" "Research Smoke Summary" "Research Smoke Report"
"${GOALX_BIN}" save research-smoke >/dev/null
research_save_dir="$(saved_dir_for research-smoke)"
[[ -n "${research_save_dir}" ]] || fail "unable to locate saved research run"
assert_file "${research_save_dir}/run-charter.json"
assert_file "${research_save_dir}/summary.md"
assert_file "${research_save_dir}/reports/session-1-report.md"
"${GOALX_BIN}" stop --run research-smoke >/dev/null
"${GOALX_BIN}" drop --run research-smoke >/dev/null
next_output="$("${GOALX_BIN}" next 2>&1)"
expect_contains "${next_output}" "goalx run --from research-smoke --intent debate"
research_result="$("${GOALX_BIN}" result research-smoke 2>&1)"
expect_contains "${research_result}" "Research Smoke Summary"

step "Scenario C1: debate phase"
debate_output="$("${GOALX_BIN}" run --from research-smoke --intent debate --name debate-smoke 2>&1)"
expect_contains "${debate_output}" "Run 'debate-smoke' started"
ACTIVE_RUNS+=("debate-smoke")
sleep 3
debate_status="$("${GOALX_BIN}" status --run debate-smoke 2>&1)"
expect_contains "${debate_status}" "debate-smoke"
"${GOALX_BIN}" observe --run debate-smoke >/dev/null
"${GOALX_BIN}" stop --run debate-smoke >/dev/null
"${GOALX_BIN}" drop --run debate-smoke >/dev/null

step "Scenario C2: implement phase"
implement_output="$("${GOALX_BIN}" run --from research-smoke --intent implement --name implement-smoke 2>&1)"
expect_contains "${implement_output}" "Run 'implement-smoke' started"
ACTIVE_RUNS+=("implement-smoke")
sleep 3
implement_status="$("${GOALX_BIN}" status --run implement-smoke 2>&1)"
expect_contains "${implement_status}" "implement-smoke"
"${GOALX_BIN}" verify --run implement-smoke >/dev/null
"${GOALX_BIN}" stop --run implement-smoke >/dev/null
"${GOALX_BIN}" drop --run implement-smoke >/dev/null

step "Scenario C3: explore phase"
explore_output="$("${GOALX_BIN}" run --from research-smoke --intent explore --name explore-smoke 2>&1)"
expect_contains "${explore_output}" "Run 'explore-smoke' started"
ACTIVE_RUNS+=("explore-smoke")
sleep 3
explore_status="$("${GOALX_BIN}" status --run explore-smoke 2>&1)"
expect_contains "${explore_status}" "explore-smoke"
"${GOALX_BIN}" observe --run explore-smoke >/dev/null
"${GOALX_BIN}" stop --run explore-smoke >/dev/null
"${GOALX_BIN}" drop --run explore-smoke >/dev/null

step "Release validation passed"
printf 'goalx release validation succeeded\n'
