# Batch Run: Project Structure Plan

File-to-issue mapping for `forge run --all-issues`. Used to validate that independent issues don't collide on the same files.

## Dependency Graph

```
#10 CI Pipeline
 ├── #11 Lint & Format
 │    └── #13 Pre-commit Hooks (lefthook)
 ├── #12 Test Hardening (coverage)
 ├── #14 Security Scanning (govulncheck + deadcode)
 ├── #15 Reproducible Builds (GoReleaser)
 └── #16 Branch Protection
      ├── (depends on #11, #12, #14)

#25 DAG Scheduler
 └── #26 Multiple Plans in One Run
      └── #27 Worktree Pool

#28 Per-plan Agent Env Overrides
 ├── #29 Cost Tracking
 └── #30 Model Fallback

#31 CR Retry Loop
 └── #32 Human-in-the-loop (GitHub)
      ├── #33 Human-in-the-loop (Slack)
      └── #34 Auto-merge on Approval

#35 forge serve
 ├── #36 Slack Slash Command Trigger
 ├── #37 Health Check + Task Status
 ├── #38 Telegram Bot Trigger
 ├── #48 Cron Trigger
 ├── #49 Prometheus Metrics
 └── #50 Web Dashboard

#39 Per-plan Allowed Tools
 └── #40 Install Approval Flow

#56 Dependency Graph Parser
 └── #57 Batch Execution from GitHub Issues
      └── #58 forge plan --batch

Standalone (no dependencies):
 #17 GitLab VCS Provider
 #18 Linear Tracker
 #19 Monday.com Tracker
 #20 Discord Notifier
 #21 Teams Notifier
 #22 Aider Agent
 #23 OpenHands Agent
 #24 Codex Agent
 #41 Sandbox Mode (Docker)
 #42 Audit Log
 #43 forge edit Stale Rebase (bug)
 #44 Per-plan Jira Board Override
 #45 Configurable PR Body Template
 #46 Plan Generator (goal -> plan.md)
 #47 Multi-repo Orchestration
 #51 direnv Integration
 #53 Make Run ID Accessible
 #54 Shell Completion for --issue
```

## Issue -> File Mapping

---

### #10 -- CI Pipeline (GitHub Actions)
- **Modifies:** (none -- file already exists at `.github/workflows/ci.yml`)
- **Creates:** `.github/workflows/ci.yml` (if not yet created; currently exists)
- **Depends on:** (none)
- **Notes:** Foundation issue. File already exists in repo. This issue validates/extends it.

---

### #11 -- Lint and Format Expansion
- **Modifies:** `.golangci.yml`, `Makefile`
- **Creates:** (none)
- **Depends on:** #10

---

### #12 -- Test Hardening (coverage enforcement)
- **Modifies:** `.github/workflows/ci.yml`, `Makefile`
- **Creates:** `.testcoverage.yml`
- **Depends on:** #10

---

### #13 -- Pre-commit Hooks (lefthook)
- **Modifies:** `Makefile`
- **Creates:** `lefthook.yml`
- **Depends on:** #11

---

### #14 -- Security Scanning (govulncheck + deadcode)
- **Modifies:** `.github/workflows/ci.yml`, `Makefile`
- **Creates:** (none)
- **Depends on:** #10

---

### #15 -- Reproducible Builds (GoReleaser)
- **Modifies:** `cmd/forge/main.go`, `Makefile`, `.github/workflows/release.yml`, `.goreleaser.yml`
- **Creates:** (none -- files already exist)
- **Depends on:** #10
- **Notes:** `cmd/forge/main.go` version vars and `.goreleaser.yml` already exist. This validates/extends.

---

### #16 -- Branch Protection Rules
- **Modifies:** `Makefile`, `lefthook.yml`
- **Creates:** `.github/ruleset.json`
- **Depends on:** #11, #12, #14

---

### #17 -- GitLab VCS Provider (glab CLI)
- **Modifies:** `internal/config/config.go` (add `gitlab` to VCS provider validation)
- **Creates:** `internal/provider/vcs/gitlab.go`, `internal/provider/vcs/gitlab_test.go`
- **Depends on:** (none)

---

### #18 -- Linear Tracker (GraphQL API)
- **Modifies:** `internal/config/config.go` (add `linear` to tracker provider validation)
- **Creates:** `internal/provider/tracker/linear.go`, `internal/provider/tracker/linear_test.go`
- **Depends on:** (none)

---

### #19 -- Monday.com Tracker (REST API)
- **Modifies:** `internal/config/config.go` (add `monday` to tracker provider validation)
- **Creates:** `internal/provider/tracker/monday.go`, `internal/provider/tracker/monday_test.go`
- **Depends on:** (none)

---

### #20 -- Discord Notifier (webhook)
- **Modifies:** `internal/config/config.go` (add `discord` to notifier provider validation)
- **Creates:** `internal/provider/notifier/discord.go`, `internal/provider/notifier/discord_test.go`
- **Depends on:** (none)

---

### #21 -- Microsoft Teams Notifier (webhook)
- **Modifies:** `internal/config/config.go` (add `teams` to notifier provider validation)
- **Creates:** `internal/provider/notifier/teams.go`, `internal/provider/notifier/teams_test.go`
- **Depends on:** (none)

---

### #22 -- Aider Agent Provider
- **Modifies:** `internal/config/config.go` (add `aider` to agent provider validation)
- **Creates:** `internal/provider/agent/aider.go`, `internal/provider/agent/aider_test.go`
- **Depends on:** (none)

---

### #23 -- OpenHands Agent Provider
- **Modifies:** `internal/config/config.go` (add `openhands` to agent provider validation)
- **Creates:** `internal/provider/agent/openhands.go`, `internal/provider/agent/openhands_test.go`
- **Depends on:** (none)

---

### #24 -- Codex CLI Agent Provider
- **Modifies:** `internal/config/config.go` (add `codex` to agent provider validation)
- **Creates:** `internal/provider/agent/codex.go`, `internal/provider/agent/codex_test.go`
- **Depends on:** (none)

---

### #25 -- DAG Scheduler for Plan Dependencies
- **Modifies:** `internal/plan/plan.go` (extend frontmatter with `depends_on` field)
- **Creates:** `internal/pipeline/dag.go`, `internal/pipeline/dag_test.go`
- **Depends on:** (none)

---

### #26 -- Multiple Plans in One Run
- **Modifies:** `cmd/forge/cmd_run.go`, `internal/pipeline/run.go`, `internal/state/state.go`
- **Creates:** (none)
- **Depends on:** #25

---

### #27 -- Worktree Pool for Concurrent Runs
- **Modifies:** `internal/config/config.go` (add pool size to `WorktreeConfig`)
- **Creates:** `internal/provider/worktree/pool.go`, `internal/provider/worktree/pool_test.go`
- **Depends on:** #26

---

### #28 -- Per-plan Agent Env Var Overrides
- **Modifies:** `internal/plan/plan.go` (extend frontmatter with `env` map), `internal/pipeline/run.go`, `internal/provider/agent/claude.go`
- **Creates:** (none)
- **Depends on:** (none)

---

### #29 -- Cost Tracking (token usage + estimates)
- **Modifies:** `internal/state/state.go` (add cost/token fields), `internal/provider/agent/claude.go` (parse token output), `cmd/forge/cmd_status.go` (display cost)
- **Creates:** (none)
- **Depends on:** #28

---

### #30 -- Model Fallback on Failure/Timeout
- **Modifies:** `internal/config/config.go` (add `agent.fallback` section), `internal/pipeline/run.go` (retry logic), `internal/provider/agent/claude.go` (fallback invocation)
- **Creates:** (none)
- **Depends on:** #28

---

### #31 -- CR Retry Loop (configurable max retries)
- **Modifies:** `internal/config/config.go` (add `cr.max_retries`), `internal/pipeline/run.go` (loop termination), `internal/state/state.go` (retry count tracking)
- **Creates:** (none)
- **Depends on:** (none)

---

### #32 -- Human-in-the-loop via GitHub
- **Modifies:** `internal/pipeline/run.go` (add approval step), `internal/provider/vcs/github.go` (poll for approval comment), `internal/config/config.go` (approval config)
- **Creates:** (none)
- **Depends on:** #31

---

### #33 -- Human-in-the-loop via Slack
- **Modifies:** `internal/provider/notifier/slack.go` (add interactive message support), `internal/pipeline/run.go` (approval via Slack path)
- **Creates:** (none)
- **Depends on:** #32

---

### #34 -- Auto-merge on Approval
- **Modifies:** `internal/provider/vcs/github.go` (add `gh pr merge --auto`), `internal/config/config.go` (add `pr.auto_merge` + merge strategy), `internal/pipeline/run.go` (trigger merge after approval)
- **Creates:** (none)
- **Depends on:** #32

---

### #35 -- forge serve HTTP Daemon Mode
- **Modifies:** `cmd/forge/main.go` (register serve subcommand)
- **Creates:** `cmd/forge/cmd_serve.go`, `internal/server/server.go`, `internal/server/server_test.go`
- **Depends on:** (none)

---

### #36 -- Slack Slash Command Trigger
- **Modifies:** (none)
- **Creates:** `internal/server/slack.go`, `internal/server/slack_test.go`
- **Depends on:** #35

---

### #37 -- Health Check + Task Status Endpoints
- **Modifies:** `internal/server/server.go` (register routes)
- **Creates:** `internal/server/handlers.go`
- **Depends on:** #35

---

### #38 -- Telegram Bot Trigger
- **Modifies:** (none)
- **Creates:** `internal/server/telegram.go`, `internal/server/telegram_test.go`
- **Depends on:** #35

---

### #39 -- Per-plan Allowed Tools in Frontmatter
- **Modifies:** `internal/plan/plan.go` (add `allowed_tools` to frontmatter), `internal/provider/agent/claude.go` (pass tool restrictions), `internal/pipeline/run.go` (enforce restrictions)
- **Creates:** (none)
- **Depends on:** (none)

---

### #40 -- Install Approval Flow
- **Modifies:** `internal/pipeline/run.go` (detect install commands, pause), `internal/config/config.go` (add approval policy config)
- **Creates:** (none)
- **Depends on:** #39

---

### #41 -- Sandbox Mode (Docker Isolation)
- **Modifies:** `internal/config/config.go` (add `agent.sandbox` option)
- **Creates:** `internal/provider/agent/sandbox.go`, `internal/provider/agent/sandbox_test.go`
- **Depends on:** (none)

---

### #42 -- Audit Log (command + file changes)
- **Modifies:** `cmd/forge/cmd_logs.go` (add `--audit` flag)
- **Creates:** `internal/state/audit.go`, `internal/state/audit_test.go`
- **Depends on:** (none)

---

### #43 -- forge edit Rebase Uses Stale Base Branch (bug)
- **Modifies:** `cmd/forge/cmd_edit.go` (fetch latest remote before rebase), `internal/provider/vcs/github.go` (add remote fetch helper)
- **Creates:** (none)
- **Depends on:** (none)

---

### #44 -- Per-plan Jira Board Override
- **Modifies:** `internal/plan/plan.go` (add `board_id` to frontmatter), `internal/pipeline/run.go` (override tracker config per-plan)
- **Creates:** (none)
- **Depends on:** (none)

---

### #45 -- Configurable PR Body Template
- **Modifies:** `internal/config/config.go` (add `vcs.pr_template`), `internal/provider/vcs/github.go` (template rendering for PR body), `internal/pipeline/run.go` (pass template data)
- **Creates:** (none)
- **Depends on:** (none)

---

### #46 -- Plan Generator (goal -> plan.md)
- **Modifies:** `internal/provider/agent/claude.go` (add plan generation prompt mode), `cmd/forge/main.go` (register plan subcommand)
- **Creates:** `cmd/forge/cmd_plan.go`
- **Depends on:** (none)

---

### #47 -- Multi-repo Orchestration
- **Modifies:** `internal/config/config.go` (add `repos` list), `internal/pipeline/run.go` (cross-repo coordination), `internal/provider/vcs/github.go` (multi-repo PR linking)
- **Creates:** (none)
- **Depends on:** (none)

---

### #48 -- Cron Trigger (scheduled runs)
- **Modifies:** `internal/config/config.go` (add `schedule` section)
- **Creates:** `internal/server/cron.go`
- **Depends on:** #35

---

### #49 -- Prometheus Metrics
- **Modifies:** (none)
- **Creates:** `internal/server/metrics.go`
- **Depends on:** #35

---

### #50 -- Web Dashboard (React + SSE)
- **Modifies:** `internal/server/server.go` (serve static files + SSE endpoint)
- **Creates:** `web/` (new directory tree)
- **Depends on:** #35

---

### #51 -- direnv Integration for Env Vars
- **Modifies:** `internal/config/config.go` (direnv detection + fallback), `internal/config/env.go` (add direnv loading path)
- **Creates:** (none)
- **Depends on:** (none)

---

### #53 -- Make Run ID Easily Accessible
- **Modifies:** `cmd/forge/cmd_run.go` (print RUN_ID, write `.forge/last-run-id`), `cmd/forge/cmd_resume.go` (add `--latest` flag)
- **Creates:** (none)
- **Depends on:** (none)
- **Notes:** May also add `forge run list` as a new subcommand; `cmd_runs.go` already exists and covers this.

---

### #54 -- Shell Completion for --issue Flag
- **Modifies:** `cmd/forge/cmd_run.go` (register flag completion func), `cmd/forge/helpers.go` (add issue completion helper)
- **Creates:** (none)
- **Depends on:** (none)

---

### #56 -- Dependency Graph Parser and Topological Sort
- **Modifies:** (none)
- **Creates:** `internal/graph/topsort.go`, `internal/graph/topsort_test.go`
- **Depends on:** (none)

---

### #57 -- Batch Execution from GitHub Issues
- **Modifies:** `cmd/forge/cmd_run.go` (add `--all-issues` flag), `internal/provider/types.go` (add `ListIssues` to VCS interface), `internal/provider/vcs/github.go` (implement `ListIssues`)
- **Creates:** `internal/pipeline/batch.go`, `internal/pipeline/batch_test.go`
- **Depends on:** #56

---

### #58 -- forge plan --batch
- **Modifies:** (none -- `cmd_plan.go` from #46 is a prerequisite in concept, but #58 creates its own version)
- **Creates:** `cmd/forge/cmd_plan.go`, `internal/planner/planner.go`, `internal/planner/planner_test.go`
- **Depends on:** #57

---

## Collision Analysis

Files that appear in multiple **independent** issues (no dependency chain between them) are flagged below. These are potential merge conflict risks if executed in parallel.

### CRITICAL: `internal/config/config.go`

Touched by **17 independent issues**. Nearly every feature adds config fields.

| Issue | Change |
|-------|--------|
| #17 | Add `gitlab` VCS provider |
| #18 | Add `linear` tracker provider |
| #19 | Add `monday` tracker provider |
| #20 | Add `discord` notifier provider |
| #21 | Add `teams` notifier provider |
| #22 | Add `aider` agent provider |
| #23 | Add `openhands` agent provider |
| #24 | Add `codex` agent provider |
| #27 | Add worktree pool size |
| #30 | Add `agent.fallback` |
| #31 | Add `cr.max_retries` |
| #32 | Add approval config |
| #34 | Add `pr.auto_merge` |
| #40 | Add install approval policy |
| #41 | Add `agent.sandbox` |
| #45 | Add `vcs.pr_template` |
| #47 | Add `repos` list |
| #48 | Add `schedule` section |
| #51 | Add direnv detection |

**Mitigation:** Most changes add new fields to different config structs, so textual conflicts are unlikely if changes append rather than modify existing lines. However, parallel execution should serialize config.go modifications or use additive-only patterns.

---

### HIGH: `internal/pipeline/run.go`

Touched by **12 issues**, several of which are independent:

| Issue | Change |
|-------|--------|
| #26 | Multi-plan parent run |
| #28 | Per-plan env override |
| #30 | Model fallback retry |
| #31 | CR retry max |
| #32 | Approval step |
| #33 | Slack approval path |
| #34 | Auto-merge trigger |
| #39 | Tool restriction enforcement |
| #40 | Install detection + pause |
| #44 | Per-plan board override |
| #45 | PR template data |
| #47 | Multi-repo coordination |

**Independent collision pairs:**
- #28 vs #31 (no dependency chain)
- #28 vs #39 (no dependency chain)
- #28 vs #44 (no dependency chain)
- #28 vs #45 (no dependency chain)
- #28 vs #47 (no dependency chain)
- #31 vs #39 (no dependency chain)
- #31 vs #44 (no dependency chain)
- #31 vs #45 (no dependency chain)
- #31 vs #47 (no dependency chain)
- #39 vs #44 (no dependency chain)
- #39 vs #45 (no dependency chain)
- #39 vs #47 (no dependency chain)
- #44 vs #45 (no dependency chain)
- #44 vs #47 (no dependency chain)
- #45 vs #47 (no dependency chain)

**Mitigation:** This file is the central pipeline orchestrator. Changes touch different steps/phases, but structural conflicts are likely. Recommend executing these in dependency-chain batches, not fully parallel.

---

### HIGH: `internal/provider/vcs/github.go`

Touched by **6 issues**, several independent:

| Issue | Change |
|-------|--------|
| #32 | Poll for approval comment |
| #34 | `gh pr merge --auto` |
| #43 | Fetch remote before rebase |
| #45 | PR body template rendering |
| #47 | Multi-repo PR linking |
| #57 | `ListIssues` implementation |

**Independent collision pairs:**
- #43 vs #45 (no dependency chain)
- #43 vs #47 (no dependency chain)
- #43 vs #57 (no dependency chain)
- #45 vs #47 (no dependency chain)
- #45 vs #57 (no dependency chain)
- #47 vs #57 (no dependency chain)

---

### MODERATE: `internal/provider/agent/claude.go`

Touched by **4 issues**, several independent:

| Issue | Change |
|-------|--------|
| #28 | Per-plan env overrides |
| #29 | Token usage parsing |
| #30 | Fallback invocation |
| #39 | Tool restrictions |
| #46 | Plan generation prompt |

**Independent collision pairs:**
- #28/#29/#30 form a chain, but #39 and #46 are independent of them and each other.

---

### MODERATE: `internal/plan/plan.go`

Touched by **4 issues**, all independent:

| Issue | Change |
|-------|--------|
| #25 | Add `depends_on` frontmatter field |
| #28 | Add `env` frontmatter map |
| #39 | Add `allowed_tools` frontmatter list |
| #44 | Add `board_id` frontmatter field |

**All four are independent of each other.** Each adds a new YAML field to the Plan struct. Low textual conflict risk if appended, but the `Parse` function and struct definition will have overlapping edits.

---

### MODERATE: `internal/state/state.go`

Touched by **3 issues**, partially independent:

| Issue | Change |
|-------|--------|
| #26 | Parent/child run tracking |
| #29 | Cost/token fields |
| #31 | Retry count tracking |

**Independent collision pairs:**
- #29 vs #31 (no dependency chain)
- #26 vs #31 (no dependency chain)

---

### MODERATE: `cmd/forge/cmd_run.go`

Touched by **4 issues**, partially independent:

| Issue | Change |
|-------|--------|
| #26 | Multi-plan glob support |
| #53 | Print RUN_ID, write last-run-id |
| #54 | Flag completion func for --issue |
| #57 | Add `--all-issues` flag |

**Independent collision pairs:**
- #53 vs #54 (no dependency chain)
- #53 vs #26 (no dependency chain)
- #54 vs #26 (no dependency chain)

---

### MODERATE: `Makefile`

Touched by **5 issues** in the infra chain:

| Issue | Change |
|-------|--------|
| #11 | Replace goimports with gofumpt in `fmt` |
| #12 | Add `coverage` and `coverage-check` targets |
| #13 | Add `setup` target |
| #14 | Add `vuln` and `deadcode` targets |
| #15 | Update `build` ldflags, add `release-dry` |
| #16 | Add `protect` target |

These are mostly in a dependency chain (#10 -> #11/#12/#14 -> #13/#15/#16), so serial execution within the chain avoids collisions.

---

### MODERATE: `.github/workflows/ci.yml`

Touched by **3 issues**:

| Issue | Change |
|-------|--------|
| #10 | Create/validate baseline |
| #12 | Add coverage step |
| #14 | Add security scanning steps |

All depend on #10, but #12 and #14 are independent of each other.

---

### LOW: `cmd/forge/cmd_plan.go`

Touched by **2 issues**:

| Issue | Change |
|-------|--------|
| #46 | Creates `cmd_plan.go` for `forge plan <goal>` |
| #58 | Creates `cmd_plan.go` for `forge plan --batch` |

**These are independent and both create the same file.** This is a hard collision. Recommend merging #46 and #58 into a single `cmd_plan.go` implementation, or having #58 depend on #46.

---

### LOW: `internal/provider/types.go`

Touched by **1 issue**:

| Issue | Change |
|-------|--------|
| #57 | Add `ListIssues` to VCS interface |

No collision risk -- only one issue modifies it.

---

### LOW: `internal/server/server.go`

Touched by **3 issues**, all in a chain:

| Issue | Change |
|-------|--------|
| #35 | Creates the file |
| #37 | Registers health check routes |
| #50 | Adds static file serving + SSE |

All depend on #35, but #37 and #50 are independent of each other.

---

## Recommended Execution Batches

To minimize collisions, execute issues in these batches:

**Batch 1 -- Foundation (serial):**
#10, #11, #12, #13, #14, #15, #16

**Batch 2 -- Provider expansion (parallel, no shared files):**
#17, #18, #19, #20, #21, #22, #23, #24

**Batch 3 -- Core features (serialize on `run.go`):**
#31, #28, #25, #39, #44, #45

**Batch 4 -- Dependent features (after batch 3):**
#29, #30, #32, #33, #34, #26, #27, #40

**Batch 5 -- Server stack (serial on `server.go`):**
#35, #36, #37, #38, #48, #49, #50

**Batch 6 -- Standalone (parallel):**
#41, #42, #43, #46, #47, #51, #53, #54

**Batch 7 -- Batch run (serial):**
#56, #57, #58
