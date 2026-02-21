package server_test

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shahar-caura/forge/internal/server"
	"github.com/shahar-caura/forge/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSEHubBroadcast(t *testing.T) {
	dir := t.TempDir()
	state.SetRunsDir(dir)
	t.Cleanup(func() { state.SetRunsDir(".forge/runs") })

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	hub := server.NewSSEHub(dir, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Start(ctx)

	// Give the watcher time to start.
	time.Sleep(100 * time.Millisecond)

	// Connect two SSE clients.
	bodies := make([]string, 2)
	done := make(chan int, 2)

	for i := range 2 {
		i := i
		req := httptest.NewRequest("GET", "/api/events", nil)
		rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

		go func() {
			// This blocks until context is cancelled.
			hub.ServeHTTP(rec, req.WithContext(ctx))
			bodies[i] = rec.Body.String()
			done <- i
		}()
	}

	// Give clients time to register.
	time.Sleep(100 * time.Millisecond)

	// Write a state file to trigger a broadcast.
	rs := state.New("sse-test-001", "plans/test.md")
	require.NoError(t, rs.Save())

	// Wait for events to propagate.
	time.Sleep(500 * time.Millisecond)

	// Cancel context to disconnect clients.
	cancel()

	// Wait for both clients to finish.
	for range 2 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for SSE clients to finish")
		}
	}

	// Both clients should have received the event.
	for i, body := range bodies {
		assert.True(t, strings.Contains(body, "sse-test-001"),
			"client %d did not receive event: %s", i, body)
	}
}

// flushRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {
	// no-op for testing; data is already in the buffer.
}
