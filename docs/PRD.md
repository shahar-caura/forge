# Forge — PRD (V1)

## What
A Go CLI that executes a single development plan file end-to-end, fire-and-forget.

## V1 Scope
One command. One plan. One worktree. No DAG, no parallelism, no queue.

```
forge run plans/auth.md
```

This:
1. Reads `forge.yaml` for all config
2. Creates a Jira issue from the plan title/description
3. Runs custom worktree script to create an isolated branch (`feature/<JIRA-KEY>-<slug>`)
4. Spawns `claude -p` headless in the worktree with the plan as prompt
5. Commits, pushes, creates a PR via `gh`
6. Waits for CR bot (~3 min), fetches comments
7. Spawns `claude -p` again to address CR feedback
8. Amends commit, force-pushes
9. Posts response comments on the PR
10. Sends Slack DM: "PR ready for review" + link

Steps 6-9 run once (no infinite loop in V1).

## Fire-and-Forget

`forge run` is a Go binary. It can be:
- Backgrounded: `forge run plans/auth.md &`
- Run in tmux/screen on a remote box
- Triggered via Claude Code `/forge` slash command (which runs the binary and returns)

The Go process spawns `claude -p` as a child process. The parent Go process manages the pipeline. Notifications go to Slack on completion or error. No interactive prompts ever.

### Claude Code Integration

`.claude/commands/forge.md`:
```
Run the forge orchestrator for the plan at $ARGUMENTS:
\`\`\`bash
nohup forge run "$ARGUMENTS" > /tmp/forge-$(date +%s).log 2>&1 &
echo "Forge started in background. Check Slack for updates."
\`\`\`
```

This means: open Claude Code, type `/forge plans/auth.md`, and walk away.

## Config — `forge.yaml`

Single file. All env vars resolved here. Nothing else reads env vars.

```yaml
# forge.yaml
root_branch: dev

# Custom worktree command (supports git-crypt repos)
worktree:
  create_cmd: "./scripts/git-worktree-add.sh {{.Branch}} {{.BaseBranch}}"
  # Must print the worktree path to stdout
  remove_cmd: "git worktree remove --force {{.Path}}"

github:
  # Uses `gh` CLI auth — no token needed if `gh auth status` works

jira:
  base_url: https://yourteam.atlassian.net
  email: ${JIRA_EMAIL}        # resolved from env at load time
  api_token: ${JIRA_API_TOKEN}
  project: PROJ

slack:
  webhook_url: ${SLACK_WEBHOOK_URL}
  user_id: ${SLACK_USER_ID}   # for DMs

agent:
  # V1: Claude Code subscription via `claude -p`
  # No API key needed — uses your logged-in session
  binary: claude               # path to claude CLI
  default_allowed_tools:
    - Read
    - Write
    - Bash
  timeout: 45m
  # V3: per-plan model overrides via env vars passed to `claude -p`
  # env:
  #   ANTHROPIC_BASE_URL: "https://openrouter.ai/api"
  #   ANTHROPIC_AUTH_TOKEN: "${OPENROUTER_API_KEY}"
  #   ANTHROPIC_API_KEY: ""
  #   ANTHROPIC_MODEL: "minimax/minimax-m2.5"

cr:
  wait_seconds: 180            # how long to wait for CR bot
  bot_usernames:               # filter comments by these authors
    - github-actions[bot]
    - coderabbit[bot]
```

### Model Flexibility (V3 plumbing, ready now)

The `claude` CLI routes to any backend via env vars. Forge spawns `claude -p` via `os/exec` — passing extra env vars is one line of Go. V1 uses the default (your subscription). V3 adds `agent.env` in config and per-plan overrides:

```yaml
# V3 example — different models per plan
plans:
  - id: boilerplate
    file: plans/boilerplate.md
    agent:
      env:
        ANTHROPIC_BASE_URL: "https://openrouter.ai/api"
        ANTHROPIC_AUTH_TOKEN: "${OPENROUTER_API_KEY}"
        ANTHROPIC_API_KEY: ""
        ANTHROPIC_MODEL: "minimax/minimax-m2.5"

  - id: auth-core
    file: plans/auth.md
    # no agent.env override → uses subscription (free)
```

No proxy server, no router daemon — OpenRouter's Anthropic-compatible endpoint works natively.

### Config Validation

On startup, `forge` validates:
- All `${ENV_VAR}` references resolve to non-empty values
- `gh auth status` succeeds
- `claude --version` succeeds
- Worktree create_cmd path exists and is executable
- Jira base_url is reachable
- Slack webhook returns 200 on test

If anything fails → print what's wrong, exit 1. No partial runs.

## Project Structure

See [STRUCTURE.md](STRUCTURE.md) for the full layout.

Each provider role (VCS, Tracker, Notifier, Agent, Worktree) has its own interface in `provider/types.go` and its own subdirectory under `internal/provider/`.

## What V1 Does NOT Do
- No DAG / dependency graph
- No parallelism / multiple worktrees at once
- No model selection per task / OpenRouter (plumbing ready, wired in V3)
- No GitLab / Monday / Linear / Teams
- No retry on agent failure (fails → notifies)
- No web UI or dashboard
- No persistent state / database
