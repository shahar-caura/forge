package intent

import "errors"

// Result holds the classified intent from a natural language query.
type Result struct {
	Argv       []string `json:"argv"`
	Confidence float64  `json:"confidence"`
	Reasoning  string   `json:"reasoning"`
}

var (
	// ErrNoClaude indicates the claude CLI is not installed.
	ErrNoClaude = errors.New("claude CLI not found in PATH")

	// ErrClassificationFailed indicates the LLM could not classify the input.
	ErrClassificationFailed = errors.New("classification failed")
)
