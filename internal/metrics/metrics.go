package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	SignalsFetched = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "trigger_signals_fetched_total",
		Help: "信号拉取次数",
	}, []string{"source_type", "status"})

	RulesEvaluated = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "trigger_rules_evaluated_total",
		Help: "规则评估次数",
	}, []string{"result"}) // matched / not_matched

	RulesFired = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "trigger_rules_fired_total",
		Help: "规则触发（非冷却期）次数",
	}, []string{"rule_id"})

	ActionExecuted = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "trigger_actions_executed_total",
		Help: "动作执行次数",
	}, []string{"action_type", "status"})

	SignalIngestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "trigger_signal_ingest_duration_seconds",
		Help:    "信号摄入耗时",
		Buckets: prometheus.DefBuckets,
	}, []string{"source_type"})

	ActiveSources = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "trigger_active_sources",
		Help: "当前活跃信号源数量",
	})

	ActiveRules = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "trigger_active_rules",
		Help: "当前活跃规则数量",
	})
)

// Register 注册所有指标
func Register() {
	prometheus.MustRegister(
		SignalsFetched,
		RulesEvaluated,
		RulesFired,
		ActionExecuted,
		SignalIngestDuration,
		ActiveSources,
		ActiveRules,
	)
}
