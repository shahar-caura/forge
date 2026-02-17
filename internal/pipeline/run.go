package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/provider"
)

// Providers holds the wired provider implementations for a pipeline run.
type Providers struct {
	VCS      provider.VCS
	Agent    provider.Agent
	Worktree provider.Worktree
}

// Run executes the forge pipeline: read plan → create worktree → run agent → commit → PR.
func Run(ctx context.Context, cfg *config.Config, providers Providers, planPath string, logger *slog.Logger) error {
	// Step 1: Read plan file.
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("step 1 (read plan): %w", err)
	}
	plan := string(planBytes)

	// Step 2: Generate branch name.
	branch := branchName(planPath)
	logger.Info("generated branch name", "branch", branch)

	// Step 3: Create worktree.
	worktreePath, err := providers.Worktree.Create(ctx, branch, cfg.VCS.BaseBranch)
	if err != nil {
		return fmt.Errorf("step 3 (create worktree): %w", err)
	}
	defer func() {
		if worktreePath != "" {
			if cleanupErr := providers.Worktree.Remove(ctx, worktreePath); cleanupErr != nil {
				logger.Error("worktree cleanup failed", "error", cleanupErr)
			}
		}
	}()

	// Step 4: Run agent.
	if err := providers.Agent.Run(ctx, worktreePath, plan); err != nil {
		return fmt.Errorf("step 4 (run agent): %w", err)
	}

	// Step 5: Commit and push.
	commitMsg := fmt.Sprintf("forge: %s", branch)
	if err := providers.VCS.CommitAndPush(ctx, worktreePath, branch, commitMsg); err != nil {
		return fmt.Errorf("step 5 (commit and push): %w", err)
	}

	// Step 6: Create PR.
	title := fmt.Sprintf("forge: %s", filepath.Base(strings.TrimSuffix(planPath, filepath.Ext(planPath))))
	pr, err := providers.VCS.CreatePR(ctx, branch, cfg.VCS.BaseBranch, title, plan)
	if err != nil {
		return fmt.Errorf("step 6 (create PR): %w", err)
	}

	logger.Info("pipeline complete", "pr", pr.URL)
	return nil
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)

// branchName generates a sanitized branch name from a plan file path.
func branchName(planPath string) string {
	base := filepath.Base(planPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ToLower(base)
	base = strings.ReplaceAll(base, " ", "-")
	base = nonAlphanumeric.ReplaceAllString(base, "")
	base = strings.Trim(base, "-")

	if base == "" {
		base = "unnamed"
	}

	return "forge/" + base
}
