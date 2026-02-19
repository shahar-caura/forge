package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/shahar-caura/forge/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize forge.yaml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdInit()
		},
	}
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

	// Generate .forge.env files for secrets management.
	if err := generateEnvFiles(data); err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: could not generate env files: %v\n", err)
	}

	return nil
}

// generateEnvFiles creates .forge.env (project) and ~/.config/forge/env (global)
// with placeholder values for secrets referenced in forge.yaml.
func generateEnvFiles(data initData) error {
	// Collect project-local and global env vars.
	var projectLines, globalLines []string

	if data.Tracker {
		projectLines = append(projectLines,
			"# Jira connection",
			fmt.Sprintf("JIRA_URL=%s", data.TrackerBaseURL),
			fmt.Sprintf("JIRA_EMAIL=%s", data.TrackerEmail),
		)
		globalLines = append(globalLines, "JIRA_API_TOKEN=")
	}
	if data.Notifier {
		globalLines = append(globalLines, "SLACK_WEBHOOK_URL=")
	}

	// Write .forge.env (project-local) if there's anything project-specific.
	if len(projectLines) > 0 {
		content := "# Forge project-local environment\n# This file is loaded automatically by forge. Do not commit.\n\n"
		content += strings.Join(projectLines, "\n") + "\n"
		if err := os.WriteFile(".forge.env", []byte(content), 0o600); err != nil {
			return fmt.Errorf("writing .forge.env: %w", err)
		}
		fmt.Println("Wrote .forge.env")
	}

	// Create or skip ~/.config/forge/env (global secrets).
	if len(globalLines) > 0 {
		globalPath := config.GlobalEnvPath()
		if _, err := os.Stat(globalPath); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", filepath.Dir(globalPath), err)
			}
			content := "# Forge global secrets — shared across all projects\n\n"
			content += strings.Join(globalLines, "\n") + "\n"
			if err := os.WriteFile(globalPath, []byte(content), 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", globalPath, err)
			}
			fmt.Printf("Wrote %s\n", globalPath)
		} else {
			fmt.Fprintf(os.Stderr, "\nNote: %s already exists. Ensure these vars are set:\n", globalPath)
			for _, line := range globalLines {
				if k, _, ok := strings.Cut(line, "="); ok {
					fmt.Fprintf(os.Stderr, "  - %s\n", k)
				}
			}
		}
	}

	// Hint about .gitignore.
	if len(projectLines) > 0 {
		fmt.Fprintf(os.Stderr, "\nTip: add .forge.env to .gitignore to avoid committing secrets.\n")
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
