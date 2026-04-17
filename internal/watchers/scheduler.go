package watchers

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/adortb/adortb-trigger-engine/internal/rules"
)

// Scheduler 定时调度器，定期拉取信号并触发规则评估
type Scheduler struct {
	registry *SourceRegistry
	engine   *rules.Engine
	interval time.Duration
	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewScheduler 创建调度器
// interval 为默认轮询间隔（各信号源可覆盖）
func NewScheduler(registry *SourceRegistry, engine *rules.Engine, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &Scheduler{
		registry: registry,
		engine:   engine,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start 启动调度循环（非阻塞）
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})

	go s.loop(ctx)
	log.Printf("[INFO] 触发引擎调度器已启动，轮询间隔 %s", s.interval)
}

// Stop 停止调度循环并等待退出
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	<-s.doneCh
	log.Println("[INFO] 触发引擎调度器已停止")
}

// RunOnce 立即执行一次拉取+评估（用于测试或手动触发）
func (s *Scheduler) RunOnce(ctx context.Context) error {
	return s.tick(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 启动时立即执行一次
	if err := s.tick(ctx); err != nil {
		log.Printf("[ERROR] 初始触发评估失败: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := s.tick(ctx); err != nil {
				log.Printf("[ERROR] 触发评估失败: %v", err)
			}
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) error {
	log.Println("[DEBUG] 开始信号拉取与规则评估...")

	signals, err := s.registry.FetchAllAndSave(ctx)
	if err != nil {
		// 部分失败时仍继续评估已成功的信号
		log.Printf("[WARN] 部分信号源拉取失败: %v", err)
	}

	if len(signals) == 0 {
		log.Println("[DEBUG] 无可用信号，跳过规则评估")
		return nil
	}

	if err := s.engine.EvaluateAndFire(ctx, signals); err != nil {
		log.Printf("[ERROR] 规则评估失败: %v", err)
		return err
	}

	log.Printf("[DEBUG] 规则评估完成，信号源数: %d", len(signals))
	return nil
}
