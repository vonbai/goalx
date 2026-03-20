# AutoResearch — Implementation Plan (v6)

> Spec: `docs/specs/2026-03-19-autoresearch-design.md` (v6)
> Master/Subagent 架构。框架做编排，AI 判断一切。

## Phase 1: Core (~200 lines)

### 1.1 config (~170 lines)
- `config.go`:
  - Config, TargetConfig, ContextConfig, HarnessConfig, MasterConfig, SessionConfig
  - EngineConfig（command 模板 + model 映射）
  - **Preset 系统**：default / turbo / deep，展开为 engine + model（按 role + mode）
  - **四层配置合并**：built-in → user (~/.autoresearch/config.yaml) → project (.autoresearch/config.yaml) → run (ar.yaml)
  - `LoadConfig()` — 按层级加载、preset 展开、合并
  - `ValidateConfig()` — 启动前校验 objective / target / harness / engine / model / sessions
  - `ResolveEngine(engine, model)` — 查 engines 定义，替换 {model_id}，返回最终命令
  - Mode-aware defaults：research → claude, develop → codex
  - Name slug 生成（从 objective 提取关键词）
  - `sessions` 字段（per-session engine/model 覆盖）
  - `TmuxSessionName(projectID, runName)` — 生成 run 级唯一 tmux session 名
  - `ResolveRunName()` / 冲突检查（runDir 已存在、tmux session 已存在则失败）
- **不抽象 thinking** — 各引擎在 command 模板里自行处理
- **依赖**: `gopkg.in/yaml.v3`
- **测试**: preset 展开、配置合并优先级、engine 解析、mode-aware defaults、sessions 继承、placeholder 拒绝、session/diversity 校验、tmux session name 生成、run 名冲突拒绝

### 1.2 journal (~30 lines)
- `journal.go`: JournalEntry, LoadJournal, Summary
- 极简：框架只提供读 journal 给 ar status 用
- 写 journal 是 agent 的事（在 protocol 里指导）
- **测试**: 读 JSONL、Summary

**Phase 1 完成标志**: `go test ./...` 全绿。config 四层合并 + engine 解析 + 启动前校验正确。

---

## Phase 2: CLI 核心 (~420 lines)

### 2.1 CLI 框架 (~70 lines)
- `cmd/ar/main.go`: 子命令路由，标准库 flag
- 子命令：init, start, list, status, attach, stop, review, diff, keep, archive, drop, report

### 2.2 ar init (~50 lines)
- `cli/init.go`:
  1. 解析 CLI 参数：objective, --research/--develop, --parallel, --name, --model, --engine
  2. Name: 用户指定 → 或从 objective slug 生成
  3. 加载 project config 作为基础，CLI 参数覆盖
  4. Research 模式：自动填 target（报告 + 全项目 readonly）+ 简单 harness
  5. Develop 模式：target/harness 留 TODO placeholder
  6. 生成 ar.yaml → 打印下一步提示
- **不做自动检测**：harness 由用户在 project config 或 ar.yaml 中配置
- **测试**: research/develop 模式生成的 ar.yaml 内容、placeholder 会被 `ar start` 拒绝

### 2.3 tmux (~70 lines)
- `cli/tmux.go`: NewSession / NewWindow / RenameWindow / SendKeys / AttachSession / KillSession / KillWindow / SessionExists
- tmux session 按 run 唯一命名：`ar-{projectID}-{runName}`

### 2.4 worktree (~50 lines)
- `cli/worktree.go`: CreateWorktree / RemoveWorktree / MergeWorktree / TagArchive
- Branch 命名：`ar/{run-name}/{session-number}`

### 2.5 protocol 渲染 (~60 lines)
- `cli/protocol.go`:
  - `RenderMasterProtocol(config) (string, error)` — 渲染 master.md
  - `RenderSubagentProtocol(config, sessionIdx) (string, error)` — 渲染 program.md
  - 每个 session 用自己解析后的 engine command（含 model）
- **测试**: 两种模式 × master/subagent 渲染结果、per-session engine

### 2.6 harness 解析 (~20 lines)
- `cli/harness.go`: 变量替换（{worktree} 等）

### 2.7 adapter 生成 (~30 lines)
- `cli/adapter.go`: 按 engine 类型生成 worktree 内的适配文件
  - `claude-code` → 合并已有 `.claude/hooks.json` + 追加 Stop hook + `git update-index --assume-unchanged`
  - `codex` → 不改 AGENTS.md（靠 protocol 的 guidance 检查指令）
  - 项目的 AGENTS.md / CLAUDE.md / hooks 全部原样保留

### 2.8 heartbeat (~20 lines)
- `cli/heartbeat.go`: 生成 heartbeat shell loop 命令，在 tmux `heartbeat` window 中常驻
  - 每 check_interval: `tmux send-keys -t {tmuxSession}:master "Heartbeat: check now." Enter`
  - `ar start` 退出后 heartbeat 仍然存活
  - `ar stop` 通过 kill tmux session 一起退出

### 2.9 ar start (~100 lines)
- `cli/start.go`:
  1. 如果有 CLI objective 参数 → 先执行 init 逻辑
  2. 四层配置加载 + 合并
  3. `ValidateConfig()`，失败直接退出，不创建任何副作用
  4. 如果有 `sessions` → 用 sessions；否则从 parallel + diversity_hints 展开
  5. 每个 session 解析最终 engine/model → ResolveEngine() → 得到命令
  6. 计算 runDir: `~/.autoresearch/runs/{projectID}/{name}/`
  7. 计算 tmux session 名：`ar-{projectID}-{name}`
  8. 检查 runDir / tmux session 是否冲突，冲突直接失败
  9. 创建 run 目录 + 复制 config snapshot
  10. 创建 N 个 worktree → `{runDir}/worktrees/{name}-{i}/`
  11. 渲染 N 个 program-{i}.md + 1 个 master.md → run 目录（全部绝对路径）
  12. 每个 worktree 生成 engine adapter（合并 hooks + assume-unchanged）
  13. 初始化 journal × N + master.jsonl
  14. 初始化 `selection.json`（可选，keep/archive 时写入）
  15. 创建 guidance 目录 + 空 guidance 文件 × N
  16. tmux: master window + N session windows + heartbeat window（统一交互模式）
  17. 打印状态

**Phase 2 完成标志**: `ar init` 生成 ar.yaml + `ar start` 能启动 + heartbeat window 定时触发 master。

---

## Phase 3: 状态 + Review (~220 lines)

### 3.1 ar list (~30 lines)
- `cli/list.go`: 扫描 `~/.autoresearch/runs/{projectID}/*/ar.yaml`，展示当前项目的所有 run
- 字段：name, mode, engine, status, sessions, created, tmux-session

### 3.2 ar status (~40 lines)
- 读 subagent journal + master journal → 表格
- 支持 `--run <name>`，默认取当前项目 `ar.yaml` 指向的 run

### 3.3 ar attach (~15 lines)
- 默认 attach 当前 run 的 master，可指定 session
- 支持 `--run <name>`

### 3.4 ar stop (~20 lines)
- 按 run 定位 tmux session，kill 整个 session（含 heartbeat window）
- 支持 `--run <name>`

### 3.5 ar review (~50 lines)
- 多 session 对比
- 支持 `--run <name>`

### 3.6 ar diff (~15 lines)
- 支持 `--run <name>`

### 3.7 ar keep (~25 lines)
- `develop`: merge 选中 session branch 到当前分支
- `research`: 标记并保留选中 session/report，不 merge 到主分支
- 结果写入 `selection.json`
- 支持 `--run <name>`

### 3.8 ar archive (~15 lines)
- 支持 `--run <name>`

### 3.9 ar drop (~15 lines)
- 支持 `--run <name>`

### 3.10 ar report (~30 lines)
- 支持 `--run <name>`

**Phase 3 完成标志**: 完整 review 工作流 + `ar list` 展示历史 run。

---

## Phase 4: Templates (~320 lines)

### 4.1 master.md.tmpl (~100 lines)
- 监督循环指导
- check_interval
- run-scoped tmux session 名
- 存活检查命令
- journal + git log 读取（绝对路径）
- guidance 状态检查（ack 确认 / 升级判断）
- harness 执行
- AI 判断目标达成
- 三级递进决策：guide → restart（guidance 无效）→ restart（agent 死了）
- summary / guidance 绝对路径写入

### 4.2 program.md.tmpl (~100 lines)
- research: 只写报告不改代码
- develop: 朝目标写代码，harness gate 必须过
- guidance 检查：每次 commit/harness 后读 guidance 绝对路径
- guidance 响应：遵循 → journal ack → 清空文件
- diversity hint
- context files
- journal 绝对路径 + journal 格式
- budget 提示

### 4.3 report.md.tmpl (~40 lines)
- 多 session 对比

### 4.4 adapter 模板 (~80 lines)
- 各 engine 的 hooks / 启动 prompt 适配

**Phase 4 完成标志**: 模板可直接被 agent 执行。

---

## Phase 5: 实战验证

### 5.1 ar init 快速启动
- `ar init "测试目标" --research` → 验证生成的 ar.yaml
- `ar start "测试目标" --research` → 验证合一模式

### 5.2 配置层级
- 设置 project config → ar.yaml 只写 name/objective → 验证继承正确
- per-session engine 覆盖 → 验证每个 tmux window 用正确的命令
- 无效 placeholder / engine / sessions 配置 → 验证 `ar start` 在创建 run 前直接失败

### 5.3 单 subagent develop
- master + 1 subagent
- 验证：master 监督 → guidance → subagent 完成 → summary

### 5.4 并行 develop
- master + 2 subagent
- 验证：review → keep → drop

### 5.5 混合引擎
- session-1 用 claude-code，session-2 用 codex
- 验证各自启动命令正确

### 5.6 research → develop 衔接
- 完整两步工作流 + context 引用

### 5.7 多 run 并行
- 同项目或不同项目连续启动两个 run
- 验证 tmux session、heartbeat、attach/stop 互不串台

---

## 实施顺序

```
Phase 1 (Core)       ████░░░░░░░░  ~200 lines
Phase 2 (CLI 核心)   ░░░░█████░░░  ~420 lines
Phase 3 (Review)     ░░░░░░░░██░░  ~220 lines
Phase 4 (Templates)  ░░░░░░░░░██░  ~320 lines
Phase 5 (验证)       ░░░░░░░░░░░█  实战
```

总 Go: ~840 行。总模板: ~320 行。

## 依赖

```
go 1.25+
gopkg.in/yaml.v3
```
