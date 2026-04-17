package store

import (
	"context"
	"encoding/json"
	"time"
)

// Store 定义所有持久化操作接口
type Store interface {
	// signal sources
	CreateSignalSource(ctx context.Context, s *SignalSource) error
	GetSignalSource(ctx context.Context, id int64) (*SignalSource, error)
	ListSignalSources(ctx context.Context, status string) ([]*SignalSource, error)
	UpdateSignalSourceStatus(ctx context.Context, id int64, status string) error

	// trigger rules
	CreateTriggerRule(ctx context.Context, r *TriggerRule) error
	GetTriggerRule(ctx context.Context, id int64) (*TriggerRule, error)
	ListTriggerRules(ctx context.Context, status string) ([]*TriggerRule, error)
	UpdateTriggerRuleStatus(ctx context.Context, id int64, status string) error
	RecordRuleFired(ctx context.Context, id int64, firedAt time.Time) error

	// signal snapshots
	SaveSignalSnapshot(ctx context.Context, snap *SignalSnapshot) error
	GetLatestSnapshot(ctx context.Context, sourceID int64) (*SignalSnapshot, error)
	GetLatestSnapshotByType(ctx context.Context, sourceType string) (*SignalSnapshot, error)

	// trigger firings
	SaveTriggerFiring(ctx context.Context, f *TriggerFiring) error
	ListTriggerFirings(ctx context.Context, filter ListFiringsFilter) ([]*TriggerFiring, error)
}

// IngestSignalRequest 信号摄入请求
type IngestSignalRequest struct {
	Source string         `json:"source"`
	Data   map[string]any `json:"data"`
	RawMsg json.RawMessage
}
