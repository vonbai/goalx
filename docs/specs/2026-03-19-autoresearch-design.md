# AutoResearch — Design Spec (v7)

> 自主研究框架。Master/Subagent 架构，AI 判断一切，框架只做编排。

## 1. Problem Statement

AI coding agent 能自主写代码、分析架构、产出报告。但缺少：
- 持久性：agent 会崩溃、卡住、自以为完成
- 并行性：同一问题多路探索
- 编排：worktree 隔离、tmux 管理、一键启动

### 灵感来源

| 来源 | 借鉴什么 |
|------|---------|
| [karpathy/autoresearch](https://github.com/karpathy/autoresearch) | protocol + journal + 无人值守 |
| [lidangzzz/goal-driven](https://github.com/lidangzzz/goal-driven) | master/subagent + criteria 验证 + 持续运行 |

### 与两者的区别

- Karpathy：优化模型参数，never stop，keep/revert。我们做软件研发，有明确终点。
- Goal-Driven：master 是 prompt，subagent 也是 prompt。我们加了框架编排（worktree、tmux、journal）和并行探索。

### 第一客户：QuantOS

驱动 AI agent 自主研发 QuantOS——架构调研、功能实现、代码重构。

## 2. Design Philosophy

| 原则 | 含义 |
|------|------|
| **框架做编排，agent 做判断** | Go 代码管 worktree/tmux/journal，AI 判断目标是否达成 |
| **协议即引擎** | master.md + program.md 是灵魂 |
| **有目标有终点** | master agent 判断目标达成即停 |
| **持久运行** | subagent 偏了 → 引导；崩溃/卡住 → 重启 |
| **并行探索** | 同一目标 N 个 subagent，master 统一监督 |
| **精巧不复杂** | ~700 行 Go + 声明式配置 + protocol 模板 |

### 框架 vs Agent 职责边界

| 框架做的（Go 代码） | Agent 做的（AI 判断） |
|---------------------|----------------------|
| 创建 git worktree | 理解目标 |
| 启动 tmux session/window | 判断目标是否达成 |
| 渲染 protocol 模板 | 决定引导/重启 subagent |
| 提供 journal 基础设施 | 评估代码/报告质量 |
| 创建 guidance 目录/文件 | 写引导内容、检查 ack |
| ar review/keep/drop 命令 | 给 subagent 反馈和方向 |

**框架不写 criteria 验证逻辑。** Objective 就是 criteria，master agent 自己理解和判断。

## 3. Architecture

```
┌──────────────────────────────────────────────────────────┐
│  ar — CLI                                                 │
│                                                           │
│  ar start → worktree × N → tmux (master + N subagent)    │
│  ar status / ar attach / ar stop                          │
│  ar review / ar diff / ar keep / ar archive / ar drop     │
└──────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────┐
│  Core: Config / Journal                                   │
└──────────────────────────────────────────────────────────┘
```

### 数据流

```
ar init "objective" [--research|--develop] [--parallel N] [--name NAME]
  ├─ 继承 project/user config
  ├─ 生成 ar.yaml（可 review/编辑）
  └─ 提示 "ar start"

ar start [或 ar start "objective" --research 合一模式]
  ├─ 如果有 CLI objective → 先执行 init 生成 ar.yaml
  ├─ 解析 ar.yaml
  ├─ 校验配置（objective / target / harness / engine/model / codex迁移提示 / sessions冲突）
  ├─ 检查 run name / tmux session / runDir 是否冲突；冲突则直接失败
  ├─ 创建 ~/.autoresearch/runs/{project-id}/{name}/ run 目录
  ├─ 复制 ar.yaml 到 run 目录（YAML snapshot）
  ├─ 创建 N 个 git worktree → ~/.autoresearch/runs/.../worktrees/
  ├─ 每个 worktree:
  │   ├─ 渲染 program.md（含 resume-first 段 + 绝对路径）
  │   ├─ 初始化 journal.jsonl + guidance.md
  │   ├─ 生成 adapter（claude: hooks.json + assume-unchanged）
  │   └─ Trust 引导（claude: .claude.json / codex: config.toml）
  ├─ 渲染 master.md（含 acceptance checklist 指令 + 绝对路径）
  ├─ 初始化 acceptance.md（空，master 首次 heartbeat 前填写）
  ├─ tmux session "ar-{project-id}-{run-name}":
  │   ├─ window "master": 启动 master agent
  │   ├─ window "{name}-1": 启动 subagent 1
  │   ├─ window "{name}-2": 启动 subagent 2
  │   └─ window "heartbeat": 定时 send-keys 循环
  └─ 输出状态

Heartbeat（tmux 管理的独立 window/process）:
  every check_interval:
    tmux send-keys → master window: "Heartbeat: execute check cycle now."
  master 收到 prompt → 执行一轮检查 → 回到空闲等待下一次

Master agent 检查循环（每次 heartbeat 触发）:
    1. subagent 活着吗？（tmux 检查）
    2. 读 journal + git log（了解进展）
    3. 检查 guidance ack（subagent 是否已读并执行引导？）
    4. 跑 harness（build + test 结果）
    5. AI 判断：目标达成了吗？
    6. 达成 → 写总结，停止
    7. 没达成 + 方向偏了 → 写 guidance（不重启）
    8. 没达成 + guidance 无效（2 轮未 ack/未改善）→ 重启
    9. 没达成 + agent 死了 → 重启
    10. budget 耗尽 → 写进展总结，停止

Subagent 工作:
  读 program.md → 朝目标工作 → 每步 commit + 记 journal
  → guidance 检查（三层保障）:
      1. protocol 指令: 每次 commit/harness 后 cat guidance 文件
      2. Claude Stop hook / 启动 prompt: 引擎层重复提醒
      3. ar start 生成的 adapter 文件配置上述机制
  → 有 guidance → 遵循 + journal ack + 清空文件
  → harness gate 必须过 → 继续 → 直到目标达成或被 master 停止

人类醒来:
  ar review → 对比 N 个 session → keep / archive / drop
```

## 4. 两种模式

| | research（调研） | develop（开发） |
|--|--|--|
| **subagent 输出** | 方案报告 | 可工作的代码 |
| **subagent 改代码？** | 不改，代码全 readonly | 改 |
| **master 判断什么** | 报告是否全面、有深度、可决策 | 代码是否满足 objective |
| **master 重启时** | "报告缺少 X 方面分析，补充" | "测试还没全过，继续修" |

**Research parallel 默认各自独立探索，不自动交叉传播。** 保留每个 session 的独立灵感。
想要观点交锋时，手动开一个新 research run，context 引用想对比的报告。

### 典型工作流

**直接 develop：**
```
ar start (mode: develop) → master 监督 subagent → 目标达成 → ar review → ar keep
```

**research → develop：**
```
ar start (mode: research, parallel: 3) → 3 份独立报告 → ar review → ar keep（保留胜出报告）
ar start (mode: develop, context 引用报告) → 实施 → ar review → ar keep（合并胜出分支）
```

**research → 观点交锋 → develop：**
```
ar start (mode: research, parallel: 3) → 3 份独立报告 → ar review → 发现分歧
ar start (mode: research, parallel: 2,
         context 引用 session-1 和 session-3 的报告,
         diversity_hints: "支持方案 A" / "支持方案 B") → 对比报告
ar start (mode: develop, context 引用共识) → 实施
```

## 5. Master / Subagent Protocol

### 5.1 Master Protocol (master.md)

Master agent 的行为完全由 protocol 定义，不是 Go 代码：

```
你是 master agent。你的唯一职责是确保目标达成。

目标：{objective}

你管理 {N} 个 subagent，在各自的 worktree 中工作。

首次 heartbeat 前：
  写 acceptance checklist 到 {acceptance_path}（3-7 条可验证标准）
  这是你对 objective 的显式解读，后续每轮都对照检查。

你会定期收到 "Heartbeat" 提示。每次收到时，先读 acceptance checklist，再执行以下检查：

1. 检查 subagent 存活：
   tmux list-panes -t {tmux_session}:{session} -F "#{pane_pid}"

2. 读 subagent 进展：
   cat {journal_path}
   cd {worktree} && git log --oneline -5

3. 跑验证：
   cd {worktree} && {harness_command}

4. 检查 guidance 状态：
   - 上一轮写过 guidance？检查 subagent journal 有没有 ack
   - 有 ack + 方向已调整 → 引导成功
   - 有 ack + 方向没改善 → 考虑升级干预
   - 无 ack（2+ 轮）→ subagent 可能没在工作，考虑重启

5. 用你的判断力评估：目标达成了吗？
   - 不要只看测试通过。理解 objective，检查代码是否真正满足。

6. 决策（三级递进）：
   - 达成 → 写总结到 {summary_path}，停止所有 subagent
   - 方向偏了但 agent 还活着 → 写 guidance 到 {guidance_path}
   - guidance 无效（写了但 2 轮未 ack 或未改善）→ 重启
   - agent 死了 → 重启并指出上次停在哪里
   - budget 耗尽 → 写进展总结，停止

你不要自己写代码。你只负责监督和评估。
```

### 5.2 Subagent Protocol (program.md)

```
你是 subagent。目标：{objective}

恢复 context（重启后不从头开始）：
1. 读 journal：cat {journal_path}
2. 读 guidance：cat {guidance_path}
3. 检查 worktree 状态：git status && git log --oneline -5
4. 从最后记录的状态继续

{mode-specific guidance}

工作方式：
1. 读代码理解架构
2. 朝目标推进，每步 commit
3. harness gate 必须过：{harness_command}
4. 每次 commit / harness 后检查 master 引导：
   cat {guidance_path}
   如果有内容 → 优先遵循 → journal 记 ack → 清空文件
5. 记 journal：{journal_path}
6. 目标达成就报告 done
```

### 5.3 Protocol 是灵魂

Go 代码只做：渲染模板 → 创建 worktree → 启动 tmux → 完事。

所有"智能"都在 master.md 和 program.md 里。agent 的行为由 protocol 决定，
不由 Go 代码决定。这就是"协议即引擎"。

## 6. N 路并行探索

```yaml
parallel: 3
diversity_hints:
  - "从性能角度"
  - "从架构简洁性角度"
  - "从可扩展性角度"
```

1 个 master + N 个 subagent。Master 同时监督所有 subagent。

每个 subagent：独立 worktree + branch + journal + tmux window。
各自的 program.md 包含不同的 diversity hint。

## 7. Config Format

ar.yaml 只写本次 run 独有的东西。engine/model/harness/preset 从上层继承。

```yaml
# ar.yaml — run 级配置（大部分字段可省略，继承上层）
name: event-sourcing            # run 名称（auto-slug from objective if omitted）
mode: research | develop
objective: "..."                # 目标 = criteria，master agent 理解并判断
preset: default                 # default | turbo | deep（可选，继承）

# 覆盖 preset 的 engine/model（可选）
# engine: codex
# model: fast

# 覆盖 master（可选）
# master:
#   model: sonnet

# 简单模式：N 个相同配置的 session
parallel: 2
diversity_hints:
  - "激进方案"
  - "保守方案"

# 高级模式：per-session engine/model（指定后 parallel + diversity_hints 忽略）
# sessions:
#   - hint: "Claude 分析"
#     engine: claude-code
#     model: opus
#   - hint: "Codex 编码"
#     engine: codex
#     model: codex

# target
target:
  files: [...]                  # research: 报告路径 / develop: 可改的代码
  readonly: [...]               # 可读不可改

# harness gate（可选，继承自 project config）
harness:
  command: "go build ./... && go test ./..."
  timeout: 5m

# context
context:
  files: [...]                  # 额外 context（如上一步的 research 报告）

# budget
budget:
  max_duration: 12h             # 最长运行时间
  max_rounds: 20                # subagent 最多工作轮次（可选）

# master
master:
  engine: claude-code           # master agent 使用的 AI engine（默认同 engine）
  check_interval: 5m            # 检查频率
```

### 示例：Research（简单，继承 project config）

```yaml
name: event-sourcing
mode: research
objective: |
  调研 quant_agent 是否应该引入 event sourcing：
  分析数据流、评估适用性、给出 2-3 方案、推荐最优。
parallel: 3
# engine/model/master 继承自 .autoresearch/config.yaml

target:
  files: ["report.md"]
  readonly: ["quant_agent/", "pkg/"]

harness:
  command: "test -s report.md && echo 'ok'"
  timeout: 30s

budget:
  max_duration: 6h

diversity_hints:
  - "从性能角度"
  - "从架构简洁性角度"
  - "从可扩展性角度"
```

### 示例：Develop（覆盖 model）

```yaml
name: refactor-quant
mode: develop
objective: |
  重构精简 quant_agent 架构：
  合并重叠包、消除过深抽象、删除死代码。
  包数量减少 30%+，全部测试通过。
model: haiku              # 重构任务用便宜模型
parallel: 2

target:
  files: ["quant_agent/"]
  readonly: ["pkg/", "cmd/"]

# harness 继承自 project config

budget:
  max_duration: 12h

diversity_hints:
  - "激进方案：大幅合并包"
  - "保守方案：最小改动"
```

### 示例：混合引擎 per-session

```yaml
name: arch-deep-dive
mode: research
objective: "深度分析 quant_agent 架构"

sessions:
  - hint: "Claude 深度分析"
    engine: claude-code
    model: opus
  - hint: "Codex 快速扫描"
    engine: codex
  - hint: "Aider 代码级走读"
    engine: aider

target:
  files: ["report.md"]
  readonly: ["."]
```

### 示例：Research → Develop 衔接

```yaml
name: impl-event-bus
mode: develop
objective: |
  按照 event-sourcing 调研报告的方案实施：
  实现轻量级 event bus。

context:
  files: ["~/.autoresearch/runs/{project-id}/event-sourcing/worktrees/es-1/report.md"]

target:
  files: ["quant_agent/"]
  readonly: ["pkg/", "cmd/"]

harness:
  command: "go build ./quant_agent/... && go test ./quant_agent/... -count=1"
  timeout: 8m

budget:
  max_duration: 8h

master:
  check_interval: 5m
```

## 8. Engine Adapters

所有引擎都是本地 CLI，统一通过 tmux 管理。
Engine 定义 command 模板，`{model_id}` 和 `{protocol}` 占位符在启动时替换。

**设计原则：Claude 做大脑，Codex 做双手。**
- Claude（opus/sonnet）：判断、监督、调研、灵感、方向
- Codex（gpt-5.3-codex）：写代码、review、执行

**所有引擎统一用交互模式。** Agent 启动后持续运行，维护 context。
框架不需要区分引擎——启动、检活、重启、引导的逻辑全部统一。

```yaml
engines:
  claude-code:
    # 交互模式，auto 权限（tmux 无人值守，比 dangerously-skip 更安全）
    command: "claude --model {model_id} --permission-mode auto"
    prompt: "Read {protocol} and follow it exactly."
    models:
      opus:   claude-opus-4-6
      sonnet: claude-sonnet-4-6
      haiku:  claude-haiku-4-5

  codex:
    # 交互模式，全自动（auto-approve + workspace-write sandbox）
    # 注意：gpt-5.3-codex 和 gpt-5.2 会触发 codex CLI 的交互式迁移提示，不可用
    command: "codex -m {model_id} --full-auto"
    prompt: "Read {protocol} and follow it exactly."
    models:
      codex:    gpt-5.4              # 旗舰 (Intelligence 57, #2)
      best:     gpt-5.4              # 同上
      balanced: gpt-5.4              # 统一用 5.4（5.3/5.2 有迁移提示 bug）
      fast:     gpt-5.4-mini         # 极速 (271 tok/s, Intelligence 48)

  droid:
    command: "droid exec --auto high -f"
    prompt: "{protocol}"

  aider:
    # 两步启动：先启动 aider，再 send-keys 发 /read
    command: "aider --model {model_id} --no-auto-commits --yes"
    prompt: "/read {protocol}"
```

**统一交互模式的好处：**
- Agent 工作完不退出 → master 可以 send-keys 追加指令
- Context 保留 → 不用重启从头来
- 框架代码统一 → 不需要按引擎类型分支

**两个 auto 标志的含义：**
- Claude: `--permission-mode auto` — 自动审批（比 --dangerously-skip-permissions 更安全）
- Codex: `--full-auto` — auto-approve + workspace-write sandbox

**Trust 自动引导：** `ar start` 在创建 worktree 后自动写入 trust 配置：
- Claude: 向 `~/.claude.json` 写入 `hasTrustDialogAccepted: true`
- Codex: 向 `~/.codex/config.toml` 写入 `trust_level = "trusted"`
避免首次启动时弹出交互式信任确认。

各引擎自行管理 "thinking effort"（不在框架层抽象）：
- Claude: model 选择 + `--effort high/max`（可选）
- Codex: `-c model_reasoning_effort="high"`（可选）

## 9. tmux Layout

```
ar start (name: event-sourcing, parallel: 2)
  → tmux new-session -d -s ar-data-dev-quantos-event-sourcing

  # master window (交互模式)
  → tmux rename-window -t ar-data-dev-quantos-event-sourcing:0 master
  → tmux send-keys "claude --model opus --dangerously-skip-permissions \
      'Read ~/.autoresearch/runs/{project-id}/event-sourcing/master.md and follow it.'" Enter

  # subagent windows (统一交互模式)
  → for i in 1..2:
       wt = ~/.autoresearch/runs/{project-id}/event-sourcing/worktrees/es-{i}
       git worktree add ${wt} -b ar/event-sourcing/{i}
       渲染 program-{i}.md → run 目录（含绝对路径）
       生成 adapter → worktree（claude: 合并 hooks.json + assume-unchanged）
       tmux new-window -t ar-data-dev-quantos-event-sourcing -n es-{i} -c ${wt}
       tmux send-keys "{engine command} '{prompt}'" Enter

  # heartbeat window（tmux 内常驻）
  → tmux new-window -d -t ar-data-dev-quantos-event-sourcing -n heartbeat \
      "while sleep ${check_interval}; do tmux send-keys -t ar-data-dev-quantos-event-sourcing:master 'Heartbeat: check now.' Enter; done"
```

`ar attach` 默认 attach 到 master window。
`ar attach event-sourcing-1` attach 到指定 subagent。

## 10. Commands

| 命令 | 作用 |
|------|------|
| `ar init "objective"` | 自动检测项目 → 生成 ar.yaml（可 `--research`/`--develop`/`--parallel N`/`--name`） |
| `ar start` | 读 ar.yaml → 创建 run 目录 + worktree → tmux → 启动 |
| `ar start "objective"` | init + start 一步完成（零配置快速启动） |
| `ar list` | 列出所有 run（active / completed / archived） |
| `ar status` | 当前 run 的各 session 进度 |
| `ar attach [name]` | tmux attach（默认 master） |
| `ar stop [name]` | 停止 |
| `ar review` | 对比所有 session |
| `ar diff <a> [b]` | 对比 session 代码/报告 |
| `ar keep <session>` | develop: ff-only merge 到当前分支（dirty tree 拒绝）；research: 标记保留 |
| `ar archive <session>` | git tag + 保留 |
| `ar drop` | kill tmux + 删 worktrees + 删 branches + 删 runDir |
| `ar report` | 从 journal 生成格式化报告 |

所有单 run 命令支持 `--run <name>`，默认取 `ar.yaml` 的 name。

**安全机制：**
- `ar keep` (develop): 检查项目 worktree 是否 dirty（允许 ar.yaml/.autoresearch 除外），只做 ff-only merge
- `ar start`: 校验 codex model 不会触发交互式迁移提示
- `ar start`: branch 已存在不自动删除（避免误删历史），报错退出
- `ar drop`: 完全清理（worktree + branch + runDir），name 可复用

## 11. Journal

JSONL，记录 subagent 进展。Master 读 journal 了解 subagent 状态。

```jsonl
{"round":1,"commit":"a1b2c3d","desc":"读完 react 包","status":"progress"}
{"round":2,"commit":"b2c3d4e","desc":"开始拆分 loop_run.go","status":"progress"}
{"round":3,"commit":"c3d4e5f","desc":"编译报错，修复 import","status":"fix"}
{"round":4,"commit":"d4e5f6g","desc":"全部 test pass","status":"done"}
```

Master 也有自己的 journal（master.jsonl），记录检查和决策：

```jsonl
{"ts":"...","action":"check","session":"session-1","finding":"12/23 tests pass, still working"}
{"ts":"...","action":"check","session":"session-2","finding":"agent inactive, restarting"}
{"ts":"...","action":"restart","session":"session-2","reason":"agent crashed"}
{"ts":"...","action":"done","session":"session-1","finding":"23/23 tests pass, objective met"}
```

## 12. 文件结构 — 用户级状态，项目零侵入

运行状态（journals、guidance、protocols、worktrees）全部放 `~/.autoresearch/`。
项目目录只有 `ar.yaml`（run 配置）和可选的 `.autoresearch/config.yaml`（项目默认值）。

### 用户级目录

```
~/.autoresearch/
├── config.yaml                              # 用户配置 + 引擎定义
└── runs/
    └── {project-id}/                        # 按项目隔离（路径 slug）
        └── {run-name}/
            ├── ar.yaml                      # config snapshot
            ├── master.md                    # master protocol（渲染后）
            ├── master.jsonl                 # master journal
            ├── acceptance.md                # master 写的验收标准（首次 heartbeat 前）
            ├── selection.json               # keep/archive 结果（可选）
            ├── program-1.md                 # subagent-1 protocol（含绝对路径）
            ├── program-2.md
            ├── journals/
            │   ├── session-1.jsonl          # subagent 写，master 读
            │   └── session-2.jsonl
            ├── guidance/
            │   ├── session-1.md             # master 写，subagent 读
            │   └── session-2.md
            ├── summary.md                   # master 最终输出
            └── worktrees/
                ├── {run-name}-1/            # git worktree, branch: ar/{name}/1
                └── {run-name}-2/
```

### 项目目录（干净）

```
{project}/
├── ar.yaml                          # 当前 run 配置（用户编辑）
├── .autoresearch/
│   └── config.yaml                  # 项目默认值（可选，可 commit）
└── ... 项目代码
```

### project-id

从项目绝对路径 slug 生成：`/data/dev/quantos` → `data-dev-quantos`。

```go
func projectID() string {
    root, _ := filepath.Abs(".")
    return strings.ReplaceAll(strings.TrimPrefix(root, "/"), "/", "-")
}
func runDir(name string) string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".autoresearch", "runs", projectID(), name)
}
```

### 为什么不放项目内

| | 项目内 `.autoresearch/` | 用户级 `~/.autoresearch/` |
|--|---|---|
| gitignore | 需要精细规则 | **不需要** |
| worktree 路径 | 相对路径不通 | **天然绝对路径** |
| 项目干净度 | journals + worktrees 在项目里 | **项目只有 ar.yaml** |
| git 操作影响 | rebase/switch 可能碰到 | **完全隔离** |

### 读写路径全部绝对

Protocol 模板渲染时写入绝对路径。Master 和 subagent 用完全相同的路径访问同一文件。

```
Master 写 guidance:  → ~/.autoresearch/runs/.../guidance/session-1.md
Subagent 读 guidance: → ~/.autoresearch/runs/.../guidance/session-1.md（同一个文件）

Subagent 写 journal: → ~/.autoresearch/runs/.../journals/session-1.jsonl
Master 读 journal:    → ~/.autoresearch/runs/.../journals/session-1.jsonl（同一个文件）
```

### Worktree 产出 vs 共享状态

| 数据 | 位置 | 谁写 | 谁读 |
|------|------|------|------|
| journal | `~/.autoresearch/runs/.../journals/` | subagent | master |
| guidance | `~/.autoresearch/runs/.../guidance/` | master | subagent |
| protocol | `~/.autoresearch/runs/.../program-{i}.md` | ar start | subagent（一次） |
| acceptance | `~/.autoresearch/runs/.../acceptance.md` | master（首次） | master（每轮对照） |
| master.jsonl | `~/.autoresearch/runs/.../` | master | master, ar status |
| summary | `~/.autoresearch/runs/.../` | master | 人类 |
| 代码改动 | worktree 内 | subagent | master (git log) |
| 报告 | worktree 内 (report.md) | subagent | master, 人类 |

### Engine Adapter（项目零侵入）

ar start 在 worktree 内生成 adapter 文件，但**不进 git commit**：

| Engine | 生成什么 | 处理方式 |
|--------|---------|---------|
| claude-code | `.claude/hooks.json` | 合并已有 hooks + 追加 Stop hook + `git update-index --assume-unchanged` |
| codex | 不改 AGENTS.md | 靠 protocol (program.md) 的 guidance 检查指令 |

项目的 AGENTS.md、CLAUDE.md、.claude/hooks.json 全部原样保留。

Git branch 命名：`ar/{run-name}/{session-number}`

可以同时有多个 active run。每个 run 使用独立 tmux session，名字由 `{project-id}` + `{run-name}` 派生。
同一项目下 run name 必须唯一；若 runDir 或 tmux session 已存在，`ar start` 直接失败。
历史 run 的 journals/summary 永久保留，供后续 run 做 context 引用。

## 13. 配置层级

四层配置，下层覆盖上层。每层只写需要覆盖的字段：

```
Built-in defaults → ~/.autoresearch/config.yaml → {project}/.autoresearch/config.yaml → ar.yaml → CLI flags
```

### 13.1 Presets（内置预设）

核心理念：**Claude 做大脑（判断/调研/灵感），Codex 做双手（编码/review）。**

依据：Arena 排名 Claude opus #1（判断），Aider benchmark GPT-5 系 88%（编码，Claude 72%）。

```
             Master               Research Subagent       Develop Subagent
             ──────────────────   ─────────────────────   ──────────────────────
 default     claude / opus        claude / sonnet         codex / gpt-5.4
             Arena #1 判断力       综合分析+灵感            Intelligence 57, #2

 turbo       claude / sonnet      claude / haiku          codex / gpt-5.4-mini
             够用                  快速扫描                271 tok/s 极速迭代

 deep        claude / opus        claude / opus           codex / gpt-5.4
             最强判断              最深分析                同 default develop model
```

**选择依据（Benchmark 数据，2026.03）：**

| 角色 | 选谁 | 数据支撑 |
|------|------|---------|
| Master | Claude opus | Arena #1 (ELO 1502)，判断/监督/灵感的 gold standard |
| Research sub | Claude sonnet/opus | Claude 在对话理解和综合分析上领先 Arena 全场 |
| Develop sub | Codex gpt-5.4 | Intelligence 57 (#2)，Aider 88% vs Claude 72% |
| Develop (turbo) | Codex gpt-5.4-mini | 271.7 tok/s (#1 速度)，Intelligence 48 仍很强 |

**注意：** gpt-5.3-codex 和 gpt-5.2 在 Codex CLI 中会触发交互式迁移提示，
无人值守场景不可用。所有 preset 统一使用 gpt-5.4 或 gpt-5.4-mini。

### 13.2 Built-in defaults（硬编码）

```yaml
preset: default
mode: develop
parallel: 1

master:
  engine: claude-code
  model: opus
  check_interval: 5m

# subagent 默认值（按 mode 选 engine）
research_defaults:
  engine: claude-code
  model: sonnet

develop_defaults:
  engine: codex
  model: codex              # → gpt-5.4 (Intelligence 57, all presets unified)

budget:
  max_duration: 8h
```

### 13.3 User config（`~/.autoresearch/config.yaml`）

引擎定义 + 个人偏好。配一次，所有项目生效。

```yaml
defaults:
  preset: default

# 引擎定义
engines:
  claude-code:
    command: "claude --model {model_id} --permission-mode auto"
    prompt: "Read {protocol} and follow it exactly."
    models:
      opus:   claude-opus-4-6
      sonnet: claude-sonnet-4-6
      haiku:  claude-haiku-4-5

  codex:
    command: "codex -m {model_id} --full-auto"
    prompt: "Read {protocol} and follow it exactly."
    models:
      codex:    gpt-5.4             # 旗舰（5.3/5.2 有迁移提示 bug）
      balanced: gpt-5.4
      fast:     gpt-5.4-mini
      best:     gpt-5.4

  droid:
    command: "droid exec --auto high -f"
    prompt: "{protocol}"

  aider:
    command: "aider --model {model_id} --no-auto-commits --yes"
    prompt: "/read {protocol}"
```

不在框架层抽象 "thinking"。各引擎自己管：
- Claude: model 选择 = thinking 深度
- Codex: 需要时加 `-c model_reasoning_effort="high"` 到 engine args

### 13.4 Project config（`.autoresearch/config.yaml`）

项目级默认值。配一次，所有 run 继承。

```yaml
harness:
  command: "go build ./quant_agent/... && go test ./quant_agent/... -count=1"
  timeout: 5m

target:
  readonly: ["pkg/", "cmd/"]
```

### 13.5 Run config（`ar.yaml`）

只写本次 run 独有的。

```yaml
name: event-sourcing
mode: research
objective: "..."
parallel: 3
# preset/engine/model 全部继承
```

或覆盖：

```yaml
name: refactor-react
mode: develop
objective: "..."
preset: deep               # 重要重构，用高质量配置
```

或 per-session：

```yaml
name: cross-review
mode: research
objective: "架构 review"
sessions:
  - hint: "Claude 架构审查"
    engine: claude-code
    model: opus
  - hint: "Codex 代码审查"
    engine: codex
    model: default
```

### 13.6 CLI flags（最高优先级）

```bash
ar start --preset turbo
ar start --preset deep --parallel 2
ar init "..." --research
```

### 13.7 启动前校验

`ar start` 在创建 run 目录、worktree、tmux 之前必须先校验配置：

- `objective` 非空
- run name 不与现有 runDir / tmux session 冲突
- `target.files` 合法且没有 develop placeholder
- `harness.command` 非空且没有 placeholder
- `engine` / `model` 能解析
- `sessions` 与 `parallel` / `diversity_hints` 不冲突
- `diversity_hints` 数量要么为空，要么与展开后的 session 数匹配

校验失败直接退出，不产生任何运行时副作用。

### 配置解析

```
config = merge(built_in, user_config, project_config, preset_expand, ar_yaml, cli_flags)
```

```
session.engine = session.engine ?? ar.yaml.engine ?? preset[mode].engine
session.model  = session.model  ?? ar.yaml.model  ?? preset[mode].model
master.engine  = master.engine  ?? preset.master.engine  ?? "claude-code"
master.model   = master.model   ?? preset.master.model   ?? "opus"
```

## 14. `ar init` — 脚手架

`ar init` 是纯脚手架。不做"智能检测"，只生成 ar.yaml 模板，继承上层配置。

### 用法

```bash
# 分步 — 生成模板，编辑，启动
ar init "调研 event sourcing 的可行性" --research --parallel 3
vim ar.yaml
ar start

# 合一 — 直接启动（快速探索，不需要精确配置）
ar start "调研 event sourcing" --research --parallel 3
```

### `ar init` 做什么

1. 解析 CLI 参数：objective, --research/--develop, --parallel, --name, --model, --engine
2. Name: 用户指定 `--name` 或从 objective slug 生成
3. 继承 project config（`.autoresearch/config.yaml`），再覆盖 CLI 参数
4. Research 模式：自动填 target（报告文件 + 全项目 readonly）+ 简单 harness
5. Develop 模式：target/harness 留 placeholder，让用户填
6. 生成 ar.yaml → 提示 `vim ar.yaml && ar start`

`ar start` 不接受这些 placeholder 直接启动；用户必须先填成真实配置。

### Research 模式默认值

```yaml
target:
  files: ["report.md"]              # worktree 内，git tracked
  readonly: ["."]
harness:
  command: "test -s report.md && echo 'ok'"
```

### Develop 模式占位

```yaml
target:
  files: ["TODO: specify directories to modify"]
  readonly: []
harness:
  command: "TODO: build + test command"
```

不猜 harness。项目自己知道该怎么 build/test，写在 project config 里一劳永逸。

### `ar init` 生成的 ar.yaml

```bash
ar init "调研 event sourcing" --research --parallel 3
```

```yaml
# ar.yaml — generated by ar init
# engine/model/master 继承自 ~/.autoresearch/config.yaml 和 .autoresearch/config.yaml
name: event-sourcing
mode: research
objective: |
  调研 event sourcing
parallel: 3

target:
  files: ["report.md"]             # worktree 内，git tracked
  readonly: ["."]

harness:
  command: "test -s report.md && echo 'ok'"
  timeout: 30s

budget:
  max_duration: 6h

# 取消注释并编辑：
# diversity_hints:
#   - "从性能角度"
#   - "从架构简洁性角度"
#   - "从可扩展性角度"

# Per-session 覆盖（高级用法，替代 parallel + diversity_hints）：
# sessions:
#   - hint: "深度分析"
#     model: opus
#   - hint: "快速扫描"
#     engine: codex
```

### Per-session 高级配置示例

```yaml
name: arch-explore
mode: research
objective: "分析 quant_agent 架构"

# 不用 parallel + diversity_hints，直接定义每个 session
sessions:
  - hint: "用 Claude 做深度分析"
    engine: claude-code
    model: opus
  - hint: "用 Codex 做快速扫描"
    engine: codex
  - hint: "用 Aider 做代码级分析"
    engine: aider
    model: claude-sonnet-4-6
```

每个 session 的 engine/model 独立解析。未指定的字段继承 ar.yaml → project config → user config → defaults。

### Research → Develop 衔接

```bash
# Step 1: Research
ar start "调研 event sourcing" --research --parallel 3
ar review && ar keep session-1   # 标记并保留胜出报告

# Step 2: Develop — 引用上一步的报告
ar init "实施 event sourcing" --develop \
  --context ~/.autoresearch/runs/{project-id}/event-sourcing/worktrees/es-1/report.md
vim ar.yaml    # 设置 target.files, harness
ar start
```

### `ar list`

```bash
$ ar list
NAME                MODE      ENGINE       STATUS      SESSIONS  CREATED
event-sourcing      research  claude-code  completed   3/3 done  2026-03-19
impl-event-bus      develop   claude-code  active      1/2 wip   2026-03-20
```

## 15. 代码量估算

| 模块 | 估计行数 |
|------|---------|
| **Core** (config.go, journal.go) | ~180 |
| **CLI** (init, start, stop, status, attach, list, review, keep, archive, drop, diff, report, protocol, harness, tmux, worktree) | ~580 |
| **CLI entry** (cmd/ar/main.go) | ~70 |
| **总 Go** | **~830** |
| **master.md.tmpl** | ~100 |
| **program.md.tmpl** | ~100 |
| **report.md.tmpl** | ~40 |
| **adapters** | ~80 |
| **总模板** | **~320** |

## 16. 未来扩展

- Level 2 SDK（进程内）
- Web Dashboard
- Webhook 通知
- Cross-session 知识共享
