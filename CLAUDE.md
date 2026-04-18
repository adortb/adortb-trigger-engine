# CLAUDE.md — adortb-trigger-engine

## 项目概述

条件触发广告引擎。语言 Go 1.25.3，监听端口 `:8111`（可配置）。

## 目录结构

```
cmd/trigger-engine/main.go
internal/
  rules/
    rule_engine.go    # 加载规则 → 评估条件 → 冷却检查 → 执行动作
    evaluator.go      # 递归评估 JSON 树条件（AND/OR/叶子节点）
    action.go         # HTTPActionExecutor（4 种内置动作）
  signals/
    types.go          # Source 接口 + SignalData 类型
    weather.go        # 天气信号源（HTTP GET + Bearer Token）
    location.go       # 位置/地理围栏信号源
    event.go          # Webhook 推送事件源（内存缓存 + channel）
    custom.go         # 通用 HTTP 自定义信号源
  watchers/
    registry.go       # 信号源注册表 + 并发拉取（WaitGroup）
    scheduler.go      # 定时调度（默认 5min 轮询）
  store/
    store.go          # 存储接口
    models.go         # SignalSource / TriggerRule / TriggerFiring / SignalSnapshot
    postgres.go       # PostgreSQL 实现（pgx v5）
  integration/
    admin_client.go   # 调用 adortb-admin API 执行广告操作
  api/
    handler.go        # HTTP 路由
    server.go         # 服务器初始化
  metrics/metrics.go
migrations/001_trigger.up.sql
```

## 条件语法

条件以 JSONB 存储，支持任意深度嵌套：

```json
{
  "and": [
    {"path": "weather.condition", "op": "in", "value": ["rain"]},
    {
      "or": [
        {"path": "weather.intensity", "op": "gt", "value": 7},
        {"path": "time.hour", "op": "between", "value": [8, 20]}
      ]
    }
  ]
}
```

**支持的操作符**：`eq` / `neq` / `gt` / `gte` / `lt` / `lte` / `in` / `not_in` / `between` / `contains` / `exists`

**自动注入时间字段**（无需外部信号）：`time.hour` / `time.minute` / `time.weekday` / `time.day` / `time.month`

**路径解析**：点分法访问嵌套字段，如 `weather.city`、`stock.price`

## 冷却期机制

```go
// rule_engine.go
if rule.LastFiredAt != nil {
    elapsed := time.Since(*rule.LastFiredAt)
    if elapsed < time.Duration(rule.CooldownSec)*time.Second {
        return nil  // 跳过，在冷却期内
    }
}
// 触发后更新 last_fired_at + fire_count++
```

- 默认冷却期：**3600 秒**（API 创建时未指定则用此默认值）
- 冷却期以规则为粒度，不同规则独立计算

## 信号源接入

| 类型 | 常量 | 数据获取方式 |
|------|------|------------|
| `weather` | `TypeWeather` | HTTP GET + API Key，无端点时返回模拟数据 |
| `location` | `TypeLocation` | HTTP GET + Bearer Token，地理围栏事件 |
| `event` | `TypeEvent` | 内存 + channel（Push 注入），Webhook 接收 |
| `custom` | `TypeCustom` | 通用 HTTP GET，支持自定义 Header |

新增信号源：实现 `signals.Source` 接口（`Fetch` + `Type`），在 `registry.go` `buildSource` 函数添加 case。

## 内置动作类型

| 动作 | 调用目标 |
|------|---------|
| `activate_campaign` | POST `{adminBaseURL}/api/v1/campaigns/{id}/activate` |
| `deactivate_campaign` | POST `{adminBaseURL}/api/v1/campaigns/{id}/deactivate` |
| `boost_bid` | PATCH `{adminBaseURL}/api/v1/campaigns/{id}`，body: `{bid_cpm_multiplier: X}` |
| `notify_webhook` | POST 自定义 URL，body 含信号快照和触发详情 |

新增动作：实现 `ActionExecutor` 接口并在 `action.go` 的 Execute switch 添加 case。

## 信号并发拉取

```go
// registry.go：所有信号源并发 Fetch，单个失败不影响其他
var wg sync.WaitGroup
for id, src := range sources {
    wg.Add(1)
    go func(sourceID int64, s signals.Source) {
        defer wg.Done()
        data, _ := s.Fetch(ctx)
        // 保存快照 + 按 type 合并
    }(id, src)
}
wg.Wait()
```

## 关键约定

- JSONB 字段：`conditions`（条件树）、`actions`（动作数组）、`auth`（信号源认证）
- `TriggerFiring` 记录触发时的完整信号快照，便于复盘
- `/v1/rules/:id/test` 接口用模拟信号评估，不实际执行也不更新 `last_fired_at`
- 调度器默认 5 分钟，启动时立即执行一次；支持 `Stop()` 优雅停机

## 数据库

四张核心表：`signal_sources` / `trigger_rules` / `trigger_firings` / `signal_snapshots`

```bash
psql $DATABASE_URL < migrations/001_trigger.up.sql
```
