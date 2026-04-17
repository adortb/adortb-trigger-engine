package signals

import (
	"context"
	"testing"
	"time"
)

func TestEventSource_PushAndFetch(t *testing.T) {
	es := NewEventSource()

	// 初始状态
	data, err := es.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() 失败: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("初始数据应为空，实际: %v", data)
	}

	// 推送数据
	es.Push(SignalData{"condition": "rain", "city": "Tokyo"})

	data, err = es.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() 失败: %v", err)
	}
	if data["condition"] != "rain" {
		t.Errorf("condition = %v，期望 rain", data["condition"])
	}
	if data["city"] != "Tokyo" {
		t.Errorf("city = %v，期望 Tokyo", data["city"])
	}
}

func TestEventSource_EventChannel(t *testing.T) {
	es := NewEventSource()

	payload := SignalData{"event": "goal_scored", "team": "Japan"}
	es.Push(payload)

	select {
	case got := <-es.Events():
		if got["event"] != "goal_scored" {
			t.Errorf("event = %v，期望 goal_scored", got["event"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("超时：事件通道未收到数据")
	}
}

func TestEventSource_ConcurrentPush(t *testing.T) {
	es := NewEventSource()

	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func(n int) {
			es.Push(SignalData{"n": n})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}

	data, err := es.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() 失败: %v", err)
	}
	if data == nil {
		t.Error("并发推送后数据不应为 nil")
	}
}

func TestEventSource_ImmutableFetch(t *testing.T) {
	es := NewEventSource()
	es.Push(SignalData{"key": "original"})

	got, _ := es.Fetch(context.Background())
	got["key"] = "modified" // 修改副本

	got2, _ := es.Fetch(context.Background())
	if got2["key"] != "original" {
		t.Errorf("Fetch 应返回副本，原数据被篡改: %v", got2["key"])
	}
}
