package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adortb/adortb-trigger-engine/internal/rules"
	"github.com/adortb/adortb-trigger-engine/internal/signals"
	"github.com/adortb/adortb-trigger-engine/internal/store"
	"github.com/adortb/adortb-trigger-engine/internal/watchers"
)

// Handler HTTP 请求处理器
type Handler struct {
	store    store.Store
	engine   *rules.Engine
	registry *watchers.SourceRegistry
}

// NewHandler 创建处理器
func NewHandler(s store.Store, engine *rules.Engine, registry *watchers.SourceRegistry) *Handler {
	return &Handler{store: s, engine: engine, registry: registry}
}

// ─── Signal Sources ────────────────────────────────────────────────────────

// HandleCreateSource POST /v1/sources
func (h *Handler) HandleCreateSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}
	var src store.SignalSource
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
		return
	}
	if src.Name == "" || src.Type == "" {
		writeError(w, http.StatusBadRequest, "name 和 type 为必填字段")
		return
	}
	if src.Status == "" {
		src.Status = "active"
	}
	if src.PollingIntervalSec <= 0 {
		src.PollingIntervalSec = 300
	}

	if err := h.store.CreateSignalSource(r.Context(), &src); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("创建失败: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, src)
}

// HandleGetCurrentSignal GET /v1/signals/:source_id/current
func (h *Handler) HandleGetCurrentSignal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}
	id, err := extractPathID(r.URL.Path, "/v1/signals/", "/current")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 优先从内存中的活跃信号源拉取
	src, ok := h.registry.Get(id)
	if ok {
		data, err := src.Fetch(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("拉取信号失败: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"source_id": id, "data": data})
		return
	}

	// 从数据库获取最新快照
	snap, err := h.store.GetLatestSnapshot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "信号快照不存在")
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// HandleIngestSignal POST /v1/signals/ingest
func (h *Handler) HandleIngestSignal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}
	var req struct {
		Source string         `json:"source"`
		Data   map[string]any `json:"data"`
	}
	// 支持直接发整个 payload，source 字段为必须
	raw := make(map[string]any)
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
		return
	}

	srcType, _ := raw["source"].(string)
	if srcType == "" {
		writeError(w, http.StatusBadRequest, "source 字段为必填")
		return
	}
	req.Source = srcType
	// 去掉 source 字段，其余为信号数据
	delete(raw, "source")
	req.Data = raw

	// 查找对应的信号源并推送（EventSource）
	sources, err := h.store.ListSignalSources(r.Context(), "active")
	if err == nil {
		for _, s := range sources {
			if s.Type == signals.TypeEvent && (s.Name == srcType || s.Type == srcType) {
				if evtSrc, ok := h.registry.Get(s.ID); ok {
					if es, ok := evtSrc.(*signals.EventSource); ok {
						es.Push(signals.SignalData(req.Data))
					}
				}
			}
		}
	}

	// 保存快照
	dataJSON, _ := json.Marshal(req.Data)
	// 查找匹配的 source_id
	var sourceID int64
	if err == nil {
		for _, s := range sources {
			if s.Type == srcType || s.Name == srcType {
				sourceID = s.ID
				break
			}
		}
	}
	if sourceID > 0 {
		snap := &store.SignalSnapshot{
			SourceID: sourceID,
			Data:     json.RawMessage(dataJSON),
		}
		_ = h.store.SaveSignalSnapshot(r.Context(), snap)
	}

	// 立即触发规则评估
	go func() {
		signalMap := map[string]any{srcType: req.Data}
		if fireErr := h.engine.EvaluateAndFire(r.Context(), signalMap); fireErr != nil {
			// 后台处理，不影响响应
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "source": srcType})
}

// ─── Trigger Rules ─────────────────────────────────────────────────────────

// HandleCreateRule POST /v1/rules
func (h *Handler) HandleCreateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}
	var rule store.TriggerRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
		return
	}
	if rule.Name == "" {
		writeError(w, http.StatusBadRequest, "name 为必填字段")
		return
	}
	if len(rule.Conditions) == 0 {
		writeError(w, http.StatusBadRequest, "conditions 为必填字段")
		return
	}
	if len(rule.Actions) == 0 {
		writeError(w, http.StatusBadRequest, "actions 为必填字段")
		return
	}
	if rule.Status == "" {
		rule.Status = "active"
	}
	if rule.CooldownSec <= 0 {
		rule.CooldownSec = 3600
	}

	if err := h.store.CreateTriggerRule(r.Context(), &rule); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("创建规则失败: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

// HandleActivateRule POST /v1/rules/:id/activate
func (h *Handler) HandleActivateRule(w http.ResponseWriter, r *http.Request) {
	h.handleRuleStatusChange(w, r, "active", "/activate")
}

// HandleDeactivateRule POST /v1/rules/:id/deactivate
func (h *Handler) HandleDeactivateRule(w http.ResponseWriter, r *http.Request) {
	h.handleRuleStatusChange(w, r, "inactive", "/deactivate")
}

func (h *Handler) handleRuleStatusChange(w http.ResponseWriter, r *http.Request, status, suffix string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}
	id, err := extractPathID(r.URL.Path, "/v1/rules/", suffix)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.store.UpdateTriggerRuleStatus(r.Context(), id, status); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("更新状态失败: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": status})
}

// HandleTestRule POST /v1/rules/:id/test
func (h *Handler) HandleTestRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}
	id, err := extractPathID(r.URL.Path, "/v1/rules/", "/test")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	rule, err := h.store.GetTriggerRule(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "规则不存在")
		return
	}

	var simulatedSignals map[string]any
	if err := json.NewDecoder(r.Body).Decode(&simulatedSignals); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("解析模拟信号失败: %v", err))
		return
	}

	matched, err := h.engine.EvaluateRuleWithSignals(rule, simulatedSignals)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("评估失败: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rule_id":  id,
		"matched":  matched,
		"signals":  simulatedSignals,
		"evaluated_at": time.Now().UTC(),
	})
}

// ─── Firings ───────────────────────────────────────────────────────────────

// HandleListFirings GET /v1/firings
func (h *Handler) HandleListFirings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}
	q := r.URL.Query()
	filter := store.ListFiringsFilter{Limit: 50}

	if v := q.Get("rule_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.RuleID = id
		}
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.To = &t
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}

	firings, err := h.store.ListTriggerFirings(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("查询失败: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"firings": firings, "count": len(firings)})
}

// HandleHealth GET /health
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "adortb-trigger-engine"})
}

// ─── 工具函数 ──────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

// extractPathID 从路径中提取 ID，如 /v1/rules/123/activate → 123
func extractPathID(path, prefix, suffix string) (int64, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimSuffix(trimmed, suffix)
	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的 ID: %s", trimmed)
	}
	return id, nil
}
