package watchers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/adortb/adortb-trigger-engine/internal/signals"
	"github.com/adortb/adortb-trigger-engine/internal/store"
)

// SourceRegistry 信号源注册表，维护活跃的信号源实例
type SourceRegistry struct {
	mu      sync.RWMutex
	sources map[int64]signals.Source // sourceID -> Source
	store   store.Store
}

// NewSourceRegistry 创建信号源注册表
func NewSourceRegistry(s store.Store) *SourceRegistry {
	return &SourceRegistry{
		sources: make(map[int64]signals.Source),
		store:   s,
	}
}

// Register 注册信号源
func (r *SourceRegistry) Register(id int64, src signals.Source) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[id] = src
}

// Unregister 注销信号源
func (r *SourceRegistry) Unregister(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sources, id)
}

// Get 获取信号源
func (r *SourceRegistry) Get(id int64) (signals.Source, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	src, ok := r.sources[id]
	return src, ok
}

// All 返回所有注册的信号源（快照）
func (r *SourceRegistry) All() map[int64]signals.Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[int64]signals.Source, len(r.sources))
	for k, v := range r.sources {
		result[k] = v
	}
	return result
}

// FetchAllAndSave 拉取所有信号源数据并保存快照，返回信号集合
func (r *SourceRegistry) FetchAllAndSave(ctx context.Context) (map[string]any, error) {
	sources := r.All()
	combined := make(map[string]any, len(sources))

	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(sources))

	for id, src := range sources {
		wg.Add(1)
		go func(sourceID int64, s signals.Source) {
			defer wg.Done()

			data, err := s.Fetch(ctx)
			if err != nil {
				log.Printf("[WARN] 信号源 %d (%s) 拉取失败: %v", sourceID, s.Type(), err)
				errCh <- fmt.Errorf("source %d: %w", sourceID, err)
				return
			}

			// 保存快照
			dataJSON, _ := json.Marshal(data)
			snap := &store.SignalSnapshot{
				SourceID: sourceID,
				Data:     json.RawMessage(dataJSON),
			}
			if saveErr := r.store.SaveSignalSnapshot(ctx, snap); saveErr != nil {
				log.Printf("[WARN] 保存信号快照失败 source=%d: %v", sourceID, saveErr)
			}

			// 合并到 combined（按 type 分组）
			mu.Lock()
			combined[s.Type()] = map[string]any(data)
			mu.Unlock()

			log.Printf("[DEBUG] 信号源 %d (%s) 拉取完成", sourceID, s.Type())
		}(id, src)
	}

	wg.Wait()
	close(errCh)

	// 返回第一个错误（若有），但不中断已成功的结果
	for err := range errCh {
		if err != nil {
			return combined, err
		}
	}
	return combined, nil
}

// LoadFromDB 从数据库加载所有活跃信号源并实例化
func (r *SourceRegistry) LoadFromDB(ctx context.Context) error {
	sources, err := r.store.ListSignalSources(ctx, "active")
	if err != nil {
		return fmt.Errorf("加载信号源失败: %w", err)
	}

	for _, s := range sources {
		src := buildSource(s)
		if src != nil {
			r.Register(s.ID, src)
			log.Printf("[INFO] 加载信号源 %d (%s, type=%s)", s.ID, s.Name, s.Type)
		}
	}
	return nil
}

// buildSource 根据数据库记录实例化信号源
func buildSource(s *store.SignalSource) signals.Source {
	var authMap map[string]string
	if len(s.Auth) > 0 {
		_ = json.Unmarshal(s.Auth, &authMap)
	}

	switch s.Type {
	case signals.TypeWeather:
		apiKey := authMap["api_key"]
		return signals.NewWeatherSource(s.EndpointURL, apiKey)
	case signals.TypeLocation:
		token := authMap["token"]
		return signals.NewLocationSource(s.EndpointURL, token)
	case signals.TypeEvent:
		return signals.NewEventSource()
	case signals.TypeCustom:
		headers := make(map[string]string)
		for k, v := range authMap {
			headers[k] = v
		}
		return signals.NewCustomSource(s.EndpointURL, headers)
	default:
		log.Printf("[WARN] 未知信号源类型: %s", s.Type)
		return nil
	}
}

// WatchInterval 返回信号源的轮询间隔，默认 300 秒
func WatchInterval(s *store.SignalSource) time.Duration {
	if s.PollingIntervalSec > 0 {
		return time.Duration(s.PollingIntervalSec) * time.Second
	}
	return 300 * time.Second
}
