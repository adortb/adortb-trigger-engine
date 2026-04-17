package store

import (
	"encoding/json"
	"time"
)

type SignalSource struct {
	ID                 int64           `json:"id"`
	Name               string          `json:"name"`
	Type               string          `json:"type"`
	EndpointURL        string          `json:"endpoint_url"`
	Auth               json.RawMessage `json:"auth,omitempty"`
	PollingIntervalSec int             `json:"polling_interval_sec"`
	Status             string          `json:"status"`
	CreatedAt          time.Time       `json:"created_at"`
}

type TriggerRule struct {
	ID           int64           `json:"id"`
	AdvertiserID int64           `json:"advertiser_id"`
	Name         string          `json:"name"`
	CampaignID   int64           `json:"campaign_id"`
	Conditions   json.RawMessage `json:"conditions"`
	Actions      json.RawMessage `json:"actions"`
	CooldownSec  int             `json:"cooldown_sec"`
	LastFiredAt  *time.Time      `json:"last_fired_at,omitempty"`
	FireCount    int64           `json:"fire_count"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
}

type TriggerFiring struct {
	ID              int64           `json:"id"`
	RuleID          int64           `json:"rule_id"`
	SignalSnapshot  json.RawMessage `json:"signal_snapshot"`
	ActionsExecuted json.RawMessage `json:"actions_executed"`
	FiredAt         time.Time       `json:"fired_at"`
}

type SignalSnapshot struct {
	ID         int64           `json:"id"`
	SourceID   int64           `json:"source_id"`
	Data       json.RawMessage `json:"data"`
	ReceivedAt time.Time       `json:"received_at"`
}

type ListFiringsFilter struct {
	RuleID int64
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}
