package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server HTTP 服务器
type Server struct {
	srv     *http.Server
	handler *Handler
}

// NewServer 创建 HTTP 服务器，端口 8112
func NewServer(port int, h *Handler) *Server {
	mux := http.NewServeMux()
	s := &Server{handler: h}
	s.registerRoutes(mux)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// signal sources
	mux.HandleFunc("/v1/sources", s.handler.HandleCreateSource)
	// signal ingestion & query
	mux.HandleFunc("/v1/signals/ingest", s.handler.HandleIngestSignal)
	mux.HandleFunc("/v1/signals/", s.handler.HandleGetCurrentSignal)
	// trigger rules
	mux.HandleFunc("/v1/rules", s.handler.HandleCreateRule)
	mux.HandleFunc("/v1/rules/", s.routeRules)
	// firings history
	mux.HandleFunc("/v1/firings", s.handler.HandleListFirings)
	// infra
	mux.HandleFunc("/health", s.handler.HandleHealth)
	mux.Handle("/metrics", promhttp.Handler())
}

// routeRules 路由带 ID 的规则操作
func (s *Server) routeRules(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case hasSuffix(path, "/activate"):
		s.handler.HandleActivateRule(w, r)
	case hasSuffix(path, "/deactivate"):
		s.handler.HandleDeactivateRule(w, r)
	case hasSuffix(path, "/test"):
		s.handler.HandleTestRule(w, r)
	default:
		http.NotFound(w, r)
	}
}

func hasSuffix(path, suffix string) bool {
	return len(path) > len(suffix) && path[len(path)-len(suffix):] == suffix
}

// Start 启动服务器（阻塞）
func (s *Server) Start() error {
	return s.srv.ListenAndServe()
}

// Shutdown 优雅停机
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
