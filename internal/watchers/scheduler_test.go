package watchers

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adortb/adortb-trigger-engine/internal/rules"
	"github.com/adortb/adortb-trigger-engine/internal/signals"
	"github.com/adortb/adortb-trigger-engine/internal/store"
)

// ─── mock store ────────────────────────────────────────────────────────────

type mockStore struct {
	mu          sync.Mutex
	ruleFired   int64
	snapshots   []*store.SignalSnapshot
	firings     []*store.TriggerFiring
	activeRules []*store.TriggerRule
}

func (m *mockStore) CreateSignalSource(_ context.Context, _ *store.SignalSource) error { return nil }
func (m *mockStore) GetSignalSource(_ context.Context, _ int64) (*store.SignalSource, error) {
	return nil, nil
}
func (m *mockStore) ListSignalSources(_ context.Context, _ string) ([]*store.SignalSource, error) {
	return nil, nil
}
func (m *mockStore) UpdateSignalSourceStatus(_ context.Context, _ int64, _ string) error {
	return nil
}
func (m *mockStore) CreateTriggerRule(_ context.Context, _ *store.TriggerRule) error { return nil }
func (m *mockStore) GetTriggerRule(_ context.Context, _ int64) (*store.TriggerRule, error) {
	return nil, nil
}
func (m *mockStore) ListTriggerRules(_ context.Context, _ string) ([]*store.TriggerRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeRules, nil
}
func (m *mockStore) UpdateTriggerRuleStatus(_ context.Context, _ int64, _ string) error {
	return nil
}
func (m *mockStore) RecordRuleFired(_ context.Context, _ int64, _ time.Time) error {
	atomic.AddInt64(&m.ruleFired, 1)
	return nil
}
func (m *mockStore) SaveSignalSnapshot(_ context.Context, snap *store.SignalSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots = append(m.snapshots, snap)
	return nil
}
func (m *mockStore) GetLatestSnapshot(_ context.Context, _ int64) (*store.SignalSnapshot, error) {
	return nil, nil
}
func (m *mockStore) GetLatestSnapshotByType(_ context.Context, _ string) (*store.SignalSnapshot, error) {
	return nil, nil
}
func (m *mockStore) SaveTriggerFiring(_ context.Context, f *store.TriggerFiring) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.firings = append(m.firings, f)
	return nil
}
func (m *mockStore) ListTriggerFirings(_ context.Context, _ store.ListFiringsFilter) ([]*store.TriggerFiring, error) {
	return nil, nil
}

// ─── mock executor ─────────────────────────────────────────────────────────

type mockExecutor struct {
	mu      sync.Mutex
	calls   []rules.Action
	failAll bool
}

func (e *mockExecutor) Execute(_ context.Context, action rules.Action, _ map[string]any) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, action)
	return nil
}

// ─── mock source ────────────────────────────────────────────────────────────

type mockSource struct {
	fetchCount int64
	data       signals.SignalData
	srcType    string
}

func (s *mockSource) Fetch(_ context.Context) (signals.SignalData, error) {
	atomic.AddInt64(&s.fetchCount, 1)
	return s.data, nil
}
func (s *mockSource) Type() string { return s.srcType }

// ─── 测试 ──────────────────────────────────────────────────────────────────

func TestScheduler_RunOnce_FetchesAndEvaluates(t *testing.T) {
	ms := &mockStore{}
	me := &mockExecutor{}

	// 设置活跃规则（条件：weather.condition == "rain"）
	cond := `{"path":"weather.condition","op":"eq","value":"rain"}`
	actions := `[{"type":"activate_campaign","params":{"campaign_id":1}}]`
	ms.activeRules = []*store.TriggerRule{
		{
			ID:          1,
			Name:        "雨天规则",
			Conditions:  json.RawMessage(cond),
			Actions:     json.RawMessage(actions),
			CooldownSec: 3600,
			Status:      "active",
		},
	}

	engine := rules.NewEngine(ms, me)
	registry := NewSourceRegistry(ms)

	// 注册模拟信号源
	src := &mockSource{
		srcType: signals.TypeWeather,
		data:    signals.SignalData{"condition": "rain", "intensity": float64(5)},
	}
	registry.Register(1, src)

	scheduler := NewScheduler(registry, engine, time.Minute)

	ctx := context.Background()
	if err := scheduler.RunOnce(ctx); err != nil {
		t.Fatalf("RunOnce() 失败: %v", err)
	}

	if atomic.LoadInt64(&src.fetchCount) != 1 {
		t.Errorf("信号源应被拉取 1 次，实际 %d", src.fetchCount)
	}

	if len(me.calls) != 1 {
		t.Errorf("期望执行 1 个动作，实际 %d", len(me.calls))
	}
	if me.calls[0].Type != rules.ActionActivateCampaign {
		t.Errorf("动作类型错误: %s", me.calls[0].Type)
	}
}

func TestScheduler_CooldownPreventsReFiring(t *testing.T) {
	ms := &mockStore{}
	me := &mockExecutor{}

	now := time.Now()
	cond := `{"path":"weather.condition","op":"eq","value":"rain"}`
	actions := `[{"type":"activate_campaign","params":{"campaign_id":1}}]`
	ms.activeRules = []*store.TriggerRule{
		{
			ID:          1,
			Name:        "冷却测试规则",
			Conditions:  json.RawMessage(cond),
			Actions:     json.RawMessage(actions),
			CooldownSec: 3600,
			LastFiredAt: &now, // 刚刚触发过
			Status:      "active",
		},
	}

	engine := rules.NewEngine(ms, me)
	registry := NewSourceRegistry(ms)
	registry.Register(1, &mockSource{
		srcType: signals.TypeWeather,
		data:    signals.SignalData{"condition": "rain"},
	})

	scheduler := NewScheduler(registry, engine, time.Minute)
	if err := scheduler.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() 失败: %v", err)
	}

	if len(me.calls) != 0 {
		t.Errorf("冷却期内不应触发动作，但执行了 %d 个", len(me.calls))
	}
}

func TestScheduler_StartStop(t *testing.T) {
	ms := &mockStore{}
	me := &mockExecutor{}

	engine := rules.NewEngine(ms, me)
	registry := NewSourceRegistry(ms)

	scheduler := NewScheduler(registry, engine, 50*time.Millisecond)

	ctx := context.Background()
	scheduler.Start(ctx)
	time.Sleep(120 * time.Millisecond)
	scheduler.Stop()
}
