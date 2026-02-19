package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/shahar-caura/forge/internal/pipeline"
	"github.com/shahar-caura/forge/internal/provider/agent"
	"github.com/shahar-caura/forge/internal/provider/notifier"
	"github.com/shahar-caura/forge/internal/provider/tracker"
	"github.com/shahar-caura/forge/internal/provider/vcs"
	"github.com/shahar-caura/forge/internal/provider/worktree"
	"github.com/shahar-caura/forge/internal/state"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := run(logger); err != nil {
		logger.Error("forge failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: forge <init|run|resume|runs|status|logs|steps|edit|completion>")
	}

	switch os.Args[1] {
	case "init":
		return cmdInit()
	case "run":
		return cmdRun(logger)
	case "resume":
		return cmdResume(logger)
	case "runs":
		return cmdRuns(logger)
	case "status":
		return cmdStatus()
	case "logs":
		return cmdLogs()
	case "steps":
		return cmdSteps()
	case "edit":
		return cmdEdit(logger)
	case "completion":
		return cmdCompletion()
	default:
		return fmt.Errorf("usage: forge <init|run|resume|runs|status|logs|steps|edit|completion>")
	}
}

func cmdRun(logger *slog.Logger) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge run <plan.md>")
	}

	planPath := os.Args[2]

	if _, err := os.Stat(planPath); err != nil {
		return fmt.Errorf("plan file: %w", err)
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Generate run ID: YYYYMMDD-HHMMSS-<slug>
	slug := pipeline.SlugFromTitle(filepath.Base(strings.TrimSuffix(planPath, filepath.Ext(planPath))))
	runID := time.Now().Format("20060102-150405") + "-" + slug

	rs := state.New(runID, planPath)
	if err := rs.Save(); err != nil {
		return fmt.Errorf("saving initial run state: %w", err)
	}

	logger.Info("starting run", "id", runID, "plan", planPath)

	providers, err := wireProviders(cfg, logger)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pipelineErr := pipeline.Run(ctx, cfg, providers, planPath, rs, logger)

	// Best-effort cleanup of old completed runs.
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}

	return pipelineErr
}

func cmdResume(logger *slog.Logger) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge resume <run-id> [--from <step-name>]")
	}

	runID := os.Args[2]

	// Parse optional --from flag.
	var fromStep string
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--from" {
			if i+1 >= len(os.Args) {
				return fmt.Errorf("--from requires a step name")
			}
			fromStep = os.Args[i+1]
			break
		}
	}

	rs, err := state.Load(runID)
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}

	if _, err := os.Stat(rs.PlanPath); err != nil {
		return fmt.Errorf("plan file %q: %w", rs.PlanPath, err)
	}

	if fromStep != "" {
		idx, ok := state.StepIndex(fromStep)
		if !ok {
			return fmt.Errorf("unknown step %q; valid steps: %s", fromStep, strings.Join(state.StepNames, ", "))
		}
		rs.ResetFrom(idx)
		logger.Info("resuming from step", "step", state.StepNames[idx])
	} else {
		if rs.Status == state.RunCompleted {
			return fmt.Errorf("run %q already completed; use --from <step> to re-run from a specific step", runID)
		}
		// Reset failed status to active for re-run.
		rs.Status = state.RunActive
		// Reset any failed steps to pending so they get re-run.
		for i := range rs.Steps {
			if rs.Steps[i].Status == state.StepFailed {
				rs.Steps[i].Status = state.StepPending
				rs.Steps[i].Error = ""
			}
		}
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Info("resuming run", "id", runID, "plan", rs.PlanPath)

	providers, err := wireProviders(cfg, logger)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pipelineErr := pipeline.Run(ctx, cfg, providers, rs.PlanPath, rs, logger)

	// Best-effort cleanup.
	if deleted, err := state.Cleanup(cfg.State.Retention.Duration); err != nil {
		logger.Warn("state cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("cleaned up old run states", "deleted", deleted)
	}

	return pipelineErr
}

func cmdRuns(logger *slog.Logger) error {
	runs, err := state.List()
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	fmt.Printf("%-30s  %-10s  %-20s  %s\n", "ID", "STATUS", "CREATED", "PLAN")
	for _, r := range runs {
		fmt.Printf("%-30s  %-10s  %-20s  %s\n",
			r.ID,
			r.Status,
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.PlanPath,
		)
	}

	_ = logger // unused but kept for consistency
	return nil
}

func cmdSteps() error {
	for i, name := range state.StepNames {
		fmt.Printf("%2d  %s\n", i, name)
	}
	return nil
}

func cmdStatus() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge status <run-id>")
	}

	rs, err := state.Load(os.Args[2])
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}

	elapsed := rs.UpdatedAt.Sub(rs.CreatedAt).Truncate(time.Second)
	if rs.Status == state.RunActive {
		elapsed = time.Since(rs.CreatedAt).Truncate(time.Second)
	}

	fmt.Printf("Run:      %s\n", rs.ID)
	fmt.Printf("Status:   %s\n", rs.Status)
	fmt.Printf("Plan:     %s\n", rs.PlanPath)
	fmt.Printf("Elapsed:  %s\n", elapsed)
	if rs.Branch != "" {
		fmt.Printf("Branch:   %s\n", rs.Branch)
	}
	if rs.PRUrl != "" {
		fmt.Printf("PR:       %s\n", rs.PRUrl)
	}
	fmt.Println()

	fmt.Printf("%-4s  %-20s  %-10s  %s\n", "STEP", "NAME", "STATUS", "ERROR")
	for i, step := range rs.Steps {
		marker := " "
		if step.Status == state.StepRunning {
			marker = ">"
		}
		errMsg := ""
		if step.Error != "" {
			errMsg = step.Error
			if len(errMsg) > 60 {
				errMsg = errMsg[:60] + "..."
			}
		}
		fmt.Printf("%s%3d  %-20s  %-10s  %s\n", marker, i, step.Name, step.Status, errMsg)
	}

	return nil
}

func cmdLogs() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge logs <run-id> [--follow|-f] [--step N]")
	}

	runID := os.Args[2]
	step := 4 // default: agent run step
	follow := false

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--follow", "-f":
			follow = true
		case "--step":
			if i+1 >= len(os.Args) {
				return fmt.Errorf("--step requires a number")
			}
			n, err := strconv.Atoi(os.Args[i+1])
			if err != nil {
				return fmt.Errorf("--step: %w", err)
			}
			step = n
			i++
		}
	}

	logPath := pipeline.AgentLogPath(runID, step)

	if follow {
		cmd := exec.Command("tail", "-f", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("opening log: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(os.Stdout, f)
	return err
}

func cmdEdit(logger *slog.Logger) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge edit <run-id> [push]")
	}

	runID := os.Args[2]
	push := len(os.Args) >= 4 && os.Args[3] == "push"

	rs, err := state.Load(runID)
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}

	if rs.Branch == "" {
		return fmt.Errorf("run %q has no branch yet (step 'generate branch' not completed)", runID)
	}

	cfg, err := config.Load("forge.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	wtPath := rs.WorktreePath

	// If worktree path is empty or directory doesn't exist, re-create it.
	if wtPath == "" || !dirExists(wtPath) {
		repoRoot, err := filepath.Abs(".")
		if err != nil {
			return fmt.Errorf("resolving repo root: %w", err)
		}

		wt := worktree.New(
			cfg.Worktree.CreateCmd,
			cfg.Worktree.RemoveCmd,
			cfg.Worktree.Cleanup,
			repoRoot,
			logger,
		)

		wtPath, err = wt.Create(context.Background(), rs.Branch, cfg.VCS.BaseBranch)
		if err != nil {
			return fmt.Errorf("re-creating worktree: %w", err)
		}

		rs.WorktreePath = wtPath
		if err := rs.Save(); err != nil {
			return fmt.Errorf("saving run state: %w", err)
		}
	}

	// Fast-forward local branch to match remote (agent/CR may have pushed).
	// Skip if worktree is mid-rebase/merge — pull would fail anyway.
	if detectDirtyGitState(wtPath) == "" {
		pull := exec.Command("git", "fetch", "origin", rs.Branch)
		pull.Dir = wtPath
		if _, err := pull.CombinedOutput(); err == nil {
			merge := exec.Command("git", "merge", "--ff-only", "origin/"+rs.Branch)
			merge.Dir = wtPath
			if out, err := merge.CombinedOutput(); err != nil {
				logger.Warn("fast-forward failed (may have local changes)", "error", err, "output", strings.TrimSpace(string(out)))
			}
		}
	}

	if push {
		message := parseFlag(os.Args[4:], "-m")
		if message == "" {
			return fmt.Errorf("usage: forge edit <run-id> push -m \"description of changes\"")
		}
		return editPush(cfg, rs, wtPath, message, logger)
	}

	fmt.Println(wtPath)

	if cfg.Editor.Enabled {
		cmd := exec.Command(cfg.Editor.Command, wtPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("opening editor: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "\nRun 'forge edit %s push -m \"description\"' to commit and update the PR.\n", runID)
	removeHint := strings.Replace(cfg.Worktree.RemoveCmd, "{{.Path}}", wtPath, 1)
	fmt.Fprintf(os.Stderr, "Run '%s' to exit edit mode.\n", removeHint)
	return nil
}

func editPush(cfg *config.Config, rs *state.RunState, wtPath, message string, logger *slog.Logger) error {
	// Fail fast if worktree is mid-rebase or mid-merge.
	if reason := detectDirtyGitState(wtPath); reason != "" {
		return fmt.Errorf("worktree has an unfinished %s; resolve it before pushing", reason)
	}

	ctx := context.Background()
	v := vcs.New(cfg.VCS.Repo, logger)

	hasChanges, err := v.HasChanges(ctx, wtPath)
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}

	if !hasChanges {
		if hasUnpushedCommits(wtPath, rs.Branch) {
			fmt.Fprintf(os.Stderr, "Branch has diverged from remote (rebase?). Push manually:\n\n  git -C %s push --force-with-lease origin %s\n\n", wtPath, rs.Branch)
			return fmt.Errorf("no uncommitted changes but branch has unpushed commits")
		}
		return fmt.Errorf("no changes to push in %s", wtPath)
	}

	switch cfg.CR.FixStrategy {
	case "new-commit":
		msg := fmt.Sprintf("forge: %s", message)
		if err := v.CommitAndPush(ctx, wtPath, rs.Branch, msg); err != nil {
			return fmt.Errorf("commit and push: %w", err)
		}
	default: // "amend"
		// Append -m message to existing commit message before amending.
		logCmd := exec.Command("git", "log", "-1", "--format=%B")
		logCmd.Dir = wtPath
		existing, err := logCmd.Output()
		if err != nil {
			return fmt.Errorf("reading commit message: %w", err)
		}
		newMsg := strings.TrimSpace(string(existing)) + "\n\n" + message
		if err := v.AmendAndForcePushMsg(ctx, wtPath, rs.Branch, newMsg); err != nil {
			return fmt.Errorf("amend and push: %w", err)
		}
	}

	if rs.PRNumber != 0 {
		comment := fmt.Sprintf("Manual edit: %s", message)
		if err := v.PostPRComment(ctx, rs.PRNumber, comment); err != nil {
			logger.Warn("failed to post PR comment", "error", err)
		}
	}

	if rs.PRUrl != "" {
		fmt.Println(rs.PRUrl)
	}

	if cfg.Worktree.Cleanup {
		rm := exec.Command("git", "worktree", "remove", "--force", wtPath)
		if out, err := rm.CombinedOutput(); err != nil {
			logger.Warn("worktree cleanup failed", "error", err, "output", strings.TrimSpace(string(out)))
		} else {
			logger.Info("worktree removed", "path", wtPath)
		}
	}

	return nil
}

func parseFlag(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

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

// cmdInit runs an interactive wizard to generate forge.yaml.
func cmdInit() error {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("checking stdin: %w", err)
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("forge init requires an interactive terminal")
	}

	const configPath = "forge.yaml"
	scanner := bufio.NewScanner(os.Stdin)

	// Overwrite guard.
	if _, err := os.Stat(configPath); err == nil {
		if !promptYesNo(scanner, "forge.yaml already exists. Overwrite?", false) {
			return fmt.Errorf("aborted")
		}
	}

	fmt.Println("Initializing forge.yaml...")

	// === VCS ===
	fmt.Println("\n=== VCS ===")
	vcsProvider := promptString(scanner, "VCS provider", "github")
	repoDefault := detectGitHubRepo()
	repo := promptString(scanner, "Repository (owner/repo)", repoDefault)
	if repo == "" {
		return fmt.Errorf("repository is required")
	}
	baseBranchDefault := detectBaseBranch()
	baseBranch := promptString(scanner, "Base branch", baseBranchDefault)
	if baseBranch == "" {
		return fmt.Errorf("base branch is required")
	}

	// === Agent ===
	fmt.Println("\n=== Agent ===")
	agentProvider := promptString(scanner, "Agent provider", "claude")
	agentTimeout := promptString(scanner, "Agent timeout", "45m")

	// === Worktree ===
	fmt.Println("\n=== Worktree ===")
	createCmd := promptString(scanner, "Create command", "git worktree add {{.Path}} -b {{.Branch}} {{.BaseBranch}}")
	removeCmd := promptString(scanner, "Remove command", "git worktree remove --force {{.Path}}")
	cleanup := promptYesNo(scanner, "Auto-cleanup worktree?", true)

	// === Optional: Tracker ===
	data := initData{
		VCSProvider:   vcsProvider,
		Repo:          repo,
		BaseBranch:    baseBranch,
		AgentProvider: agentProvider,
		AgentTimeout:  agentTimeout,
		CreateCmd:     createCmd,
		RemoveCmd:     removeCmd,
		Cleanup:       cleanup,
	}

	if promptYesNo(scanner, "\nConfigure Jira tracker?", false) {
		fmt.Println("\n=== Tracker (Jira) ===")
		data.Tracker = true
		data.TrackerProject = promptString(scanner, "Project key", "")
		if data.TrackerProject == "" {
			return fmt.Errorf("tracker project key is required")
		}
		data.TrackerBaseURL = promptString(scanner, "Base URL", "")
		if data.TrackerBaseURL == "" {
			return fmt.Errorf("tracker base URL is required")
		}
		data.TrackerEmail = promptString(scanner, "Email", "")
		if data.TrackerEmail == "" {
			return fmt.Errorf("tracker email is required")
		}
		data.TrackerBoardID = promptString(scanner, "Board ID (optional)", "")
	}

	if promptYesNo(scanner, "\nConfigure Slack notifications?", false) {
		fmt.Println("\n=== Notifier (Slack) ===")
		data.Notifier = true
	}

	if promptYesNo(scanner, "\nConfigure code review loop?", false) {
		fmt.Println("\n=== Code Review ===")
		data.CR = true
		data.CRPattern = promptString(scanner, "Comment pattern (regex)", "")
		if data.CRPattern == "" {
			return fmt.Errorf("cr.comment_pattern is required when CR is enabled")
		}
		data.CRStrategy = promptString(scanner, "Fix strategy (amend/new-commit)", "amend")
		data.CRPollTimeout = promptString(scanner, "Poll timeout", "5m")
		data.CRPollInterval = promptString(scanner, "Poll interval", "15s")
	}

	tmpl, err := template.New("forge.yaml").Parse(forgeYAMLTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}

	fmt.Printf("\nWrote %s\n", configPath)

	// Warn about secrets.
	var envVars []string
	if data.Tracker {
		envVars = append(envVars, "JIRA_API_TOKEN")
	}
	if data.Notifier {
		envVars = append(envVars, "SLACK_WEBHOOK_URL")
	}
	if len(envVars) > 0 {
		fmt.Fprintf(os.Stderr, "\nWarning: forge.yaml references these environment variables:\n")
		for _, v := range envVars {
			fmt.Fprintf(os.Stderr, "  - %s\n", v)
		}
		fmt.Fprintf(os.Stderr, "Set them in your shell before running forge.\n")
	}

	return nil
}

type initData struct {
	VCSProvider   string
	Repo          string
	BaseBranch    string
	AgentProvider string
	AgentTimeout  string
	CreateCmd     string
	RemoveCmd     string
	Cleanup       bool

	Tracker        bool
	TrackerProject string
	TrackerBaseURL string
	TrackerEmail   string
	TrackerBoardID string

	Notifier bool

	CR             bool
	CRPattern      string
	CRStrategy     string
	CRPollTimeout  string
	CRPollInterval string
}

const forgeYAMLTemplate = `# Forge configuration
# Environment variables are resolved at load time: ${VAR_NAME}

vcs:
  provider: {{.VCSProvider}}
  repo: {{.Repo}}
  base_branch: {{.BaseBranch}}
{{if .Tracker}}
tracker:
  provider: jira
  project: {{.TrackerProject}}
  base_url: {{.TrackerBaseURL}}
  email: {{.TrackerEmail}}
  token: ${JIRA_API_TOKEN}
{{- if .TrackerBoardID}}
  board_id: {{.TrackerBoardID}}
{{- end}}
{{else}}
# tracker:
#   provider: jira
#   project: PROJ
#   base_url: https://yourco.atlassian.net
#   email: you@company.com
#   token: ${JIRA_API_TOKEN}
#   board_id: 1
{{end}}
{{- if .Notifier}}
notifier:
  provider: slack
  webhook_url: ${SLACK_WEBHOOK_URL}
{{else}}
# notifier:
#   provider: slack
#   webhook_url: ${SLACK_WEBHOOK_URL}
{{end}}
agent:
  provider: {{.AgentProvider}}
  timeout: {{.AgentTimeout}}

worktree:
  create_cmd: "{{.CreateCmd}}"
  remove_cmd: "{{.RemoveCmd}}"
  cleanup: {{.Cleanup}}
{{if .CR}}
cr:
  enabled: true
  comment_pattern: "{{.CRPattern}}"
  fix_strategy: {{.CRStrategy}}
  poll_timeout: {{.CRPollTimeout}}
  poll_interval: {{.CRPollInterval}}
{{else}}
# cr:
#   enabled: true
#   poll_timeout: 5m
#   poll_interval: 15s
#   comment_pattern: "Claude finished"
#   fix_strategy: amend
{{end -}}
`

// detectGitHubRepo parses the "origin" remote URL for owner/repo.
func detectGitHubRepo() string {
	if repo := parseGitHubRemote("origin"); repo != "" {
		return repo
	}
	fmt.Fprintf(os.Stderr, "Warning: could not detect repo from 'origin' remote\n")
	return ""
}

func parseGitHubRemote(name string) string {
	out, err := exec.Command("git", "remote", "get-url", name).Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))

	// SSH: git@github.com:owner/repo.git
	if after, ok := strings.CutPrefix(url, "git@github.com:"); ok {
		return strings.TrimSuffix(after, ".git")
	}
	// HTTPS/SSH-protocol: anything containing github.com/owner/repo
	if _, after, ok := strings.Cut(url, "github.com/"); ok {
		return strings.TrimSuffix(after, ".git")
	}
	return ""
}

// detectBaseBranch determines the default branch from the remote HEAD,
// falling back to checking for main then master.
func detectBaseBranch() string {
	// Try symbolic-ref for remote HEAD.
	if out, err := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD").Output(); err == nil {
		ref := strings.TrimSpace(string(out))
		// refs/remotes/origin/main -> main
		if parts := strings.Split(ref, "/"); len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Fall back: check if main or master exist.
	for _, branch := range []string{"main", "master"} {
		if err := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch).Run(); err == nil {
			return branch
		}
	}
	return "main"
}

func promptString(scanner *bufio.Scanner, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal
	}
	return input
}

func promptYesNo(scanner *bufio.Scanner, label string, defaultYes bool) bool {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	fmt.Printf("%s %s: ", label, hint)
	scanner.Scan()
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

func cmdCompletion() error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: forge completion <zsh|bash>")
	}
	switch os.Args[2] {
	case "zsh":
		fmt.Print(zshCompletion())
	case "bash":
		fmt.Print(bashCompletion())
	default:
		return fmt.Errorf("unsupported shell %q; supported: zsh, bash", os.Args[2])
	}
	return nil
}

func stepNamesHyphenated() []string {
	out := make([]string, len(state.StepNames))
	for i, name := range state.StepNames {
		out[i] = strings.ReplaceAll(name, " ", "-")
	}
	return out
}

func zshCompletion() string {
	steps := stepNamesHyphenated()
	return `_forge() {
  emulate -L zsh

  local -a subcmds
  subcmds=(init run resume runs status logs steps edit completion)

  local -a steps
  steps=(` + strings.Join(steps, " ") + `)

  local -a run_ids
  run_ids=(${(f)"$(for f in .forge/runs/*.yaml(N); do basename $f .yaml; done)"})

  if (( CURRENT == 2 )); then
    _describe 'command' subcmds
    return
  fi

  case "${words[2]}" in
    run)
      _files -g '*.md'
      ;;
    resume)
      if [[ "${words[CURRENT-1]}" == "--from" || "${words[CURRENT-1]}" == "--f" ]]; then
        _describe 'step' steps
      elif (( CURRENT == 3 )); then
        _describe 'run-id' run_ids
      else
        compadd -- --from
      fi
      ;;
    status)
      if (( CURRENT == 3 )); then
        _describe 'run-id' run_ids
      fi
      ;;
    edit)
      if (( CURRENT == 3 )); then
        _describe 'run-id' run_ids
      elif (( CURRENT == 4 )); then
        compadd -- push
      fi
      ;;
    logs)
      if [[ "${words[CURRENT-1]}" == "--step" ]]; then
        local -a step_nums=(4 8)
        _describe 'step-number' step_nums
      elif (( CURRENT == 3 )); then
        _describe 'run-id' run_ids
      else
        compadd -- --follow -f --step
      fi
      ;;
    completion)
      local -a shells=(zsh bash)
      _describe 'shell' shells
      ;;
  esac
}

compdef _forge forge
`
}

func bashCompletion() string {
	steps := stepNamesHyphenated()
	return `_forge() {
  local cur prev subcmds steps run_ids
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  subcmds="init run resume runs status logs steps edit completion"
  steps="` + strings.Join(steps, " ") + `"
  run_ids=$(ls .forge/runs/*.yaml 2>/dev/null | xargs -I{} basename {} .yaml)

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${subcmds}" -- "${cur}") )
    return
  fi

  case "${COMP_WORDS[1]}" in
    run)
      COMPREPLY=( $(compgen -f -X '!*.md' -- "${cur}") )
      ;;
    resume)
      if [[ "${prev}" == "--from" ]]; then
        COMPREPLY=( $(compgen -W "${steps}" -- "${cur}") )
      elif [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${run_ids}" -- "${cur}") )
      else
        COMPREPLY=( $(compgen -W "--from" -- "${cur}") )
      fi
      ;;
    status)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${run_ids}" -- "${cur}") )
      fi
      ;;
    edit)
      if [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${run_ids}" -- "${cur}") )
      elif [[ ${COMP_CWORD} -eq 3 ]]; then
        COMPREPLY=( $(compgen -W "push" -- "${cur}") )
      fi
      ;;
    logs)
      if [[ "${prev}" == "--step" ]]; then
        COMPREPLY=( $(compgen -W "4 8" -- "${cur}") )
      elif [[ ${COMP_CWORD} -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "${run_ids}" -- "${cur}") )
      else
        COMPREPLY=( $(compgen -W "--follow -f --step" -- "${cur}") )
      fi
      ;;
    completion)
      COMPREPLY=( $(compgen -W "zsh bash" -- "${cur}") )
      ;;
  esac
}

complete -F _forge forge
`
}

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
		Agent: agent.New(cfg.Agent.Timeout.Duration, logger),
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
