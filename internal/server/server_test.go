package server_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shahar-caura/forge/internal/registry"
	"github.com/shahar-caura/forge/internal/server"
	"github.com/shahar-caura/forge/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	state.SetRunsDir(dir)
	t.Cleanup(func() { state.SetRunsDir(".forge/runs") })

	// Create two fixture run files.
	runs := []*state.RunState{
		{
			ID:        "run-001",
			PlanPath:  "plans/auth.md",
			Status:    state.RunCompleted,
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
			Branch:    "feat/auth",
			PlanTitle: "Auth feature",
			Steps: []state.StepState{
				{Name: "read plan", Status: state.StepCompleted},
				{Name: "create issue", Status: state.StepCompleted},
			},
		},
		{
			ID:        "run-002",
			PlanPath:  "plans/fix.md",
			Status:    state.RunFailed,
			CreatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2025, 1, 2, 1, 0, 0, 0, time.UTC),
			Steps: []state.StepState{
				{Name: "read plan", Status: state.StepCompleted},
				{Name: "run agent", Status: state.StepFailed, Error: "timeout"},
			},
		},
	}

	for _, rs := range runs {
		require.NoError(t, rs.Save())
	}

	return dir
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	h := &server.Handlers{
		Version:   "test-v0.1.0",
		StartTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	strict := server.NewStrictHandler(h, nil)

	mux := http.NewServeMux()
	server.HandlerFromMuxWithBaseURL(strict, mux, "/api")
	return mux
}

func TestGetHealth(t *testing.T) {
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp server.HealthResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "test-v0.1.0", resp.Version)
	assert.Greater(t, resp.UptimeSeconds, 0)
}

func TestListRuns(t *testing.T) {
	setupFixtures(t)
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp server.RunList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, 2, resp.Total)
	assert.Len(t, resp.Runs, 2)
	// Sorted by created_at desc — run-002 first.
	assert.Equal(t, "run-002", resp.Runs[0].Id)
	assert.Equal(t, "run-001", resp.Runs[1].Id)
}

func TestListRunsFilterByStatus(t *testing.T) {
	setupFixtures(t)
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/runs?status=failed", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp server.RunList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, "run-002", resp.Runs[0].Id)
}

func TestListRunsPagination(t *testing.T) {
	setupFixtures(t)
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/runs?limit=1&offset=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp server.RunList
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, 2, resp.Total) // total is unaffected by pagination
	assert.Len(t, resp.Runs, 1)
	assert.Equal(t, "run-001", resp.Runs[0].Id)
}

func TestGetRun(t *testing.T) {
	setupFixtures(t)
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-001", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp server.Run
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "run-001", resp.Id)
	assert.Equal(t, "plans/auth.md", resp.PlanPath)
	assert.NotNil(t, resp.Branch)
	assert.Equal(t, "feat/auth", *resp.Branch)
	assert.Len(t, resp.Steps, 2)
}

func TestGetRunNotFound(t *testing.T) {
	dir := t.TempDir()
	state.SetRunsDir(dir)
	t.Cleanup(func() { state.SetRunsDir(".forge/runs") })

	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/nonexistent", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp server.ErrorResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, 404, resp.Code)
}

func TestGetRunStepError(t *testing.T) {
	setupFixtures(t)
	handler := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-002", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp server.Run
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	// Find the failed step.
	var failedStep *server.StepState
	for i := range resp.Steps {
		if resp.Steps[i].Status == server.StepStatusFailed {
			failedStep = &resp.Steps[i]
			break
		}
	}
	require.NotNil(t, failedStep)
	require.NotNil(t, failedStep.Error)
	assert.Equal(t, "timeout", *failedStep.Error)
}

func newTestMux(t *testing.T, runsDir string) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	h := &server.Handlers{
		Version:   "test-v0.1.0",
		StartTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Logger:    logger,
	}
	strict := server.NewStrictHandler(h, nil)
	mux := http.NewServeMux()
	server.HandlerFromMuxWithBaseURL(strict, mux, "/api")

	srv := server.New(0, runsDir, "test", logger)
	srv.RegisterLogStream(mux)
	return mux
}

func TestLogStreamReturnsExistingContent(t *testing.T) {
	dir := setupFixtures(t)

	// Write a log file.
	logContent := "line 1\nline 2\nline 3\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run-001-agent-step4.log"), []byte(logContent), 0o644))

	handler := newTestMux(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-001/logs?step=4", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))

	body := rec.Body.String()
	assert.Contains(t, body, "data: line 1")
	assert.Contains(t, body, "data: line 2")
	assert.Contains(t, body, "data: line 3")
}

func TestLogStreamMissingStep(t *testing.T) {
	dir := setupFixtures(t)
	handler := newTestMux(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-001/logs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLogStreamNotFound(t *testing.T) {
	dir := setupFixtures(t)
	handler := newTestMux(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-001/logs?step=99", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestLogStreamInvalidRunID(t *testing.T) {
	dir := setupFixtures(t)
	handler := newTestMux(t, dir)

	// Run IDs with dots or backslashes are rejected to prevent path traversal.
	for _, id := range []string{"..secret", "run.001", "run\\bad"} {
		req := httptest.NewRequest(http.MethodGet, "/api/runs/"+id+"/logs?step=4", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code, "id=%q", id)
		assert.Contains(t, rec.Body.String(), "invalid run ID", "id=%q", id)
	}
}

func TestLogStreamStripsCRLF(t *testing.T) {
	dir := setupFixtures(t)
	logContent := "line 1\r\nline 2\r\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run-001-agent-step4.log"), []byte(logContent), 0o644))

	handler := newTestMux(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-001/logs?step=4", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "\r")
	assert.Contains(t, body, "data: line 1")
	assert.Contains(t, body, "data: line 2")
}

func TestGetRunMultiRepo(t *testing.T) {
	baseDir := t.TempDir()

	// Create a "remote" repo with its own .forge/runs directory.
	repoRoot := filepath.Join(baseDir, "remote-repo")
	remoteRunsDir := filepath.Join(repoRoot, ".forge", "runs")
	require.NoError(t, os.MkdirAll(remoteRunsDir, 0o755))

	// Save a run into the remote repo's runs directory.
	state.SetRunsDir(remoteRunsDir)
	rs := &state.RunState{
		ID:        "remote-run-001",
		PlanPath:  "plans/remote.md",
		Status:    state.RunCompleted,
		CreatedAt: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2025, 1, 3, 1, 0, 0, 0, time.UTC),
		Steps:     []state.StepState{{Name: "read plan", Status: state.StepCompleted}},
	}
	require.NoError(t, rs.Save())

	// Register the remote repo in the registry.
	regFile := filepath.Join(baseDir, "repos.yaml")
	registry.SetPath(regFile)
	t.Cleanup(func() { registry.SetPath("") })
	registry.Touch(repoRoot)

	// Point local runs dir at an empty directory — no matching run locally.
	localDir := t.TempDir()
	state.SetRunsDir(localDir)
	t.Cleanup(func() { state.SetRunsDir(".forge/runs") })

	// Build a handler with MultiRepo=true.
	h := &server.Handlers{
		Version:   "test",
		StartTime: time.Now(),
		MultiRepo: true,
	}
	strict := server.NewStrictHandler(h, nil)
	mux := http.NewServeMux()
	server.HandlerFromMuxWithBaseURL(strict, mux, "/api")

	req := httptest.NewRequest(http.MethodGet, "/api/runs/remote-run-001", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp server.Run
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "remote-run-001", resp.Id)
	assert.Equal(t, "plans/remote.md", resp.PlanPath)
}

func TestSPAHandler(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>SPA</html>"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log('hi')"), 0o644))

	handler := server.SPAHandler(os.DirFS(dir))

	t.Run("serves index.html at root", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "SPA")
	})

	t.Run("serves static file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "console.log")
	})

	t.Run("falls back to index.html for unknown paths", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/runs/some-id", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "SPA")
	})
}
