package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Slack sends notifications via an incoming webhook.
type Slack struct {
	webhookURL string
	client     *http.Client
}

// New returns a Slack notifier for the given webhook URL.
func New(webhookURL string) *Slack {
	return &Slack{
		webhookURL: webhookURL,
		client:     &http.Client{},
	}
}

type webhookPayload struct {
	Text string `json:"text"`
}

// Notify sends a message to the configured Slack webhook.
func (s *Slack) Notify(ctx context.Context, message string) error {
	payload, err := json.Marshal(webhookPayload{Text: message})
	if err != nil {
		return fmt.Errorf("slack: marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("slack: creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("slack: reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: unexpected status %d: %s", resp.StatusCode, body)
	}

	if string(body) != "ok" {
		return fmt.Errorf("slack: unexpected response body: %s", body)
	}

	return nil
}
