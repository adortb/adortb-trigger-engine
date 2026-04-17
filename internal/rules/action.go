package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ActionType 动作类型
type ActionType string

const (
	ActionActivateCampaign   ActionType = "activate_campaign"
	ActionDeactivateCampaign ActionType = "deactivate_campaign"
	ActionBoostBid           ActionType = "boost_bid"
	ActionNotifyWebhook      ActionType = "notify_webhook"
)

// Action 规则触发后执行的动作
type Action struct {
	Type   ActionType     `json:"type"`
	Params map[string]any `json:"params,omitempty"`
}

// ActionExecutor 动作执行器接口
type ActionExecutor interface {
	Execute(ctx context.Context, action Action, signalData map[string]any) error
}

// HTTPActionExecutor 通过 HTTP 调用外部服务执行动作
type HTTPActionExecutor struct {
	adminBaseURL string
	client       *http.Client
}

// NewHTTPActionExecutor 创建 HTTP 动作执行器
func NewHTTPActionExecutor(adminBaseURL string) *HTTPActionExecutor {
	return &HTTPActionExecutor{
		adminBaseURL: adminBaseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Execute 根据动作类型分发执行
func (e *HTTPActionExecutor) Execute(ctx context.Context, action Action, signalData map[string]any) error {
	switch action.Type {
	case ActionActivateCampaign:
		return e.activateCampaign(ctx, action.Params)
	case ActionDeactivateCampaign:
		return e.deactivateCampaign(ctx, action.Params)
	case ActionBoostBid:
		return e.boostBid(ctx, action.Params)
	case ActionNotifyWebhook:
		return e.notifyWebhook(ctx, action.Params, signalData)
	default:
		return fmt.Errorf("未知动作类型: %s", action.Type)
	}
}

func (e *HTTPActionExecutor) activateCampaign(ctx context.Context, params map[string]any) error {
	campaignID, err := extractID(params, "campaign_id")
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/campaigns/%d/activate", e.adminBaseURL, campaignID)
	return e.doPost(ctx, url, nil)
}

func (e *HTTPActionExecutor) deactivateCampaign(ctx context.Context, params map[string]any) error {
	campaignID, err := extractID(params, "campaign_id")
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/campaigns/%d/deactivate", e.adminBaseURL, campaignID)
	return e.doPost(ctx, url, nil)
}

func (e *HTTPActionExecutor) boostBid(ctx context.Context, params map[string]any) error {
	campaignID, err := extractID(params, "campaign_id")
	if err != nil {
		return err
	}
	multiplier, ok := params["multiplier"]
	if !ok {
		return fmt.Errorf("boost_bid 缺少 multiplier 参数")
	}
	url := fmt.Sprintf("%s/api/v1/campaigns/%d", e.adminBaseURL, campaignID)
	body := map[string]any{"bid_cpm_multiplier": multiplier}
	return e.doPatch(ctx, url, body)
}

func (e *HTTPActionExecutor) notifyWebhook(ctx context.Context, params map[string]any, signalData map[string]any) error {
	webhookURL, ok := params["url"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("notify_webhook 缺少有效的 url 参数")
	}
	payload := map[string]any{
		"event":      "trigger_fired",
		"signal":     signalData,
		"details":    params,
		"fired_at":   time.Now().UTC().Format(time.RFC3339),
	}
	return e.doPost(ctx, webhookURL, payload)
}

func (e *HTTPActionExecutor) doPost(ctx context.Context, url string, body any) error {
	return e.doRequest(ctx, http.MethodPost, url, body)
}

func (e *HTTPActionExecutor) doPatch(ctx context.Context, url string, body any) error {
	return e.doRequest(ctx, http.MethodPatch, url, body)
}

func (e *HTTPActionExecutor) doRequest(ctx context.Context, method, url string, body any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return fmt.Errorf("序列化请求体失败: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &buf)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 %s 失败: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("请求 %s 返回错误状态 %d", url, resp.StatusCode)
	}
	return nil
}

func extractID(params map[string]any, key string) (int64, error) {
	v, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("缺少参数 %s", key)
	}
	switch n := v.(type) {
	case float64:
		return int64(n), nil
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("参数 %s 类型无效: %T", key, v)
	}
}
