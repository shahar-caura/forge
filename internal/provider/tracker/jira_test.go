package tracker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateIssue_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/rest/api/3/issue", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Basic ")

		var body createIssueRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "PROJ", body.Fields.Project.Key)
		assert.Equal(t, "Test Issue", body.Fields.Summary)
		assert.Equal(t, "Task", body.Fields.IssueType.Name)
		assert.Equal(t, "doc", body.Fields.Description.Type)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createIssueResponse{Key: "PROJ-42"})
	}))
	defer srv.Close()

	j := New(srv.URL, "PROJ", "user@example.com", "token", "")
	issue, err := j.CreateIssue(context.Background(), "Test Issue", "description body")

	require.NoError(t, err)
	assert.Equal(t, "PROJ-42", issue.Key)
	assert.Equal(t, srv.URL+"/browse/PROJ-42", issue.URL)
}

func TestCreateIssue_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()

	j := New(srv.URL, "PROJ", "bad@example.com", "bad-token", "")
	_, err := j.CreateIssue(context.Background(), "title", "body")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "jira: unexpected status 401")
}

func TestCreateIssue_BadResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	j := New(srv.URL, "PROJ", "user@example.com", "token", "")
	_, err := j.CreateIssue(context.Background(), "title", "body")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "jira: parsing response")
}

func TestCreateIssue_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createIssueResponse{Key: "PROJ-1"})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	j := New(srv.URL, "PROJ", "user@example.com", "token", "")
	_, err := j.CreateIssue(ctx, "title", "body")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "jira: sending request")
}

func TestCreateIssue_MissingKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createIssueResponse{Key: ""})
	}))
	defer srv.Close()

	j := New(srv.URL, "PROJ", "user@example.com", "token", "")
	_, err := j.CreateIssue(context.Background(), "title", "body")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "response missing issue key")
}

func TestCreateIssue_MovesToActiveSprint(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createIssueResponse{Key: "PROJ-99"})
	})
	mux.HandleFunc("/rest/agile/1.0/board/42/sprint", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "active", r.URL.Query().Get("state"))
		json.NewEncoder(w).Encode(sprintResponse{Values: []sprint{{ID: 123}}})
	})
	mux.HandleFunc("/rest/agile/1.0/sprint/123/issue", func(w http.ResponseWriter, r *http.Request) {
		var body moveToSprintRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, []string{"PROJ-99"}, body.Issues)
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	j := New(srv.URL, "PROJ", "user@example.com", "token", "42")
	issue, err := j.CreateIssue(context.Background(), "Sprint Issue", "body")

	require.NoError(t, err)
	assert.Equal(t, "PROJ-99", issue.Key)
}

func TestCreateIssue_NoActiveSprint_SkipsMove(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createIssueResponse{Key: "PROJ-50"})
	})
	mux.HandleFunc("/rest/agile/1.0/board/42/sprint", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(sprintResponse{Values: []sprint{}})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	j := New(srv.URL, "PROJ", "user@example.com", "token", "42")
	issue, err := j.CreateIssue(context.Background(), "No Sprint Issue", "body")

	require.NoError(t, err)
	assert.Equal(t, "PROJ-50", issue.Key)
}

func TestCreateIssue_NoBoardID_SkipsMove(t *testing.T) {
	var reqCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		assert.Equal(t, "/rest/api/3/issue", r.URL.Path, "only issue create endpoint should be called")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createIssueResponse{Key: "PROJ-10"})
	}))
	defer srv.Close()

	j := New(srv.URL, "PROJ", "user@example.com", "token", "")
	issue, err := j.CreateIssue(context.Background(), "No Board Issue", "body")

	require.NoError(t, err)
	assert.Equal(t, "PROJ-10", issue.Key)
	assert.Equal(t, int32(1), reqCount.Load(), "expected exactly 1 HTTP request (issue create only)")
}

func TestCreateIssue_SprintMoveFails_ReturnsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/issue", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createIssueResponse{Key: "PROJ-77"})
	})
	mux.HandleFunc("/rest/agile/1.0/board/42/sprint", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(sprintResponse{Values: []sprint{{ID: 456}}})
	})
	mux.HandleFunc("/rest/agile/1.0/sprint/456/issue", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["sprint is closed"]}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	j := New(srv.URL, "PROJ", "user@example.com", "token", "42")
	_, err := j.CreateIssue(context.Background(), "Fail Move Issue", "body")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "jira: moving issue to sprint")
}
