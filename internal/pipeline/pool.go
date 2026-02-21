package pipeline

import (
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
