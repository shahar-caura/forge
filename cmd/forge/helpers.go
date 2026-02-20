package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/pipeline"
	"github.com/shahar-caura/forge/internal/provider"
	"github.com/shahar-caura/forge/internal/provider/agent"
	"github.com/shahar-caura/forge/internal/provider/notifier"
	"github.com/shahar-caura/forge/internal/provider/tracker"
	"github.com/shahar-caura/forge/internal/provider/vcs"
	"github.com/shahar-caura/forge/internal/provider/worktree"
	"github.com/shahar-caura/forge/internal/state"
	"github.com/spf13/cobra"
)

// --- Dynamic completions ---

func completeRunIDs(toComplete string) ([]string, cobra.ShellCompDirective) {
	matches, err := filepath.Glob(".forge/runs/*.yaml")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var ids []string
	for _, m := range matches {
		id := strings.TrimSuffix(filepath.Base(m), ".yaml")
		if strings.HasPrefix(id, toComplete) {
			ids = append(ids, id)
		}
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}

func completeStepNames(toComplete string) ([]string, cobra.ShellCompDirective) {
	var names []string
	for _, name := range state.StepNames {
		hyphenated := strings.ReplaceAll(name, " ", "-")
		if strings.HasPrefix(hyphenated, toComplete) {
			names = append(names, hyphenated)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeIssueNumbers(toComplete string) ([]string, cobra.ShellCompDirective) {
	out, err := exec.Command("gh", "issue", "list",
		"--state", "open",
		"--limit", "50",
		"--json", "number,title",
	).Output()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var issues []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []string
	for _, issue := range issues {
		s := strconv.Itoa(issue.Number)
		if strings.HasPrefix(s, toComplete) {
			completions = append(completions, fmt.Sprintf("%s\t%s", s, issue.Title))
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// --- Provider wiring ---

func wireProviders(cfg *config.Config, logger *slog.Logger) (pipeline.Providers, error) {
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		return pipeline.Providers{}, fmt.Errorf("resolving repo root: %w", err)
	}

	p := pipeline.Providers{
		Worktree: worktree.New(
			cfg.Worktree.CreateCmd,
			cfg.Worktree.RemoveCmd,
			cfg.Worktree.Cleanup,
			repoRoot,
			logger,
		),
		Agent: newAgent(cfg, logger),
		VCS:   vcs.New(cfg.VCS.Repo, logger),
	}

	if cfg.Tracker.Provider != "" {
		p.Tracker = tracker.New(cfg.Tracker.BaseURL, cfg.Tracker.Project, cfg.Tracker.Email, cfg.Tracker.Token, cfg.Tracker.BoardID)
	}

	if cfg.Notifier.Provider != "" {
		p.Notifier = notifier.New(cfg.Notifier.WebhookURL)
	}

	return p, nil
}

func newAgent(cfg *config.Config, logger *slog.Logger) provider.Agent {
	switch cfg.Agent.Provider {
	case "ralph":
		return agent.NewRalph(cfg.Agent.Timeout.Duration, cfg.Agent.AllowedTools, logger)
	default:
		return agent.New(cfg.Agent.Timeout.Duration, logger)
	}
}

// --- Git helpers ---

// detectDirtyGitState returns a non-empty reason if the worktree is
// mid-rebase or mid-merge, empty string if clean.
func detectDirtyGitState(wtPath string) string {
	// In a worktree, .git is a file ("gitdir: <path>"), not a directory.
	gitDir := filepath.Join(wtPath, ".git")
	if data, err := os.ReadFile(gitDir); err == nil {
		if line := strings.TrimSpace(string(data)); strings.HasPrefix(line, "gitdir: ") {
			gitDir = strings.TrimPrefix(line, "gitdir: ")
			if !filepath.IsAbs(gitDir) {
				gitDir = filepath.Join(wtPath, gitDir)
			}
		}
	}

	markers := []string{"rebase-merge", "rebase-apply", "MERGE_HEAD"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(gitDir, m)); err == nil {
			return m
		}
	}
	return ""
}

func hasUnpushedCommits(wtPath, branch string) bool {
	cmd := exec.Command("git", "rev-list", "--count", "origin/"+branch+"..HEAD")
	cmd.Dir = wtPath
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n > 0
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
