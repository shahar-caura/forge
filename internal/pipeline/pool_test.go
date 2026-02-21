package pipeline

import (
	"testing"

	"github.com/shahar-caura/forge/internal/provider"
	"github.com/stretchr/testify/assert"
)

func TestAgentPool_RoundRobin(t *testing.T) {
	a1 := &mockAgent{output: "a1"}
	a2 := &mockAgent{output: "a2"}
	a3 := &mockAgent{output: "a3"}

	pool := NewAgentPool([]provider.Agent{a1, a2, a3}, []string{"claude", "codex", "gemini"})

	assert.Equal(t, a1, pool.Assign(0))
	assert.Equal(t, a2, pool.Assign(1))
	assert.Equal(t, a3, pool.Assign(2))
	assert.Equal(t, a1, pool.Assign(3)) // wraps around
	assert.Equal(t, a2, pool.Assign(4))

	assert.Equal(t, "claude", pool.AssignName(0))
	assert.Equal(t, "codex", pool.AssignName(1))
	assert.Equal(t, "gemini", pool.AssignName(2))
	assert.Equal(t, "claude", pool.AssignName(3))
}

func TestAgentPool_SingleAgent(t *testing.T) {
	a := &mockAgent{output: "only"}
	pool := NewAgentPool([]provider.Agent{a}, []string{"claude"})

	assert.Equal(t, a, pool.Assign(0))
	assert.Equal(t, a, pool.Assign(1))
	assert.Equal(t, a, pool.Assign(5))
	assert.Equal(t, "claude", pool.AssignName(0))
	assert.Equal(t, "claude", pool.AssignName(99))
}

func TestAgentPool_Primary(t *testing.T) {
	a1 := &mockAgent{output: "primary"}
	a2 := &mockAgent{output: "secondary"}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})

	assert.Equal(t, a1, pool.Primary())
}

func TestAgentPool_Len(t *testing.T) {
	a1 := &mockAgent{}
	a2 := &mockAgent{}
	pool := NewAgentPool([]provider.Agent{a1, a2}, []string{"claude", "codex"})
	assert.Equal(t, 2, pool.Len())

	singlePool := NewAgentPool([]provider.Agent{a1}, []string{"claude"})
	assert.Equal(t, 1, singlePool.Len())
}
