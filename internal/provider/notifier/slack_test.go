package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotify_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload webhookPayload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "PR ready for review: https://github.com/owner/repo/pull/1", payload.Text)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	s := New(srv.URL)
	err := s.Notify(context.Background(), "PR ready for review: https://github.com/owner/repo/pull/1")

	require.NoError(t, err)
}

func TestNotify_WebhookFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	s := New(srv.URL)
	err := s.Notify(context.Background(), "test message")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack: unexpected status 500")
}

func TestNotify_BadURL(t *testing.T) {
	s := New("http://[::1]:namedport")
	err := s.Notify(context.Background(), "test message")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack:")
}

func TestNotify_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	s := New(srv.URL)
	err := s.Notify(ctx, "test message")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack: sending request")
}

func TestNotify_UnexpectedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not ok"))
	}))
	defer srv.Close()

	s := New(srv.URL)
	err := s.Notify(context.Background(), "test message")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "slack: unexpected response body")
}
