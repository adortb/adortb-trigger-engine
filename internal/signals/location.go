package signals

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LocationSource 位置/地理围栏信号源
type LocationSource struct {
	endpointURL string
	authToken   string
	client      *http.Client
}

// NewLocationSource 创建位置信号源
func NewLocationSource(endpointURL, authToken string) *LocationSource {
	return &LocationSource{
		endpointURL: endpointURL,
		authToken:   authToken,
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (l *LocationSource) Type() string { return TypeLocation }

// Fetch 拉取地理围栏触发事件
func (l *LocationSource) Fetch(ctx context.Context) (SignalData, error) {
	if l.endpointURL == "" {
		return SignalData{"active_geofences": []string{}, "user_count": 0}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.endpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造位置请求失败: %w", err)
	}
	if l.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+l.authToken)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("位置 API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("位置 API 返回状态 %d", resp.StatusCode)
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("解析位置响应失败: %w", err)
	}
	return SignalData(raw), nil
}
