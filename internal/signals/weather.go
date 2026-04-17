package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WeatherSource 天气信号源，对接外部天气 API
type WeatherSource struct {
	endpointURL string
	apiKey      string
	client      *http.Client
}

// NewWeatherSource 创建天气信号源
func NewWeatherSource(endpointURL, apiKey string) *WeatherSource {
	return &WeatherSource{
		endpointURL: endpointURL,
		apiKey:      apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (w *WeatherSource) Type() string { return TypeWeather }

// Fetch 从天气 API 拉取数据，将响应归一化为 SignalData
func (w *WeatherSource) Fetch(ctx context.Context) (SignalData, error) {
	if w.endpointURL == "" {
		return w.mockData(), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.endpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造天气请求失败: %w", err)
	}
	if w.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.apiKey)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("天气 API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("天气 API 返回状态 %d", resp.StatusCode)
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析天气响应失败: %w", err)
	}
	return SignalData(raw), nil
}

// mockData 在没有配置端点时返回模拟数据（测试用）
func (w *WeatherSource) mockData() SignalData {
	return SignalData{
		"city":      "Tokyo",
		"condition": "clear",
		"intensity": 0,
		"temp":      22,
		"humidity":  60,
	}
}
