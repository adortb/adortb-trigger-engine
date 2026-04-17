package rules

import (
	"encoding/json"
	"testing"
)

func TestEvaluator_LeafConditions(t *testing.T) {
	e := NewEvaluator()

	signals := map[string]any{
		"weather": map[string]any{
			"city":      "Tokyo",
			"condition": "heavy_rain",
			"intensity": float64(8),
			"temp":      float64(15),
		},
		"stock": map[string]any{
			"ticker": "AAPL",
			"change": float64(5.2),
		},
	}

	tests := []struct {
		name       string
		conditions string
		want       bool
		wantErr    bool
	}{
		{
			name:       "eq 字符串匹配",
			conditions: `{"path":"weather.city","op":"eq","value":"Tokyo"}`,
			want:       true,
		},
		{
			name:       "eq 字符串不匹配",
			conditions: `{"path":"weather.city","op":"eq","value":"Osaka"}`,
			want:       false,
		},
		{
			name:       "neq 不等于",
			conditions: `{"path":"weather.city","op":"neq","value":"Osaka"}`,
			want:       true,
		},
		{
			name:       "gt 大于（满足）",
			conditions: `{"path":"weather.intensity","op":"gt","value":5}`,
			want:       true,
		},
		{
			name:       "gt 大于（不满足）",
			conditions: `{"path":"weather.intensity","op":"gt","value":10}`,
			want:       false,
		},
		{
			name:       "gte 大于等于（等值）",
			conditions: `{"path":"weather.intensity","op":"gte","value":8}`,
			want:       true,
		},
		{
			name:       "lt 小于",
			conditions: `{"path":"weather.temp","op":"lt","value":20}`,
			want:       true,
		},
		{
			name:       "lte 小于等于（等值）",
			conditions: `{"path":"weather.temp","op":"lte","value":15}`,
			want:       true,
		},
		{
			name:       "in 列表包含",
			conditions: `{"path":"weather.condition","op":"in","value":["rain","heavy_rain","storm"]}`,
			want:       true,
		},
		{
			name:       "in 列表不包含",
			conditions: `{"path":"weather.condition","op":"in","value":["clear","sunny"]}`,
			want:       false,
		},
		{
			name:       "not_in 不在列表",
			conditions: `{"path":"weather.condition","op":"not_in","value":["clear","sunny"]}`,
			want:       true,
		},
		{
			name:       "between 范围内",
			conditions: `{"path":"weather.intensity","op":"between","value":[5,10]}`,
			want:       true,
		},
		{
			name:       "between 范围外",
			conditions: `{"path":"weather.intensity","op":"between","value":[9,12]}`,
			want:       false,
		},
		{
			name:       "contains 字符串包含",
			conditions: `{"path":"weather.condition","op":"contains","value":"rain"}`,
			want:       true,
		},
		{
			name:       "exists 字段存在",
			conditions: `{"path":"weather.city","op":"exists","value":null}`,
			want:       true,
		},
		{
			name:       "路径不存在不报错",
			conditions: `{"path":"unknown.field","op":"eq","value":"x"}`,
			want:       false,
		},
		{
			name:       "不支持的操作符",
			conditions: `{"path":"weather.city","op":"regex","value":".*"}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(json.RawMessage(tt.conditions), signals)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_LogicCombination(t *testing.T) {
	e := NewEvaluator()

	signals := map[string]any{
		"weather": map[string]any{
			"condition": "heavy_rain",
			"intensity": float64(8),
			"temp":      float64(15),
		},
	}

	tests := []struct {
		name       string
		conditions string
		want       bool
	}{
		{
			name: "AND 全满足",
			conditions: `{
				"and": [
					{"path":"weather.condition","op":"in","value":["rain","heavy_rain"]},
					{"path":"weather.intensity","op":"gt","value":5}
				]
			}`,
			want: true,
		},
		{
			name: "AND 部分不满足",
			conditions: `{
				"and": [
					{"path":"weather.condition","op":"eq","value":"heavy_rain"},
					{"path":"weather.intensity","op":"gt","value":10}
				]
			}`,
			want: false,
		},
		{
			name: "OR 至少一个满足",
			conditions: `{
				"or": [
					{"path":"weather.condition","op":"eq","value":"sunny"},
					{"path":"weather.intensity","op":"gt","value":5}
				]
			}`,
			want: true,
		},
		{
			name: "OR 全不满足",
			conditions: `{
				"or": [
					{"path":"weather.condition","op":"eq","value":"sunny"},
					{"path":"weather.temp","op":"gt","value":30}
				]
			}`,
			want: false,
		},
		{
			name: "嵌套 AND/OR",
			conditions: `{
				"and": [
					{"path":"weather.condition","op":"in","value":["rain","heavy_rain"]},
					{
						"or": [
							{"path":"weather.intensity","op":"gt","value":7},
							{"path":"weather.temp","op":"lt","value":10}
						]
					}
				]
			}`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(json.RawMessage(tt.conditions), signals)
			if err != nil {
				t.Fatalf("Evaluate() 意外错误: %v", err)
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluator_TimeSignal(t *testing.T) {
	e := NewEvaluator()

	// time.hour 应由 enrichWithTime 自动注入
	signals := map[string]any{}

	// hour 应在 0~23 之间，between 0 and 23 应始终为 true
	cond := `{"path":"time.hour","op":"between","value":[0,23]}`
	got, err := e.Evaluate(json.RawMessage(cond), signals)
	if err != nil {
		t.Fatalf("评估时间条件失败: %v", err)
	}
	if !got {
		t.Error("time.hour between [0,23] 应始终为 true")
	}
}

func TestEvaluator_InvalidJSON(t *testing.T) {
	e := NewEvaluator()
	_, err := e.Evaluate(json.RawMessage(`{invalid json`), map[string]any{})
	if err == nil {
		t.Error("期望解析错误，但未报错")
	}
}

func TestResolvePath(t *testing.T) {
	signals := map[string]any{
		"weather": map[string]any{
			"city": "Tokyo",
		},
	}

	tests := []struct {
		path string
		want any
	}{
		{"weather", map[string]any{"city": "Tokyo"}},
		{"weather.city", "Tokyo"},
		{"unknown", nil},
		{"weather.unknown", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := resolvePath(tt.path, signals)
			if err != nil {
				t.Fatalf("resolvePath() error: %v", err)
			}
			if s, ok := tt.want.(string); ok {
				if got != s {
					t.Errorf("got %v, want %v", got, tt.want)
				}
			} else if tt.want == nil && got != nil {
				t.Errorf("got %v, want nil", got)
			}
		})
	}
}
