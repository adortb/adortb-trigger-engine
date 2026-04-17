package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/adortb/adortb-trigger-engine/internal/store"
)

// Engine 规则引擎，协调评估和动作执行
type Engine struct {
	store     store.Store
	evaluator *Evaluator
	executor  ActionExecutor
}

// NewEngine 创建规则引擎
func NewEngine(s store.Store, executor ActionExecutor) *Engine {
	return &Engine{
		store:     s,
		evaluator: NewEvaluator(),
		executor:  executor,
	}
}

// EvaluateAndFire 对给定信号集合评估所有活跃规则，满足条件且未在冷却期则触发
func (e *Engine) EvaluateAndFire(ctx context.Context, signals map[string]any) error {
	rules, err := e.store.ListTriggerRules(ctx, "active")
	if err != nil {
		return fmt.Errorf("加载活跃规则失败: %w", err)
	}

	for _, rule := range rules {
		if err := e.evaluateRule(ctx, rule, signals); err != nil {
			log.Printf("[WARN] 规则 %d (%s) 评估失败: %v", rule.ID, rule.Name, err)
		}
	}
	return nil
}

// EvaluateRuleWithSignals 用模拟信号测试单条规则（不实际触发动作，不记录）
func (e *Engine) EvaluateRuleWithSignals(rule *store.TriggerRule, signals map[string]any) (bool, error) {
	return e.evaluator.Evaluate(rule.Conditions, signals)
}

func (e *Engine) evaluateRule(ctx context.Context, rule *store.TriggerRule, signals map[string]any) error {
	matched, err := e.evaluator.Evaluate(rule.Conditions, signals)
	if err != nil {
		return fmt.Errorf("条件评估失败: %w", err)
	}
	if !matched {
		return nil
	}

	// 检查冷却期
	if rule.LastFiredAt != nil {
		elapsed := time.Since(*rule.LastFiredAt)
		if elapsed < time.Duration(rule.CooldownSec)*time.Second {
			log.Printf("[INFO] 规则 %d 在冷却期，跳过（剩余 %.0fs）",
				rule.ID, (time.Duration(rule.CooldownSec)*time.Second - elapsed).Seconds())
			return nil
		}
	}

	// 执行所有动作
	var actions []Action
	if err := json.Unmarshal(rule.Actions, &actions); err != nil {
		return fmt.Errorf("解析动作列表失败: %w", err)
	}

	firedAt := time.Now()
	executedActions := make([]map[string]any, 0, len(actions))

	for _, action := range actions {
		if err := e.executor.Execute(ctx, action, signals); err != nil {
			log.Printf("[ERROR] 规则 %d 动作 %s 执行失败: %v", rule.ID, action.Type, err)
			executedActions = append(executedActions, map[string]any{
				"type":   action.Type,
				"status": "failed",
				"error":  err.Error(),
			})
		} else {
			executedActions = append(executedActions, map[string]any{
				"type":   action.Type,
				"status": "success",
			})
		}
	}

	// 持久化触发记录
	snapJSON, _ := json.Marshal(signals)
	actJSON, _ := json.Marshal(executedActions)
	firing := &store.TriggerFiring{
		RuleID:          rule.ID,
		SignalSnapshot:  json.RawMessage(snapJSON),
		ActionsExecuted: json.RawMessage(actJSON),
	}
	if err := e.store.SaveTriggerFiring(ctx, firing); err != nil {
		log.Printf("[ERROR] 保存触发记录失败: %v", err)
	}

	if err := e.store.RecordRuleFired(ctx, rule.ID, firedAt); err != nil {
		log.Printf("[ERROR] 更新规则触发时间失败: %v", err)
	}

	log.Printf("[INFO] 规则 %d (%s) 触发成功，执行了 %d 个动作", rule.ID, rule.Name, len(actions))
	return nil
}
