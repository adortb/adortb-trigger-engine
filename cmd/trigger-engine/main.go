package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adortb/adortb-trigger-engine/internal/api"
	triggermetrics "github.com/adortb/adortb-trigger-engine/internal/metrics"
	"github.com/adortb/adortb-trigger-engine/internal/rules"
	"github.com/adortb/adortb-trigger-engine/internal/store"
	"github.com/adortb/adortb-trigger-engine/internal/watchers"
)

func main() {
	cfg := loadConfig()

	triggermetrics.Register()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 连接 PostgreSQL
	pg, err := store.NewPostgres(ctx, cfg.dsn)
	if err != nil {
		log.Fatalf("PostgreSQL 连接失败: %v", err)
	}
	defer pg.Close()

	// 初始化信号源注册表并从 DB 加载
	registry := watchers.NewSourceRegistry(pg)
	if err := registry.LoadFromDB(ctx); err != nil {
		log.Printf("[WARN] 从 DB 加载信号源失败: %v", err)
	}

	// 初始化规则引擎和动作执行器
	executor := rules.NewHTTPActionExecutor(cfg.adminURL)
	engine := rules.NewEngine(pg, executor)

	// 初始化调度器
	scheduler := watchers.NewScheduler(registry, engine, cfg.pollInterval)
	scheduler.Start(ctx)

	// 初始化 HTTP 服务
	handler := api.NewHandler(pg, engine, registry)
	srv := api.NewServer(cfg.port, handler)

	// 优雅停机
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("adortb-trigger-engine 启动，端口 :%d，轮询间隔 %s，admin %s",
			cfg.port, cfg.pollInterval, cfg.adminURL)
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务器错误: %v", err)
		}
	}()

	<-sigCh
	log.Println("收到停机信号，开始优雅退出...")

	scheduler.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP 停机错误: %v", err)
	}
	log.Println("触发引擎已停止")
}

type config struct {
	port         int
	dsn          string
	adminURL     string
	pollInterval time.Duration
}

func loadConfig() config {
	port := 8112
	if v := os.Getenv("TRIGGER_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &port)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/adortb_trigger?sslmode=disable"
	}

	adminURL := os.Getenv("ADMIN_BASE_URL")
	if adminURL == "" {
		adminURL = "http://localhost:8080"
	}

	pollInterval := 5 * time.Minute
	if v := os.Getenv("POLL_INTERVAL_SEC"); v != "" {
		var sec int
		fmt.Sscanf(v, "%d", &sec)
		if sec > 0 {
			pollInterval = time.Duration(sec) * time.Second
		}
	}

	return config{
		port:         port,
		dsn:          dsn,
		adminURL:     adminURL,
		pollInterval: pollInterval,
	}
}
