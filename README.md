# adortb-trigger-engine

条件触发广告引擎（第十四期）。根据天气、位置、事件等外部信号，自动触发广告活动的启停/出价调整。

## 功能概览

- JSON 树状条件表达式（支持 AND/OR 嵌套 + 10 种操作符）
- 4 种信号源：天气、位置（地理围栏）、Webhook 事件、自定义 HTTP
- 自动注入时间维度字段（`time.hour` / `time.weekday` 等）
- 冷却期机制（默认 3600s），防止频繁重复触发
- 4 种内置动作：激活/暂停活动、调整出价倍数、回调 Webhook
- 定时调度（默认 5 分钟轮询）
- 完整触发历史审计（信号快照 + 执行结果）

## 快速启动

```bash
export DATABASE_URL="postgres://user:pass@localhost/adortb_trigger"
export ADMIN_API_URL="http://adortb-admin:8080"
export PORT="8111"

go run ./cmd/trigger-engine
```

## API 端点

**信号源管理**
```
POST   /v1/sources                          # 创建信号源
GET    /v1/signals/:source_id/current       # 获取当前信号
POST   /v1/signals/ingest                  # Webhook 推送接入
```

**规则管理**
```
POST   /v1/rules                            # 创建规则
POST   /v1/rules/:id/activate              # 激活规则
POST   /v1/rules/:id/deactivate            # 停用规则
POST   /v1/rules/:id/test                  # 模拟测试（不实际执行）
```

**历史查询**
```
GET    /v1/firings?rule_id=X&from=T1&to=T2&limit=50
```

## 条件表达式示例

```json
{
  "and": [
    {"path": "weather.condition", "op": "in", "value": ["rain", "heavy_rain"]},
    {"path": "weather.intensity", "op": "gt", "value": 5},
    {"path": "time.hour", "op": "gte", "value": 8}
  ]
}
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DATABASE_URL` | `postgres://localhost/adortb_trigger` | PostgreSQL 连接串 |
| `ADMIN_API_URL` | `http://localhost:8080` | Admin API 地址 |
| `PORT` | `8111` | 监听端口 |

## 技术栈

- Go 1.25.3
- PostgreSQL（pgx v5）
- Prometheus
