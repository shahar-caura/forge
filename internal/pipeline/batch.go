package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	for i, iss := range issues {
		issueNumbers[i] = iss.Number
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

	// Execute level by level, sequentially within each level.
	completed := 0
	total := len(issueNumbers)
	for li, level := range levels {
		for _, num := range level {
			logger.Info("running issue", "level", li+1, "issue", num, "title", titleMap[num])
			if err := runSingleIssue(ctx, cfg, providers, num, titleMap[num], bodyMap[num], logger); err != nil {
				logger.Error("issue failed", "issue", num, "error", err)
				// Report blocked downstream issues.
				blocked := findBlocked(num, levels, li)
				if len(blocked) > 0 {
					logger.Warn("blocked downstream issues", "blocked", blocked)
				}
				if providers.Notifier != nil {
					msg := fmt.Sprintf("forge batch: issue #%d failed: %s\nBlocked: %v", num, err, blocked)
					_ = providers.Notifier.Notify(ctx, msg)
				}
				return fmt.Errorf("issue #%d (%s): %w", num, titleMap[num], err)
			}
			completed++
			logger.Info("issue completed", "issue", num, "progress", fmt.Sprintf("%d/%d", completed, total))
		}
	}

	logger.Info("batch complete", "completed", completed, "total", total)
	return nil
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

// findBlocked returns issue numbers from remaining levels that transitively
// depend on the failed issue. For V1, reports all issues in later levels.
func findBlocked(failed int, levels [][]int, failedLevel int) []int {
	var blocked []int
	for i := failedLevel; i < len(levels); i++ {
		for _, num := range levels[i] {
			if num != failed {
				blocked = append(blocked, num)
			}
		}
	}
	return blocked
}
