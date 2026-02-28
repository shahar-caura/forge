package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/shahar-caura/forge/internal/state"
)

// SSEHub fans out run-state change events to connected SSE clients.
type SSEHub struct {
	runsDir string
	logger  *slog.Logger

	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

// NewSSEHub creates an SSEHub watching the given runs directory.
func NewSSEHub(runsDir string, logger *slog.Logger) *SSEHub {
	return &SSEHub{
		runsDir: runsDir,
		logger:  logger,
		clients: make(map[chan []byte]struct{}),
	}
}

// Start watches runsDir for file changes and broadcasts events. Blocks until ctx is cancelled.
func (h *SSEHub) Start(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		h.logger.Error("sse: failed to create watcher", "err", err)
		return
	}
	defer func() { _ = watcher.Close() }()

	// Ensure the runs directory exists before watching.
	if err := os.MkdirAll(h.runsDir, 0o755); err != nil {
		h.logger.Error("sse: failed to create runs dir", "err", err)
		return
	}

	if err := watcher.Add(h.runsDir); err != nil {
		h.logger.Error("sse: failed to watch runs dir", "err", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			if !strings.HasSuffix(event.Name, ".yaml") || strings.HasSuffix(event.Name, ".tmp") {
				continue
			}

			id := strings.TrimSuffix(filepath.Base(event.Name), ".yaml")
			rs, err := state.Load(id)
			if err != nil {
				continue // transient read during atomic write
			}

			data, err := json.Marshal(stateToRun(rs))
			if err != nil {
				continue
			}
			h.broadcast(data)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			h.logger.Error("sse: watcher error", "err", err)
		}
	}
}

func (h *SSEHub) broadcast(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			// Slow client; drop this event.
		}
	}
}

func (h *SSEHub) addClient(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[ch] = struct{}{}
}

func (h *SSEHub) removeClient(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
	close(ch)
}

// ServeHTTP implements http.Handler for SSE connections.
func (h *SSEHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 32)
	h.addClient(ch)
	defer h.removeClient(ch)

	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case data := <-ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
