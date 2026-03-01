package intent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CommandContext is the function used to create exec.Cmd. Override in tests.
var CommandContext = exec.CommandContext

// LookPath is the function used to find executables. Override in tests.
var LookPath = exec.LookPath

// MinConfidence is the minimum confidence score required to accept a classification.
const MinConfidence = 0.5

// Classify interprets a natural language query as a forge command.
func Classify(ctx context.Context, query string) (*Result, error) {
	if _, err := LookPath("claude"); err != nil {
		return nil, ErrNoClaude
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	dc := GatherContext(".")
	prompt := BuildPrompt(query, dc)

	cmd := CommandContext(ctx, "claude", "-p", prompt, "--output-format", "json", "--max-tokens", "256")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%w: timed out after 30s", ErrClassificationFailed)
		}
		return nil, fmt.Errorf("%w: %s", ErrClassificationFailed, stderr.String())
	}

	return parseResponse(stdout.String())
}

// parseResponse extracts a Result from the claude CLI JSON output.
// Handles the JSON envelope ({"result":"..."}) and strips accidental code fences.
func parseResponse(raw string) (*Result, error) {
	// First, try to unwrap claude's JSON envelope.
	text := extractResultField(raw)

	// Strip accidental code fences (```json ... ```).
	text = stripCodeFences(text)
	text = strings.TrimSpace(text)

	var r Result
	if err := json.Unmarshal([]byte(text), &r); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON: %s", ErrClassificationFailed, err)
	}

	if len(r.Argv) == 0 {
		return nil, fmt.Errorf("%w: empty argv", ErrClassificationFailed)
	}

	if r.Confidence < MinConfidence {
		return nil, fmt.Errorf("%w: confidence %.2f below threshold %.2f: %s", ErrClassificationFailed, r.Confidence, MinConfidence, r.Reasoning)
	}

	return &r, nil
}

// extractResultField unwraps claude's {"result":"..."} envelope.
// Falls back to the raw string if parsing fails.
func extractResultField(raw string) string {
	var envelope struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return raw
	}
	if envelope.Result == "" {
		return raw
	}
	return envelope.Result
}

// stripCodeFences removes markdown code fences wrapping JSON.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line.
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence.
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}
