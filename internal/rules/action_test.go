package rules

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPActionExecutor_ActivateCampaign(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST，实际 %s", r.Method)
		}
		if r.URL.Path != "/api/v1/campaigns/42/activate" {
			t.Errorf("路径错误: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exec := NewHTTPActionExecutor(srv.URL)
	action := Action{
		Type:   ActionActivateCampaign,
		Params: map[string]any{"campaign_id": float64(42)},
	}
	if err := exec.Execute(context.Background(), action, nil); err != nil {
		t.Fatalf("Execute() 失败: %v", err)
	}
	if !called {
		t.Error("mock server 未被调用")
	}
}

func TestHTTPActionExecutor_BoostBid(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("期望 PATCH，实际 %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exec := NewHTTPActionExecutor(srv.URL)
	action := Action{
		Type:   ActionBoostBid,
		Params: map[string]any{"campaign_id": float64(10), "multiplier": 1.5},
	}
	if err := exec.Execute(context.Background(), action, nil); err != nil {
		t.Fatalf("Execute() 失败: %v", err)
	}
	if body["bid_cpm_multiplier"] != 1.5 {
		t.Errorf("bid_cpm_multiplier = %v，期望 1.5", body["bid_cpm_multiplier"])
	}
}

func TestHTTPActionExecutor_NotifyWebhook(t *testing.T) {
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exec := NewHTTPActionExecutor("")
	action := Action{
		Type:   ActionNotifyWebhook,
		Params: map[string]any{"url": srv.URL},
	}
	signalData := map[string]any{"weather": map[string]any{"condition": "rain"}}
	if err := exec.Execute(context.Background(), action, signalData); err != nil {
		t.Fatalf("Execute() 失败: %v", err)
	}
	if payload["event"] != "trigger_fired" {
		t.Errorf("event = %v，期望 trigger_fired", payload["event"])
	}
}

func TestHTTPActionExecutor_UnknownAction(t *testing.T) {
	exec := NewHTTPActionExecutor("")
	action := Action{Type: "unknown_action"}
	if err := exec.Execute(context.Background(), action, nil); err == nil {
		t.Error("期望返回错误")
	}
}

func TestHTTPActionExecutor_MissingParams(t *testing.T) {
	exec := NewHTTPActionExecutor("http://localhost")

	tests := []struct {
		name   string
		action Action
	}{
		{
			name:   "activate 缺少 campaign_id",
			action: Action{Type: ActionActivateCampaign, Params: map[string]any{}},
		},
		{
			name:   "boost_bid 缺少 multiplier",
			action: Action{Type: ActionBoostBid, Params: map[string]any{"campaign_id": float64(1)}},
		},
		{
			name:   "notify_webhook 缺少 url",
			action: Action{Type: ActionNotifyWebhook, Params: map[string]any{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := exec.Execute(context.Background(), tt.action, nil); err == nil {
				t.Error("期望返回错误，但未报错")
			}
		})
	}
}
