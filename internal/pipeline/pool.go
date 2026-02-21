package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shahar-caura/forge/internal/provider"
)

// AgentPool holds a list of agents for round-robin assignment and fallback.
type AgentPool struct {
	agents []provider.Agent
	names  []string
}

// NewAgentPool creates a pool from agents and their display names.
// agents and names must have the same length and at least one entry.
func NewAgentPool(agents []provider.Agent, names []string) *AgentPool {
	return &AgentPool{agents: agents, names: names}
}

// Assign returns the agent at index % pool size (round-robin).
func (p *AgentPool) Assign(index int) provider.Agent {
	return p.agents[index%len(p.agents)]
}

// AssignName returns the name of the agent at index % pool size.
func (p *AgentPool) AssignName(index int) string {
	return p.names[index%len(p.names)]
}

// Primary returns the first agent in the pool.
func (p *AgentPool) Primary() provider.Agent {
	return p.agents[0]
}

// Len returns the number of agents in the pool.
func (p *AgentPool) Len() int {
	return len(p.agents)
}

// retryableError returns true if the error looks like a rate-limit, quota, or
// credential issue that a different agent might not hit.
func retryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	patterns := []string{
		"rate limit", "429", "quota", "exceeded",
		"unauthorized", "403", "credentials", "timed out",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// RunWithFallback tries the agent at startIdx, falling back to the next agent
// on retryable errors. Returns the output, the name of the agent that
// succeeded, and any final error.
func (p *AgentPool) RunWithFallback(ctx context.Context, startIdx int, dir, prompt string, logger *slog.Logger) (string, string, error) {
	n := len(p.agents)
	for i := 0; i < n; i++ {
		idx := (startIdx + i) % n
		agent := p.agents[idx]
		name := p.names[idx]

		output, err := agent.Run(ctx, dir, prompt)
		if err == nil {
			return output, name, nil
		}

		if !retryableError(err) || i == n-1 {
			return output, name, fmt.Errorf("agent %s: %w", name, err)
		}

		logger.Warn("agent failed with retryable error, trying next",
			"agent", name, "error", err, "next", p.names[(startIdx+i+1)%n])
	}
	// unreachable, but satisfies the compiler
	return "", "", fmt.Errorf("all agents exhausted")
}

// fallbackAgent wraps an AgentPool to satisfy provider.Agent, transparently
// handling fallback on retryable errors. Used in batch mode so the 11-step
// pipeline doesn't need changes.
type fallbackAgent struct {
	pool     *AgentPool
	startIdx int
	logger   *slog.Logger
}

func (f *fallbackAgent) Run(ctx context.Context, dir, prompt string) (string, error) {
	output, _, err := f.pool.RunWithFallback(ctx, f.startIdx, dir, prompt, f.logger)
	return output, err
}

func (f *fallbackAgent) PromptSuffix() string {
	return f.pool.Assign(f.startIdx).PromptSuffix()
}

// NewFallbackAgent creates a provider.Agent that delegates to the pool with fallback.
func NewFallbackAgent(pool *AgentPool, startIdx int, logger *slog.Logger) provider.Agent {
	return &fallbackAgent{pool: pool, startIdx: startIdx, logger: logger}
}
