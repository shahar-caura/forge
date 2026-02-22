package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/shahar-caura/forge/web"
)

// Server is the forge dashboard HTTP server.
type Server struct {
	port      int
	runsDir   string
	version   string
	startTime time.Time
	sseHub    *SSEHub
	logger    *slog.Logger
	registry  bool // aggregate runs from all registered repos
}

// New creates a Server with the given options.
func New(port int, runsDir, version string, logger *slog.Logger) *Server {
	return &Server{
		port:      port,
		runsDir:   runsDir,
		version:   version,
		startTime: time.Now(),
		logger:    logger,
	}
}

// SetMultiRepo enables aggregation of runs from all registered repos.
func (s *Server) SetMultiRepo(enabled bool) { s.registry = enabled }

// RegisterLogStream adds the log streaming endpoint to the given mux.
func (s *Server) RegisterLogStream(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/runs/{id}/logs", s.handleLogStream)
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	// Wire up generated strict handlers.
	h := &Handlers{
		Version:   s.version,
		StartTime: s.startTime,
		Logger:    s.logger,
		MultiRepo: s.registry,
	}
	strictHandler := NewStrictHandler(h, nil)
	HandlerFromMuxWithBaseURL(strictHandler, mux, "/api")

	// SSE endpoint (outside codegen — streaming is incompatible with strict mode).
	s.sseHub = NewSSEHub(s.runsDir, s.logger)
	mux.Handle("GET /api/events", s.sseHub)

	// Log streaming SSE endpoint.
	s.RegisterLogStream(mux)

	// SPA catch-all: serves embedded static files, falls back to index.html.
	mux.Handle("/", SPAHandler(web.DistFS))

	addr := fmt.Sprintf(":%d", s.port)
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start SSE hub watcher.
	go s.sseHub.Start(ctx)

	// Start listener so we can log the actual port.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	s.logger.Info("dashboard server started", "addr", ln.Addr().String())

	// Graceful shutdown on context cancellation.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
