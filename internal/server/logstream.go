package server

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/shahar-caura/forge/internal/state"
)

// handleLogStream serves agent log content as SSE events.
// GET /api/runs/{id}/logs?step=N
func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")

	// #1: Reject path traversal attempts.
	if strings.ContainsAny(runID, "/\\.") {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	stepStr := r.URL.Query().Get("step")
	if stepStr == "" {
		http.Error(w, `missing "step" query parameter`, http.StatusBadRequest)
		return
	}
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		http.Error(w, `"step" must be an integer`, http.StatusBadRequest)
		return
	}

	logFile := filepath.Join(s.runsDir, fmt.Sprintf("%s-agent-step%d.log", runID, step))
	f, err := os.Open(logFile)
	if err != nil {
		http.Error(w, "log file not found", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// #6: Set up watcher BEFORE draining existing content to avoid
	// missing writes between drain and watch registration.
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		defer func() { _ = watcher.Close() }()
		_ = watcher.Add(logFile)
	}

	// Send existing content line by line.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024) // handle long lines
	for scanner.Scan() {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", escapeSSE(scanner.Text()))
		flusher.Flush()
	}

	// Check if the run is still active — if not, close the stream.
	if !s.isRunActive(runID) {
		return
	}

	// If watcher setup failed, we can't tail — return after existing content.
	if err != nil {
		return
	}

	// Track current position for tailing.
	offset, _ := f.Seek(0, io.SeekCurrent)
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == 0 {
				continue
			}
			// Read new content from offset.
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				return
			}
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 256*1024), 256*1024)
			for scanner.Scan() {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", escapeSSE(scanner.Text()))
				flusher.Flush()
			}
			offset, _ = f.Seek(0, io.SeekCurrent)

			// If run completed while we were reading, send remaining and close.
			if !s.isRunActive(runID) {
				return
			}
		// #8: Log watcher errors instead of silently discarding.
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			if err != nil && s.logger != nil {
				s.logger.Warn("log stream watcher error", "run", runID, "err", err)
			}
		}
	}
}

// #2: isRunActive properly loads state instead of fragile string matching.
func (s *Server) isRunActive(runID string) bool {
	statePath := filepath.Join(s.runsDir, runID+".yaml")
	rs, err := state.LoadFile(statePath)
	if err != nil {
		return false
	}
	return rs.Status == state.RunActive
}

// #4: escapeSSE strips both \r and \n for SSE safety.
func escapeSSE(line string) string {
	line = strings.ReplaceAll(line, "\r", "")
	line = strings.ReplaceAll(line, "\n", "")
	return line
}
