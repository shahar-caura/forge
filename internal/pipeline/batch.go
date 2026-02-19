package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/graph"
	"github.com/shahar-caura/forge/internal/state"
)

// RunBatch fetches all open issues (optionally filtered by label), topologically
// sorts them by dependency, and executes each in order. Sequential within each
// level for V1.
func RunBatch(ctx context.Context, cfg *config.Config, providers Providers,
	label string, dryRun bool, logger *slog.Logger) error {

	issues, err := providers.VCS.ListIssues(ctx, "open", label)
	if err != nil {
		return fmt.Errorf("listing issues: %w", err)
	}
	if len(issues) == 0 {
		logger.Info("no open issues found")
		return nil
	}

	// Build deps map.
	issueNumbers := make([]int, len(issues))
	depsMap := make(map[int][]int, len(issues))
	titleMap := make(map[int]string, len(issues))
	bodyMap := make(map[int]string, len(issues))
	issueSet := make(map[int]bool, len(issues))
	for i, iss := range issues {
		issueNumbers[i] = iss.Number
		issueSet[iss.Number] = true
		titleMap[iss.Number] = iss.Title
		bodyMap[iss.Number] = iss.Body
		if deps := graph.ParseDeps(iss.Body); len(deps) > 0 {
			depsMap[iss.Number] = deps
		}
	}

	levels, err := graph.Topsort(issueNumbers, depsMap)
	if err != nil {
		return fmt.Errorf("topological sort: %w", err)
	}

	// Dry-run: print execution plan and return.
	if dryRun {
		printPlan(levels, titleMap, logger)
		return nil
	}

	// Execute level by level, parallel within each level.
	completed := 0
	total := len(issueNumbers)
	for li, level := range levels {
		if len(level) == 1 {
			// Single issue — run directly, no goroutine overhead.
			num := level[0]
			logger.Info("running issue", "level", li+1, "issue", num, "title", titleMap[num])
			if err := runSingleIssue(ctx, cfg, providers, num, titleMap[num], bodyMap[num], logger); err != nil {
				reportFailure(ctx, providers, num, err, depsMap, issueSet, logger)
				return fmt.Errorf("issue #%d (%s): %w", num, titleMap[num], err)
			}
			completed++
			logger.Info("issue completed", "issue", num, "progress", fmt.Sprintf("%d/%d", completed, total))
			continue
		}

		// Multiple independent issues — run in parallel.
		logger.Info("running level in parallel", "level", li+1, "issues", level)
		type result struct {
			num int
			err error
		}
		results := make([]result, len(level))
		var wg sync.WaitGroup
		for i, num := range level {
			wg.Add(1)
			go func(i, num int) {
				defer wg.Done()
				logger.Info("running issue", "level", li+1, "issue", num, "title", titleMap[num])
				results[i] = result{num: num, err: runSingleIssue(ctx, cfg, providers, num, titleMap[num], bodyMap[num], logger)}
			}(i, num)
		}
		wg.Wait()

		// Check results — fail fast on first error.
		for _, r := range results {
			if r.err != nil {
				reportFailure(ctx, providers, r.num, r.err, depsMap, issueSet, logger)
				return fmt.Errorf("issue #%d (%s): %w", r.num, titleMap[r.num], r.err)
			}
			completed++
			logger.Info("issue completed", "issue", r.num, "progress", fmt.Sprintf("%d/%d", completed, total))
		}
	}

	logger.Info("batch complete", "completed", completed, "total", total)
	return nil
}

func reportFailure(ctx context.Context, providers Providers, num int, err error, depsMap map[int][]int, issueSet map[int]bool, logger *slog.Logger) {
	logger.Error("issue failed", "issue", num, "error", err)
	blocked := findBlocked(num, depsMap, issueSet)
	if len(blocked) > 0 {
		logger.Warn("blocked downstream issues", "blocked", blocked)
	}
	if providers.Notifier != nil {
		msg := fmt.Sprintf("forge batch: issue #%d failed: %s\nBlocked: %v", num, err, blocked)
		_ = providers.Notifier.Notify(ctx, msg)
	}
}

// runSingleIssue executes a single GitHub issue through the forge pipeline.
func runSingleIssue(ctx context.Context, cfg *config.Config, providers Providers,
	number int, title, body string, logger *slog.Logger) error {

	slug := SlugFromTitle(title)
	runID := time.Now().Format("20060102-150405") + "-" + slug

	// Write temp plan file.
	if err := os.MkdirAll(".forge/runs", 0o755); err != nil {
		return fmt.Errorf("creating runs dir: %w", err)
	}
	planPath := filepath.Join(".forge/runs", runID+"-plan.md")
	planContent := fmt.Sprintf("---\ntitle: %q\n---\n%s\n", title, body)
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		return fmt.Errorf("writing temp plan: %w", err)
	}

	rs := state.New(runID, planPath)
	rs.SourceIssue = number
	if err := rs.Save(); err != nil {
		return fmt.Errorf("saving initial run state: %w", err)
	}

	logger.Info("starting run from issue", "id", runID, "issue", number, "title", title)
	return Run(ctx, cfg, providers, planPath, rs, logger)
}

// printPlan prints the topsorted execution plan.
func printPlan(levels [][]int, titleMap map[int]string, logger *slog.Logger) {
	for i, level := range levels {
		for _, num := range level {
			logger.Info("plan", "level", i+1, "issue", num, "title", titleMap[num])
		}
	}
}

// findBlocked returns issue numbers that transitively depend on the failed issue.
// It builds a reverse dependency graph and walks it from the failed node.
func findBlocked(failed int, depsMap map[int][]int, issueSet map[int]bool) []int {
	// Build reverse map: issue → issues that depend on it.
	dependents := make(map[int][]int)
	for issue, deps := range depsMap {
		for _, dep := range deps {
			if issueSet[dep] {
				dependents[dep] = append(dependents[dep], issue)
			}
		}
	}

	// BFS from failed node.
	visited := map[int]bool{failed: true}
	queue := []int{failed}
	var blocked []int
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, dep := range dependents[cur] {
			if !visited[dep] {
				visited[dep] = true
				blocked = append(blocked, dep)
				queue = append(queue, dep)
			}
		}
	}
	return blocked
}
