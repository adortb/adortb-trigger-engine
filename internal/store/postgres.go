package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres PostgreSQL 实现
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres 创建 PostgreSQL store
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析 DSN 失败: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("PostgreSQL 连接失败: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Close 关闭连接池
func (p *Postgres) Close() {
	p.pool.Close()
}

// ─── Signal Sources ────────────────────────────────────────────────────────

func (p *Postgres) CreateSignalSource(ctx context.Context, s *SignalSource) error {
	auth, _ := json.Marshal(s.Auth)
	row := p.pool.QueryRow(ctx, `
		INSERT INTO signal_sources (name, type, endpoint_url, auth, polling_interval_sec, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		s.Name, s.Type, s.EndpointURL, auth, s.PollingIntervalSec, s.Status,
	)
	return row.Scan(&s.ID, &s.CreatedAt)
}

func (p *Postgres) GetSignalSource(ctx context.Context, id int64) (*SignalSource, error) {
	var s SignalSource
	var auth []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id, name, type, endpoint_url, auth, polling_interval_sec, status, created_at
		FROM signal_sources WHERE id=$1`, id).
		Scan(&s.ID, &s.Name, &s.Type, &s.EndpointURL, &auth,
			&s.PollingIntervalSec, &s.Status, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("获取 signal_source %d 失败: %w", id, err)
	}
	s.Auth = json.RawMessage(auth)
	return &s, nil
}

func (p *Postgres) ListSignalSources(ctx context.Context, status string) ([]*SignalSource, error) {
	query := `SELECT id, name, type, endpoint_url, auth, polling_interval_sec, status, created_at
		FROM signal_sources`
	args := []any{}
	if status != "" {
		query += " WHERE status=$1"
		args = append(args, status)
	}
	query += " ORDER BY id"

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询 signal_sources 失败: %w", err)
	}
	defer rows.Close()

	var result []*SignalSource
	for rows.Next() {
		var s SignalSource
		var auth []byte
		if err := rows.Scan(&s.ID, &s.Name, &s.Type, &s.EndpointURL, &auth,
			&s.PollingIntervalSec, &s.Status, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描 signal_source 失败: %w", err)
		}
		s.Auth = json.RawMessage(auth)
		result = append(result, &s)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateSignalSourceStatus(ctx context.Context, id int64, status string) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE signal_sources SET status=$1 WHERE id=$2`, status, id)
	if err != nil {
		return fmt.Errorf("更新 signal_source 状态失败: %w", err)
	}
	return nil
}

// ─── Trigger Rules ─────────────────────────────────────────────────────────

func (p *Postgres) CreateTriggerRule(ctx context.Context, r *TriggerRule) error {
	row := p.pool.QueryRow(ctx, `
		INSERT INTO trigger_rules (advertiser_id, name, campaign_id, conditions, actions, cooldown_sec, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		r.AdvertiserID, r.Name, r.CampaignID,
		[]byte(r.Conditions), []byte(r.Actions),
		r.CooldownSec, r.Status,
	)
	return row.Scan(&r.ID, &r.CreatedAt)
}

func (p *Postgres) GetTriggerRule(ctx context.Context, id int64) (*TriggerRule, error) {
	var r TriggerRule
	var cond, actions []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id, advertiser_id, name, campaign_id, conditions, actions,
			cooldown_sec, last_fired_at, fire_count, status, created_at
		FROM trigger_rules WHERE id=$1`, id).
		Scan(&r.ID, &r.AdvertiserID, &r.Name, &r.CampaignID,
			&cond, &actions, &r.CooldownSec, &r.LastFiredAt,
			&r.FireCount, &r.Status, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("获取 trigger_rule %d 失败: %w", id, err)
	}
	r.Conditions = json.RawMessage(cond)
	r.Actions = json.RawMessage(actions)
	return &r, nil
}

func (p *Postgres) ListTriggerRules(ctx context.Context, status string) ([]*TriggerRule, error) {
	query := `SELECT id, advertiser_id, name, campaign_id, conditions, actions,
		cooldown_sec, last_fired_at, fire_count, status, created_at
		FROM trigger_rules`
	args := []any{}
	if status != "" {
		query += " WHERE status=$1"
		args = append(args, status)
	}
	query += " ORDER BY id"

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询 trigger_rules 失败: %w", err)
	}
	defer rows.Close()

	var result []*TriggerRule
	for rows.Next() {
		var r TriggerRule
		var cond, actions []byte
		if err := rows.Scan(&r.ID, &r.AdvertiserID, &r.Name, &r.CampaignID,
			&cond, &actions, &r.CooldownSec, &r.LastFiredAt,
			&r.FireCount, &r.Status, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("扫描 trigger_rule 失败: %w", err)
		}
		r.Conditions = json.RawMessage(cond)
		r.Actions = json.RawMessage(actions)
		result = append(result, &r)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateTriggerRuleStatus(ctx context.Context, id int64, status string) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE trigger_rules SET status=$1 WHERE id=$2`, status, id)
	if err != nil {
		return fmt.Errorf("更新 trigger_rule 状态失败: %w", err)
	}
	return nil
}

func (p *Postgres) RecordRuleFired(ctx context.Context, id int64, firedAt time.Time) error {
	_, err := p.pool.Exec(ctx, `
		UPDATE trigger_rules
		SET last_fired_at=$1, fire_count=fire_count+1
		WHERE id=$2`, firedAt, id)
	if err != nil {
		return fmt.Errorf("更新规则触发记录失败: %w", err)
	}
	return nil
}

// ─── Signal Snapshots ──────────────────────────────────────────────────────

func (p *Postgres) SaveSignalSnapshot(ctx context.Context, snap *SignalSnapshot) error {
	row := p.pool.QueryRow(ctx, `
		INSERT INTO signal_snapshots (source_id, data)
		VALUES ($1, $2)
		RETURNING id, received_at`,
		snap.SourceID, []byte(snap.Data),
	)
	return row.Scan(&snap.ID, &snap.ReceivedAt)
}

func (p *Postgres) GetLatestSnapshot(ctx context.Context, sourceID int64) (*SignalSnapshot, error) {
	var snap SignalSnapshot
	var data []byte
	err := p.pool.QueryRow(ctx, `
		SELECT id, source_id, data, received_at
		FROM signal_snapshots
		WHERE source_id=$1
		ORDER BY received_at DESC
		LIMIT 1`, sourceID).
		Scan(&snap.ID, &snap.SourceID, &data, &snap.ReceivedAt)
	if err != nil {
		return nil, fmt.Errorf("获取最新信号快照失败: %w", err)
	}
	snap.Data = json.RawMessage(data)
	return &snap, nil
}

func (p *Postgres) GetLatestSnapshotByType(ctx context.Context, sourceType string) (*SignalSnapshot, error) {
	var snap SignalSnapshot
	var data []byte
	err := p.pool.QueryRow(ctx, `
		SELECT ss.id, ss.source_id, ss.data, ss.received_at
		FROM signal_snapshots ss
		JOIN signal_sources src ON src.id = ss.source_id
		WHERE src.type=$1
		ORDER BY ss.received_at DESC
		LIMIT 1`, sourceType).
		Scan(&snap.ID, &snap.SourceID, &data, &snap.ReceivedAt)
	if err != nil {
		return nil, fmt.Errorf("获取类型 %s 最新快照失败: %w", sourceType, err)
	}
	snap.Data = json.RawMessage(data)
	return &snap, nil
}

// ─── Trigger Firings ───────────────────────────────────────────────────────

func (p *Postgres) SaveTriggerFiring(ctx context.Context, f *TriggerFiring) error {
	row := p.pool.QueryRow(ctx, `
		INSERT INTO trigger_firings (rule_id, signal_snapshot, actions_executed)
		VALUES ($1, $2, $3)
		RETURNING id, fired_at`,
		f.RuleID, []byte(f.SignalSnapshot), []byte(f.ActionsExecuted),
	)
	return row.Scan(&f.ID, &f.FiredAt)
}

func (p *Postgres) ListTriggerFirings(ctx context.Context, filter ListFiringsFilter) ([]*TriggerFiring, error) {
	query := `SELECT id, rule_id, signal_snapshot, actions_executed, fired_at
		FROM trigger_firings WHERE 1=1`
	args := []any{}
	n := 1

	if filter.RuleID > 0 {
		query += fmt.Sprintf(" AND rule_id=$%d", n)
		args = append(args, filter.RuleID)
		n++
	}
	if filter.From != nil {
		query += fmt.Sprintf(" AND fired_at>=$%d", n)
		args = append(args, *filter.From)
		n++
	}
	if filter.To != nil {
		query += fmt.Sprintf(" AND fired_at<=$%d", n)
		args = append(args, *filter.To)
		n++
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query += fmt.Sprintf(" ORDER BY fired_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, filter.Offset)

	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询触发记录失败: %w", err)
	}
	defer rows.Close()

	var result []*TriggerFiring
	for rows.Next() {
		var f TriggerFiring
		var snap, actions []byte
		if err := rows.Scan(&f.ID, &f.RuleID, &snap, &actions, &f.FiredAt); err != nil {
			return nil, fmt.Errorf("扫描触发记录失败: %w", err)
		}
		f.SignalSnapshot = json.RawMessage(snap)
		f.ActionsExecuted = json.RawMessage(actions)
		result = append(result, &f)
	}
	return result, rows.Err()
}
