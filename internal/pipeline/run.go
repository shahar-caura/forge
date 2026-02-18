package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/plan"
	"github.com/shahar-caura/forge/internal/provider"
	"github.com/shahar-caura/forge/internal/provider/agent"
	"github.com/shahar-caura/forge/internal/state"
)

// Providers holds the wired provider implementations for a pipeline run.
type Providers struct {
	VCS      provider.VCS
	Agent    provider.Agent
	Worktree provider.Worktree
	Tracker  provider.Tracker  // nil if unconfigured
	Notifier provider.Notifier // nil if unconfigured
}

// Run executes the forge pipeline:
//
//	read plan → create issue → generate branch → create worktree → run agent →
//	commit → PR → poll cr → fix cr → push cr fix → notify.
//
// If rs has completed steps (resume), those steps are skipped and locals are restored from rs artifacts.
func Run(ctx context.Context, cfg *config.Config, providers Providers, planPath string, rs *state.RunState, logger *slog.Logger) error {
	var (
		parsedPlan   *plan.Plan
		planBody     string
		planTitle    string
		branch       string
		worktreePath string
		lastErr      error
	)

	// Restore artifacts from state on resume.
	branch = rs.Branch
	worktreePath = rs.WorktreePath

	// On failure, mark run as failed, preserve worktree, and best-effort notify.
	defer func() {
		if rs.Status != state.RunCompleted {
			rs.Status = state.RunFailed
			_ = rs.Save()

			// Best-effort failure notification — can't fail-fast when already failing.
			if providers.Notifier != nil && lastErr != nil {
				failMsg := fmt.Sprintf("forge pipeline failed: %s", lastErr)
				_ = providers.Notifier.Notify(ctx, failMsg)
			}
		}
	}()

	// Step 0: Read plan file and parse frontmatter.
	if err := runStep(rs, 0, logger, func() error {
		planBytes, err := os.ReadFile(planPath)
		if err != nil {
			return err
		}
		parsedPlan, err = plan.Parse(string(planBytes))
		if err != nil {
			return fmt.Errorf("parsing plan frontmatter: %w", err)
		}
		planBody = parsedPlan.Body
		planTitle = parsedPlan.Title
		rs.PlanTitle = planTitle
		return nil
	}); err != nil {
		lastErr = err
		return err
	}
	// Re-read plan on resume (plan content not stored in state).
	if parsedPlan == nil {
		planBytes, err := os.ReadFile(planPath)
		if err != nil {
			return fmt.Errorf("re-reading plan on resume: %w", err)
		}
		parsedPlan, err = plan.Parse(string(planBytes))
		if err != nil {
			return fmt.Errorf("parsing plan frontmatter on resume: %w", err)
		}
		planBody = parsedPlan.Body
		planTitle = parsedPlan.Title
		if planTitle == "" {
			planTitle = rs.PlanTitle
		}
	}

	// Determine display title: frontmatter title, or fallback to filename.
	displayTitle := planTitle
	if displayTitle == "" {
		displayTitle = TitleFromFilename(filepath.Base(strings.TrimSuffix(planPath, filepath.Ext(planPath))))
	}

	// Step 1: Create issue (optional — skipped if no tracker configured).
	if err := runStep(rs, 1, logger, func() error {
		if providers.Tracker == nil {
			logger.Info("no tracker configured, skipping")
			return nil
		}
		issue, err := providers.Tracker.CreateIssue(ctx, displayTitle, planBody)
		if err != nil {
			return err
		}
		rs.IssueKey = issue.Key
		rs.IssueURL = issue.URL
		logger.Info("created issue", "key", issue.Key, "url", issue.URL)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 2: Generate branch name.
	if err := runStep(rs, 2, logger, func() error {
		branch = BranchName(rs.IssueKey, displayTitle)
		if err := ValidateBranchName(branch); err != nil {
			logger.Warn("branch name validation failed, using as-is", "branch", branch, "error", err)
		}
		rs.Branch = branch
		logger.Info("generated branch name", "branch", branch)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 3: Create worktree.
	worktreeWasCompleted := rs.Steps[3].Status == state.StepCompleted
	if err := runStep(rs, 3, logger, func() error {
		path, err := providers.Worktree.Create(ctx, branch, cfg.VCS.BaseBranch)
		if err != nil {
			return err
		}
		worktreePath = path
		rs.WorktreePath = worktreePath
		return nil
	}); err != nil {
		lastErr = err
		return err
	}
	// On resume with completed worktree step, re-create if cleaned up.
	if worktreeWasCompleted && worktreePath != "" {
		if _, err := os.Stat(worktreePath); err != nil {
			logger.Info("worktree no longer exists, re-creating", "path", worktreePath)
			path, err := providers.Worktree.Create(ctx, branch, cfg.VCS.BaseBranch)
			if err != nil {
				lastErr = fmt.Errorf("step 4 (create worktree): re-creating: %w", err)
				return lastErr
			}
			worktreePath = path
			rs.WorktreePath = worktreePath
			_ = rs.Save()
		}
	}

	// Defer cleanup: remove worktree only on success.
	defer func() {
		if worktreePath == "" {
			return
		}
		if rs.Status == state.RunFailed {
			logger.Info("preserving worktree for resume", "path", worktreePath)
			return
		}
		if cleanupErr := providers.Worktree.Remove(ctx, worktreePath); cleanupErr != nil {
			logger.Error("worktree cleanup failed", "error", cleanupErr)
		}
	}()

	// Step 4: Run agent.
	if err := runStep(rs, 4, logger, func() error {
		logFile, cleanup := openAgentLog(rs.ID, 4, providers.Agent, logger)
		defer cleanup()

		agentPrompt := "Implement the following plan. Make all necessary file changes.\n\n" + planBody
		output, err := providers.Agent.Run(ctx, worktreePath, agentPrompt)
		if logFile == nil {
			saveAgentLog(rs.ID, 4, output)
		}
		if err != nil {
			return err
		}
		// Verify agent produced file changes — fail fast on no-op.
		hasChanges, chkErr := providers.VCS.HasChanges(ctx, worktreePath)
		if chkErr != nil {
			return fmt.Errorf("checking for changes: %w", chkErr)
		}
		if !hasChanges {
			reply := agentResultText(output)
			if len(reply) > 300 {
				reply = reply[:300] + "..."
			}
			return fmt.Errorf("agent produced no file changes; agent replied: %s", reply)
		}
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 5: Commit and push.
	if err := runStep(rs, 5, logger, func() error {
		commitMsg := fmt.Sprintf("forge: %s", displayTitle)
		return providers.VCS.CommitAndPush(ctx, worktreePath, branch, commitMsg)
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 6: Create PR.
	if err := runStep(rs, 6, logger, func() error {
		title := displayTitle
		pr, err := providers.VCS.CreatePR(ctx, branch, cfg.VCS.BaseBranch, title, planBody)
		if err != nil {
			return err
		}
		rs.PRUrl = pr.URL
		rs.PRNumber = pr.Number
		logger.Info("created PR", "pr", pr.URL)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 7: Poll CR (optional — skipped if CR not enabled).
	if err := runStep(rs, 7, logger, func() error {
		if !cfg.CR.Enabled {
			logger.Info("CR feedback loop disabled, skipping")
			return nil
		}
		logger.Info("polling for CR comment...", "pattern", cfg.CR.CommentPattern, "timeout", cfg.CR.PollTimeout.Duration)

		pattern, err := regexp.Compile(cfg.CR.CommentPattern)
		if err != nil {
			return fmt.Errorf("compiling comment_pattern: %w", err)
		}

		deadline := time.Now().Add(cfg.CR.PollTimeout.Duration)
		for {
			if time.Now().After(deadline) {
				return fmt.Errorf("poll timeout: no matching CR comment found after %s", cfg.CR.PollTimeout.Duration)
			}

			comments, err := providers.VCS.GetPRComments(ctx, rs.PRNumber)
			if err != nil {
				return fmt.Errorf("fetching PR comments: %w", err)
			}

			for _, c := range comments {
				if pattern.MatchString(c.Body) {
					logger.Info("matched CR comment", "author", c.Author, "id", c.ID)
					rs.CRFeedback = c.Body
					return nil
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.CR.PollInterval.Duration):
				// continue polling
			}
		}
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 8: Fix CR — re-run agent with CR feedback.
	if err := runStep(rs, 8, logger, func() error {
		if !cfg.CR.Enabled {
			logger.Info("CR feedback loop disabled, skipping")
			return nil
		}

		logFile, cleanup := openAgentLog(rs.ID, 8, providers.Agent, logger)
		defer cleanup()

		feedback := rs.CRFeedback
		fixPrompt := fmt.Sprintf("The following code review feedback was received:\n\n%s\n\nOriginal plan:\n\n%s\n\nPlease address the feedback.\n\nAfter making all changes, output a markdown summary of what you fixed and what (if anything) was left unresolved. Wrap this summary between ---CRSUMMARY--- markers, like:\n\n---CRSUMMARY---\nYour summary here.\n---CRSUMMARY---", feedback, planBody)
		output, err := providers.Agent.Run(ctx, worktreePath, fixPrompt)
		if logFile == nil {
			saveAgentLog(rs.ID, 8, output)
		}
		if err != nil {
			return err
		}
		rs.CRFixSummary = extractCRSummary(output)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 9: Push CR fix.
	if err := runStep(rs, 9, logger, func() error {
		if !cfg.CR.Enabled {
			logger.Info("CR feedback loop disabled, skipping")
			return nil
		}
		if cfg.CR.FixStrategy == "new-commit" {
			commitMsg := fmt.Sprintf("forge: address CR feedback for %s", displayTitle)
			if err := providers.VCS.CommitAndPush(ctx, worktreePath, branch, commitMsg); err != nil {
				return err
			}
		} else {
			// Default: amend.
			if err := providers.VCS.AmendAndForcePush(ctx, worktreePath, branch); err != nil {
				return err
			}
		}
		// Best-effort reply comment — use agent summary if available.
		comment := "CR feedback addressed. Changes pushed."
		if rs.CRFixSummary != "" {
			comment = rs.CRFixSummary
		}
		_ = providers.VCS.PostPRComment(ctx, rs.PRNumber, comment)
		return nil
	}); err != nil {
		lastErr = err
		return err
	}

	// Step 10: Notify (optional — skipped if no notifier configured).
	if err := runStep(rs, 10, logger, func() error {
		if providers.Notifier == nil {
			logger.Info("no notifier configured, skipping")
			return nil
		}
		msg := fmt.Sprintf("PR ready for review: %s", rs.PRUrl)
		if rs.IssueKey != "" {
			msg += fmt.Sprintf(" (issue: %s)", rs.IssueURL)
		}
		return providers.Notifier.Notify(ctx, msg)
	}); err != nil {
		lastErr = err
		return err
	}

	rs.Status = state.RunCompleted
	_ = rs.Save()
	return nil
}

// runStep executes fn for the given step index, skipping if already completed.
// It persists state transitions: pending → running → completed/failed.
func runStep(rs *state.RunState, idx int, logger *slog.Logger, fn func() error) error {
	step := &rs.Steps[idx]

	if step.Status == state.StepCompleted {
		logger.Info("skipping completed step", "step", step.Name)
		return nil
	}

	step.Status = state.StepRunning
	step.Error = ""
	_ = rs.Save()

	if err := fn(); err != nil {
		step.Status = state.StepFailed
		step.Error = err.Error()
		rs.Status = state.RunFailed
		_ = rs.Save()
		return fmt.Errorf("step %d (%s): %w", idx+1, step.Name, err)
	}

	step.Status = state.StepCompleted
	_ = rs.Save()
	return nil
}

// agentResultText extracts the "result" field from `claude -p --output-format json` output.
// Falls back to the raw string if parsing fails or the field is missing.
func agentResultText(output string) string {
	var parsed struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return output
	}
	if parsed.Result == "" {
		return output
	}
	return parsed.Result
}

var crSummaryMarker = "---CRSUMMARY---"

// extractCRSummary extracts the text between ---CRSUMMARY--- markers from agent output.
// Returns empty string if markers are missing or content is empty.
func extractCRSummary(output string) string {
	text := agentResultText(output)
	parts := strings.SplitN(text, crSummaryMarker, 3)
	if len(parts) < 3 {
		return ""
	}
	summary := strings.TrimSpace(parts[1])
	return summary
}

// saveAgentLog writes agent output to .forge/runs/<runID>-agent-step<N>.log for debugging.
func saveAgentLog(runID string, step int, output string) {
	if output == "" {
		return
	}
	dir := ".forge/runs"
	path := filepath.Join(dir, fmt.Sprintf("%s-agent-step%d.log", runID, step))
	_ = os.WriteFile(path, []byte(output), 0o644)
}

// AgentLogPath returns the path to the agent log file for a given run and step.
func AgentLogPath(runID string, step int) string {
	return filepath.Join(".forge/runs", fmt.Sprintf("%s-agent-step%d.log", runID, step))
}

// openAgentLog opens a streaming log file and wires it to the agent's LogWriter.
// Returns the opened file (nil if agent doesn't support streaming) and a cleanup func.
func openAgentLog(runID string, step int, a provider.Agent, logger *slog.Logger) (*os.File, func()) {
	ca, ok := a.(*agent.Claude)
	if !ok {
		return nil, func() {}
	}

	path := AgentLogPath(runID, step)
	f, err := os.Create(path)
	if err != nil {
		logger.Warn("failed to open agent log file, falling back to buffered", "path", path, "error", err)
		return nil, func() {}
	}

	ca.LogWriter = f
	return f, func() {
		ca.LogWriter = nil
		f.Close()
	}
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)
var validBranch = regexp.MustCompile(`^[A-Z]+-[0-9]+(-[a-z0-9]+)+$`)

// SlugFromTitle converts a title string to a kebab-case slug.
func SlugFromTitle(title string) string {
	s := strings.ToLower(title)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "unnamed"
	}
	return s
}

// TitleFromFilename converts a kebab-case or snake_case filename into a Title Cased string.
func TitleFromFilename(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_'
	})
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// BranchName generates a branch name from an issue key and title.
// With an issue key: "CAURA-288-deploy-server". Without: "forge/deploy-server".
func BranchName(issueKey, title string) string {
	slug := SlugFromTitle(title)
	if issueKey == "" {
		return "forge/" + slug
	}
	return issueKey + "-" + slug
}

// ValidateBranchName checks that a branch name matches the strict pattern ^[A-Z]+-[0-9]+(-[a-z0-9]+)+$.
func ValidateBranchName(branch string) error {
	if !validBranch.MatchString(branch) {
		return fmt.Errorf("branch name %q does not match pattern ^[A-Z]+-[0-9]+(-[a-z0-9]+)+$", branch)
	}
	return nil
}
