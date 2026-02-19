package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseDeps tests ---

func TestParseDeps_DependsOn(t *testing.T) {
	body := "This task Depends on #10"
	assert.Equal(t, []int{10}, ParseDeps(body))
}

func TestParseDeps_BlockedBy(t *testing.T) {
	body := "Blocked by #5"
	assert.Equal(t, []int{5}, ParseDeps(body))
}

func TestParseDeps_CommaSeparated(t *testing.T) {
	body := "Depends on #1, #2, #3"
	assert.Equal(t, []int{1, 2, 3}, ParseDeps(body))
}

func TestParseDeps_MultiLine(t *testing.T) {
	body := "Depends on #10\nAlso blocked by #20"
	assert.Equal(t, []int{10, 20}, ParseDeps(body))
}

func TestParseDeps_CaseInsensitive(t *testing.T) {
	body := "DEPENDS ON #7\nBLOCKED BY #8"
	assert.Equal(t, []int{7, 8}, ParseDeps(body))
}

func TestParseDeps_Deduplicated(t *testing.T) {
	body := "Depends on #5\nBlocked by #5"
	assert.Equal(t, []int{5}, ParseDeps(body))
}

func TestParseDeps_NoDeps(t *testing.T) {
	body := "Just a regular issue body with no dependencies."
	assert.Empty(t, ParseDeps(body))
}

func TestParseDeps_EmptyBody(t *testing.T) {
	assert.Empty(t, ParseDeps(""))
}

// --- Topsort tests ---

func TestTopsort_NoDeps(t *testing.T) {
	issues := []int{1, 2, 3}
	deps := map[int][]int{}

	levels, err := Topsort(issues, deps)
	require.NoError(t, err)
	require.Len(t, levels, 1)
	assert.Equal(t, []int{1, 2, 3}, levels[0])
}

func TestTopsort_LinearChain(t *testing.T) {
	// 3 depends on 2, 2 depends on 1
	issues := []int{1, 2, 3}
	deps := map[int][]int{
		2: {1},
		3: {2},
	}

	levels, err := Topsort(issues, deps)
	require.NoError(t, err)
	require.Len(t, levels, 3)
	assert.Equal(t, []int{1}, levels[0])
	assert.Equal(t, []int{2}, levels[1])
	assert.Equal(t, []int{3}, levels[2])
}

func TestTopsort_Diamond(t *testing.T) {
	// 2 and 3 depend on 1; 4 depends on 2 and 3
	issues := []int{1, 2, 3, 4}
	deps := map[int][]int{
		2: {1},
		3: {1},
		4: {2, 3},
	}

	levels, err := Topsort(issues, deps)
	require.NoError(t, err)
	require.Len(t, levels, 3)
	assert.Equal(t, []int{1}, levels[0])
	assert.Equal(t, []int{2, 3}, levels[1])
	assert.Equal(t, []int{4}, levels[2])
}

func TestTopsort_Cycle(t *testing.T) {
	issues := []int{10, 11}
	deps := map[int][]int{
		10: {11},
		11: {10},
	}

	_, err := Topsort(issues, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle")
	assert.Contains(t, err.Error(), "#10")
	assert.Contains(t, err.Error(), "#11")
}

func TestTopsort_ExternalDep(t *testing.T) {
	// Issue 2 depends on issue 99 which is not in our set — treated as resolved.
	issues := []int{1, 2}
	deps := map[int][]int{
		2: {99},
	}

	levels, err := Topsort(issues, deps)
	require.NoError(t, err)
	require.Len(t, levels, 1)
	assert.Equal(t, []int{1, 2}, levels[0])
}

func TestTopsort_EmptyInput(t *testing.T) {
	levels, err := Topsort(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, levels)
}

func TestTopsort_ThreeNodeCycle(t *testing.T) {
	issues := []int{1, 2, 3}
	deps := map[int][]int{
		1: {3},
		2: {1},
		3: {2},
	}

	_, err := Topsort(issues, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependency cycle")
}
