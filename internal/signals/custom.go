package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CustomSource 自定义信号源，通用 HTTP 拉取
type CustomSource struct {
	endpointURL string
	headers     map[string]string
	client      *http.Client
}

// NewCustomSource 创建自定义信号源
func NewCustomSource(endpointURL string, headers map[string]string) *CustomSource {
	return &CustomSource{
		endpointURL: endpointURL,
		headers:     headers,
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CustomSource) Type() string { return TypeCustom }

// Fetch 拉取自定义端点数据
func (c *CustomSource) Fetch(ctx context.Context) (SignalData, error) {
	if c.endpointURL == "" {
		return SignalData{}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造自定义请求失败: %w", err)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("自定义 API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("自定义 API 返回状态 %d", resp.StatusCode)
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析自定义响应失败: %w", err)
	}
	return SignalData(raw), nil
}
