package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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
