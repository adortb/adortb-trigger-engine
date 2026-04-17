package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AdminClient adortb-admin API 客户端
type AdminClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewAdminClient 创建 admin API 客户端
func NewAdminClient(baseURL, apiKey string) *AdminClient {
	return &AdminClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ActivateCampaign 激活 campaign
func (c *AdminClient) ActivateCampaign(ctx context.Context, campaignID int64) error {
	url := fmt.Sprintf("%s/api/v1/campaigns/%d/activate", c.baseURL, campaignID)
	return c.doPost(ctx, url, nil)
}

// DeactivateCampaign 暂停 campaign
func (c *AdminClient) DeactivateCampaign(ctx context.Context, campaignID int64) error {
	url := fmt.Sprintf("%s/api/v1/campaigns/%d/deactivate", c.baseURL, campaignID)
	return c.doPost(ctx, url, nil)
}

// UpdateBidMultiplier 调整出价倍数
func (c *AdminClient) UpdateBidMultiplier(ctx context.Context, campaignID int64, multiplier float64) error {
	url := fmt.Sprintf("%s/api/v1/campaigns/%d", c.baseURL, campaignID)
	body := map[string]any{"bid_cpm_multiplier": multiplier}
	return c.doPatch(ctx, url, body)
}

func (c *AdminClient) doPost(ctx context.Context, url string, body any) error {
	return c.doRequest(ctx, http.MethodPost, url, body)
}

func (c *AdminClient) doPatch(ctx context.Context, url string, body any) error {
	return c.doRequest(ctx, http.MethodPatch, url, body)
}

func (c *AdminClient) doRequest(ctx context.Context, method, url string, body any) error {
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
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 %s 失败: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("admin API %s 返回 %d", url, resp.StatusCode)
	}
	return nil
}
