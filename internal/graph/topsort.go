package graph

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	depPattern      = regexp.MustCompile(`(?i)(?:depends on|blocked by)\s+(#\d+(?:,\s*#\d+)*)`)
	issueNumPattern = regexp.MustCompile(`#(\d+)`)
)

// ParseDeps extracts issue dependencies from a GitHub issue body.
// It looks for "Depends on #N" and "Blocked by #N" patterns (case-insensitive),
// supporting comma-separated lists like "Depends on #1, #2, #3".
// Returns a deduplicated, sorted slice of issue numbers.
func ParseDeps(body string) []int {
	seen := map[int]bool{}
	for _, match := range depPattern.FindAllStringSubmatch(body, -1) {
		for _, numMatch := range issueNumPattern.FindAllStringSubmatch(match[1], -1) {
			n, err := strconv.Atoi(numMatch[1])
			if err != nil {
				continue
			}
			seen[n] = true
		}
	}

	result := make([]int, 0, len(seen))
	for n := range seen {
		result = append(result, n)
	}
	sort.Ints(result)
	return result
}

// Topsort performs a topological sort on the given issues using Kahn's algorithm.
// deps maps each issue to the issues it depends on.
// Dependencies not in the issues set are treated as already resolved (external).
// Returns levels: each level contains issues that can run in parallel.
// Returns an error if a dependency cycle is detected.
func Topsort(issues []int, deps map[int][]int) ([][]int, error) {
	issueSet := make(map[int]bool, len(issues))
	for _, id := range issues {
		issueSet[id] = true
	}

	// Build in-degree map and adjacency list (dep → dependents).
	inDegree := make(map[int]int, len(issues))
	dependents := make(map[int][]int) // dep → list of issues that depend on it
	for _, id := range issues {
		inDegree[id] = 0
	}
	for _, id := range issues {
		for _, dep := range deps[id] {
			if !issueSet[dep] {
				continue // external dep, treated as resolved
			}
			inDegree[id]++
			dependents[dep] = append(dependents[dep], id)
		}
	}

	// BFS from zero-in-degree nodes, level by level.
	var levels [][]int
	queue := make([]int, 0)
	for _, id := range issues {
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
	}
	sort.Ints(queue)

	processed := 0
	for len(queue) > 0 {
		level := queue
		queue = nil
		sort.Ints(level)
		levels = append(levels, level)
		processed += len(level)

		for _, id := range level {
			for _, dep := range dependents[id] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					queue = append(queue, dep)
				}
			}
		}
	}

	if processed != len(issues) {
		// Find cycle for error message.
		return nil, fmt.Errorf("dependency cycle: %s", describeCycle(issues, deps, issueSet, inDegree))
	}

	return levels, nil
}

// describeCycle finds and describes a cycle among the unprocessed nodes.
func describeCycle(issues []int, deps map[int][]int, issueSet map[int]bool, inDegree map[int]int) string {
	// Find an unprocessed node (in-degree > 0).
	var start int
	for _, id := range issues {
		if inDegree[id] > 0 {
			start = id
			break
		}
	}

	// Walk the dependency chain to find the cycle.
	visited := map[int]bool{}
	path := []int{start}
	visited[start] = true
	current := start

	for {
		var next int
		found := false
		for _, dep := range deps[current] {
			if !issueSet[dep] || inDegree[dep] == 0 {
				continue
			}
			next = dep
			found = true
			break
		}
		if !found {
			break
		}
		if visited[next] {
			// Found the cycle — trim path to start from the repeated node.
			cycleStart := -1
			for i, id := range path {
				if id == next {
					cycleStart = i
					break
				}
			}
			cycle := path[cycleStart:]
			cycle = append(cycle, next)
			parts := make([]string, len(cycle))
			for i, id := range cycle {
				parts[i] = fmt.Sprintf("#%d", id)
			}
			return strings.Join(parts, " \u2192 ")
		}
		visited[next] = true
		path = append(path, next)
		current = next
	}

	return fmt.Sprintf("#%d", start)
}
