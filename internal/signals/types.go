package signals

import "context"

// SignalData 信号数据，key 为字段名，value 为字段值
type SignalData map[string]any

// Source 信号源接口
type Source interface {
	// Fetch 拉取最新信号数据
	Fetch(ctx context.Context) (SignalData, error)
	// Type 返回信号源类型标识
	Type() string
}

// 信号源类型常量
const (
	TypeWeather  = "weather"
	TypeLocation = "location"
	TypeEvent    = "event"
	TypeCustom   = "custom"
)
