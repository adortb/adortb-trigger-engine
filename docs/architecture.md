# Architecture — adortb-trigger-engine

## 系统定位

事件驱动的广告条件触发引擎，基于外部信号（天气、位置、自定义 Webhook）评估 JSON 规则条件，满足时自动执行广告动作（启停活动、调整出价）。

## 整体架构图

```
外部信号源                  adortb-trigger-engine               adortb-admin
┌──────────┐              ┌────────────────────────────────┐    ┌───────────┐
│ Weather  │──HTTP GET───>│                                │    │           │
│   API    │              │  SourceRegistry                │    │  Activate │
├──────────┤              │  (并发 WaitGroup 拉取)          │    │  Bid Adj. │
│ Location │──HTTP GET───>│         │                      │    │           │
│   API    │              │         ▼                      │    └───────────┘
├──────────┤              │  SignalData (合并)              │         ▲
│ Webhook  │──POST Push──>│         │                      │         │
│ (Event)  │              │         ▼                      │         │
├──────────┤              │  RuleEngine.EvaluateAndFire()  │         │
│  Custom  │──HTTP GET───>│  ├── Evaluator（条件树递归）     │         │
│   API    │              │  ├── 冷却期检查                 │         │
└──────────┘              │  └── ActionExecutor ───────────┼─────────┘
                          │                                │
                          │  Scheduler（5min ticker）       │
                          │                                │
                          │  API Server（HTTP :PORT）       │
                          │  PostgreSQL（pgx v5）           │
                          └────────────────────────────────┘
```

## 核心模块

### 信号拉取（`watchers/`）

**调度器（`scheduler.go`）**：
- 5 分钟定时 ticker（可配置）
- 启动时立即执行一次（不等第一个 tick）
- 优雅停机：监听 `stopCh`

**注册表（`registry.go`）**：
```
FetchAllAndSave(ctx)
  │
  ▼ 并发拉取（WaitGroup）
  for each source in registry:
      goroutine → source.Fetch(ctx)
               → store.SaveSignalSnapshot()
               → combined[type] = data
  │
  ▼ 合并为 map[string]any（按 type 分组）
  │
  └──> 返回给 RuleEngine 评估
```

### 条件评估（`rules/evaluator.go`）

```
EvaluateNode(node, signalData) → bool

node.And → 递归 ALL(children)
node.Or  → 递归 ANY(children)
leaf node → resolvePath(node.Path, signalData) op node.Value

特殊路径：time.* → 自动注入（无需信号源）
```

**路径解析**：点分法，支持任意嵌套。类型自动转换（JSON number → float64 → int64 比较）。

### 冷却期控制（`rules/rule_engine.go`）

```
evaluateRule(ctx, rule, signals):
  1. 评估条件 → bool
  2. if !matched: return
  3. if LastFiredAt != nil AND elapsed < CooldownSec: return（跳过）
  4. 执行所有 actions
  5. store.RecordRuleFired() → last_fired_at + fire_count++
  6. store.SaveTriggerFiring() → 信号快照 + 执行结果入库
```

### 动作执行（`rules/action.go`）

```
HTTPActionExecutor.Execute(action, signalData)
  ├── activate_campaign   → POST {adminBaseURL}/api/v1/campaigns/{id}/activate
  ├── deactivate_campaign → POST {adminBaseURL}/api/v1/campaigns/{id}/deactivate
  ├── boost_bid           → PATCH {adminBaseURL}/api/v1/campaigns/{id}
  │                          body: {"bid_cpm_multiplier": X}
  └── notify_webhook      → POST {url}
                             body: {"event":"trigger_fired","signal":{},"fired_at":"..."}
```

所有 HTTP 调用超时 10s。动作独立执行，单个失败不中断其他。

## 数据库 Schema 概要

```sql
signal_sources   -- 信号源配置（type, endpoint_url, auth JSONB, polling_interval_sec）
trigger_rules    -- 规则定义（conditions JSONB, actions JSONB, cooldown_sec, last_fired_at）
trigger_firings  -- 触发历史（signal_snapshot JSONB, actions_executed JSONB）
signal_snapshots -- 信号快照（source_id, data JSONB, received_at）
```

**关键索引**：
- `(source_id, received_at DESC)` — 快速获取最新快照
- `(rule_id, fired_at DESC)` — 快速查询触发历史
- `(status)` — 规则和信号源按状态过滤

## 信号源扩展流程

```
1. internal/signals/yourtype.go
   实现 Source 接口：Fetch(ctx) + Type()

2. internal/watchers/registry.go
   buildSource() switch 添加 case

3. internal/store/models.go
   添加 TypeYour = "your_type" 常量

4. 添加 yourtype_test.go（必须）
```

## 并发与容错设计

| 问题 | 解决方案 |
|------|---------|
| 多信号源并发拉取 | WaitGroup + goroutine，失败仅跳过该源 |
| 多规则并发评估 | 串行（规则间独立，冷却期状态需串行保证） |
| 动作执行失败 | 独立记录，不影响同规则其他动作 |
| 信号源无响应 | 10s HTTP 超时，fallback 到上次快照 |

## API 数据流

```
POST /v1/signals/ingest           # Webhook 推送 → EventSource.Push()
                                       ↓
                                  内存缓存更新 → scheduler 下次拉取时使用

POST /v1/rules/:id/test           # 模拟评估（不更新 last_fired_at，不执行动作）
                                       ↓
                                  RuleEngine 内部评估 → 返回是否匹配

GET  /v1/firings?rule_id=X&...    # 查询触发历史
                                       ↓
                                  store.ListTriggerFirings() → 带信号快照
```

## 部署拓扑

```
┌─────────────────────────────────────────┐
│  adortb-trigger-engine                   │
│  ├── PostgreSQL（pgx v5）                │
│  ├── adortb-admin API（执行广告动作）      │
│  ├── 外部信号源（天气/位置 API）            │
│  └── Prometheus                          │
└─────────────────────────────────────────┘
```
