package signals

import (
	"context"
	"sync"
)

// EventSource 事件信号源，通过 webhook 接收第三方推送
// 外部系统 POST 到 /v1/signals/ingest 后，数据存储在此处供规则引擎读取
type EventSource struct {
	mu      sync.RWMutex
	latest  SignalData
	eventCh chan SignalData
}

// NewEventSource 创建事件信号源
func NewEventSource() *EventSource {
	return &EventSource{
		eventCh: make(chan SignalData, 256),
		latest:  SignalData{},
	}
}

func (e *EventSource) Type() string { return TypeEvent }

// Fetch 返回最近一次接收到的事件数据
func (e *EventSource) Fetch(_ context.Context) (SignalData, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make(SignalData, len(e.latest))
	for k, v := range e.latest {
		result[k] = v
	}
	return result, nil
}

// Push 将外部 webhook 推送的数据注入信号源
func (e *EventSource) Push(data SignalData) {
	e.mu.Lock()
	e.latest = data
	e.mu.Unlock()

	select {
	case e.eventCh <- data:
	default:
		// 队列满时丢弃旧事件，保持最新
	}
}

// Events 返回事件通道，供实时触发使用
func (e *EventSource) Events() <-chan SignalData {
	return e.eventCh
}
