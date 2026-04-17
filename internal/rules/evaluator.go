package rules

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// ConditionNode 条件节点，支持递归的 AND/OR 逻辑及叶子条件
type ConditionNode struct {
	// 逻辑组合
	And []json.RawMessage `json:"and,omitempty"`
	Or  []json.RawMessage `json:"or,omitempty"`
	// 叶子条件
	Path  string          `json:"path,omitempty"`
	Op    string          `json:"op,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// Evaluator 条件评估器
type Evaluator struct{}

// NewEvaluator 创建评估器
func NewEvaluator() *Evaluator { return &Evaluator{} }

// Evaluate 对 signals 数据集评估条件表达式
// signals 结构示例:
//
//	{"weather":{"city":"Tokyo","condition":"rain","intensity":8},"time":{"hour":14}}
func (e *Evaluator) Evaluate(conditionsJSON json.RawMessage, signals map[string]any) (bool, error) {
	return evalNode(conditionsJSON, enrichWithTime(signals))
}

// enrichWithTime 注入当前时间字段到信号集合
func enrichWithTime(signals map[string]any) map[string]any {
	now := time.Now()
	enriched := make(map[string]any, len(signals)+1)
	for k, v := range signals {
		enriched[k] = v
	}
	enriched["time"] = map[string]any{
		"hour":    now.Hour(),
		"minute":  now.Minute(),
		"weekday": int(now.Weekday()), // 0=Sunday
		"day":     now.Day(),
		"month":   int(now.Month()),
	}
	return enriched
}

// evalNode 递归评估单个条件节点
func evalNode(raw json.RawMessage, signals map[string]any) (bool, error) {
	var node ConditionNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return false, fmt.Errorf("解析条件节点失败: %w", err)
	}

	switch {
	case len(node.And) > 0:
		for _, child := range node.And {
			ok, err := evalNode(child, signals)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil

	case len(node.Or) > 0:
		for _, child := range node.Or {
			ok, err := evalNode(child, signals)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil

	case node.Path != "":
		return evalLeaf(node, signals)

	default:
		return false, fmt.Errorf("无效条件节点: 既无逻辑组合也无叶子条件")
	}
}

// evalLeaf 评估叶子条件（单个 path/op/value）
func evalLeaf(node ConditionNode, signals map[string]any) (bool, error) {
	actual, err := resolvePath(node.Path, signals)
	if err != nil {
		return false, err
	}

	switch node.Op {
	case "eq", "=", "==":
		return compareEq(actual, node.Value)
	case "neq", "!=", "<>":
		ok, err := compareEq(actual, node.Value)
		return !ok, err
	case "gt", ">":
		return compareNum(actual, node.Value, func(a, b float64) bool { return a > b })
	case "gte", ">=":
		return compareNum(actual, node.Value, func(a, b float64) bool { return a >= b })
	case "lt", "<":
		return compareNum(actual, node.Value, func(a, b float64) bool { return a < b })
	case "lte", "<=":
		return compareNum(actual, node.Value, func(a, b float64) bool { return a <= b })
	case "in":
		return compareIn(actual, node.Value)
	case "not_in":
		ok, err := compareIn(actual, node.Value)
		return !ok, err
	case "between":
		return compareBetween(actual, node.Value)
	case "contains":
		return compareContains(actual, node.Value)
	case "exists":
		return actual != nil, nil
	default:
		return false, fmt.Errorf("不支持的操作符: %s", node.Op)
	}
}

// resolvePath 通过点分路径从 signals 中取值，如 "weather.condition"
func resolvePath(path string, signals map[string]any) (any, error) {
	parts := strings.SplitN(path, ".", 2)
	val, ok := signals[parts[0]]
	if !ok {
		return nil, nil // 路径不存在，非错误
	}
	if len(parts) == 1 {
		return val, nil
	}
	// 继续深入
	nested, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("路径 %s 中间节点不是对象", path)
	}
	return resolvePath(parts[1], nested)
}

// ─── 比较函数 ──────────────────────────────────────────────────────────────

func compareEq(actual any, raw json.RawMessage) (bool, error) {
	var expected any
	if err := json.Unmarshal(raw, &expected); err != nil {
		return false, fmt.Errorf("解析期望值失败: %w", err)
	}
	return reflect.DeepEqual(toComparable(actual), toComparable(expected)), nil
}

func compareNum(actual any, raw json.RawMessage, cmp func(a, b float64) bool) (bool, error) {
	a, err := toFloat64(actual)
	if err != nil {
		return false, fmt.Errorf("转换实际值为数字失败: %w", err)
	}
	var bv any
	if err := json.Unmarshal(raw, &bv); err != nil {
		return false, fmt.Errorf("解析期望值失败: %w", err)
	}
	b, err := toFloat64(bv)
	if err != nil {
		return false, fmt.Errorf("转换期望值为数字失败: %w", err)
	}
	return cmp(a, b), nil
}

func compareIn(actual any, raw json.RawMessage) (bool, error) {
	var list []any
	if err := json.Unmarshal(raw, &list); err != nil {
		return false, fmt.Errorf("解析 in 列表失败: %w", err)
	}
	av := toComparable(actual)
	for _, item := range list {
		if reflect.DeepEqual(av, toComparable(item)) {
			return true, nil
		}
	}
	return false, nil
}

func compareBetween(actual any, raw json.RawMessage) (bool, error) {
	var bounds []any
	if err := json.Unmarshal(raw, &bounds); err != nil {
		return false, fmt.Errorf("解析 between 范围失败: %w", err)
	}
	if len(bounds) != 2 {
		return false, fmt.Errorf("between 需要恰好 2 个边界值，实际 %d 个", len(bounds))
	}
	a, err := toFloat64(actual)
	if err != nil {
		return false, err
	}
	lo, err := toFloat64(bounds[0])
	if err != nil {
		return false, err
	}
	hi, err := toFloat64(bounds[1])
	if err != nil {
		return false, err
	}
	return a >= lo && a <= hi, nil
}

func compareContains(actual any, raw json.RawMessage) (bool, error) {
	var substr string
	if err := json.Unmarshal(raw, &substr); err != nil {
		return false, fmt.Errorf("解析 contains 值失败: %w", err)
	}
	str, ok := actual.(string)
	if !ok {
		return false, nil
	}
	return strings.Contains(str, substr), nil
}

// ─── 辅助函数 ──────────────────────────────────────────────────────────────

// toFloat64 统一将数值类型转换为 float64
func toFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, fmt.Errorf("字符串 %q 无法转换为数字", n)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("不支持的数值类型 %T", v)
	}
}

// toComparable 将 JSON 数字（json.Number / float64）统一化，确保 DeepEqual 正确
func toComparable(v any) any {
	switch n := v.(type) {
	case float64:
		// 若是整数值，转为 int64 以便与整数字段比较
		if n == float64(int64(n)) {
			return int64(n)
		}
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	default:
		return v
	}
}
