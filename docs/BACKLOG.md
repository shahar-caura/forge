# Forge — Backlog

Everything not in V1. Roughly priority-ordered.

---

## V1.1 — Developer Setup

- [x] **direnv for env vars** — `.envrc` with `dotenv` directive loads `.env` automatically

---

## V1.5 — Checkpointing & Resume

- [x] **Run state file** — save progress after each step to `.forge/runs/<run-id>.yaml`
  - Tracks: current step, artifacts (jira_key, branch, worktree_path, pr_url), step history with status/errors
  - ~50 lines Go, just YAML read/write
- [x] **`forge resume <run-id>`** — continue from last failed step
  - Reads state file, skips completed steps, resumes pipeline
- [x] **`forge runs`** — list incomplete/failed runs available for resume
- [x] **Auto-cleanup** — delete state files for successful runs after N days (configurable)

---

## V2 — Parallelism & DAG

- [ ] **DAG scheduler** — toposort + goroutines + channels. ~30 lines Go. No Airflow.
  - `depends_on: [auth, dashboard]` → waits for both PRs to merge before starting
  - Poll `gh pr view --json mergedAt` on each dependency
  - Signal via `chan struct{}` when merged
- [ ] **Multiple plans in one run** — `forge run` reads all plans from config, builds DAG, runs in parallel
- [ ] **Worktree pool** — multiple concurrent worktrees, cleanup on completion
- [x] **Plan file format** — frontmatter parser for title field (Phase 3, plans-v1.md). Extended metadata (id, depends_on, security) deferred to V2.
  ```yaml
  ---
  title: Deploy Server
  ---
  # Plan content...
  ```

## V3 — Model Per Task (OpenRouter)

**Research confirmed:** `claude -p` is fully model-agnostic via env vars. OpenRouter speaks native Anthropic protocol ("Anthropic Skin") — no proxy, no router daemon, no SDK. Just env vars passed to the process.

- [ ] **`agent.env` in config** — per-plan env var overrides passed to `claude -p` via `os/exec`
  ```yaml
  plans:
    - id: boilerplate
      file: plans/boilerplate.md
      agent:
        env:
          ANTHROPIC_BASE_URL: "https://openrouter.ai/api"
          ANTHROPIC_AUTH_TOKEN: "${OPENROUTER_API_KEY}"
          ANTHROPIC_API_KEY: ""            # must be empty to prevent fallback to Anthropic auth
          ANTHROPIC_MODEL: "minimax/minimax-m2.5"
    - id: auth-core
      file: plans/auth.md
      # no env override → uses subscription (free)
  ```
- [ ] **Cost tracking** — log tokens used per task, estimate cost, include in Slack notification
  - OpenRouter returns usage in response headers / response body
  - `claude -p --output-format json` includes token counts in metadata
- [ ] **Model fallback** — if primary model fails/timeouts, retry with fallback model
  - OpenRouter has built-in fallback across providers for the same model
  - For cross-model fallback (e.g., MiniMax → Claude Sonnet), Forge retries with different env vars

**Supported backends (confirmed working with `claude -p`):**

| Backend | `ANTHROPIC_BASE_URL` | Notes |
|---|---|---|
| Claude subscription | _(default)_ | No env vars needed |
| OpenRouter | `https://openrouter.ai/api` | 320+ models, pay-per-token |
| MiniMax direct | `https://api.minimax.io/anthropic` | Cheapest Claude-compatible |
| Self-hosted vLLM/SGLang | `http://localhost:8000` | GPU cost only, full privacy |
| Hugging Face TGI | `https://<endpoint>.aws.endpoints.huggingface.cloud` | OpenAI-compat, auto-scales |
| Any Anthropic-compat API | Custom URL | Works if it speaks Anthropic Messages API |

**Key env vars:**
- `ANTHROPIC_BASE_URL` — endpoint URL
- `ANTHROPIC_AUTH_TOKEN` — API key for the provider
- `ANTHROPIC_API_KEY` — must be set to `""` when using non-Anthropic providers
- `ANTHROPIC_MODEL` — model slug (e.g., `minimax/minimax-m2.5`, `google/gemini-2.5-pro-preview`)
- `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` — cleaner requests, recommended for non-Anthropic

## V4 — Feedback Loop

- [x] **CR feedback loop (single pass)** — poll for bot review comment, agent fixes, push, reply (Phase 4, plans-v1.md)
- [ ] **CR retry loop** — configurable max retries (not just once)
- [ ] **Human-in-the-loop via GitHub** — watch for new comments after "ready for review" notification
  - Poll PR comments for new human comments
  - If human tags `@claude` or adds `forge:fix` label → re-run agent with new feedback
  - If human approves → auto-merge (optional, configurable)
- [ ] **Human-in-the-loop via Slack** — reply to the Slack DM to give instructions
  - Slack Events API → webhook → forge picks up message → re-runs agent
- [ ] **Smart merge** — after human approves, auto-merge + delete branch + cleanup worktree + signal DAG

## V5 — Remote & Mobile Trigger

- [ ] **`forge serve`** — HTTP daemon mode for webhook triggers
- [ ] **Slack slash command** — `/forge run auth` → hits forge server → starts pipeline
- [ ] **Systemd unit** — always-on daemon
- [ ] **Health check endpoint** — `GET /health` for uptime monitoring
- [ ] **Task status endpoint** — `GET /status` returns running tasks as JSON
- [ ] **Telegram bot** — alternative to Slack for mobile trigger (simpler API)

## V6 — Provider Swap

- [ ] **GitLab** — `glab` CLI wrapper, same interface as GitHub
- [ ] **Monday.com** — REST API for issue creation/status
- [ ] **Linear** — GraphQL API
- [ ] **Microsoft Teams** — webhook notifications
- [ ] **Discord** — webhook notifications
- [ ] **Alternative agent runners** (swap the CLI binary, not just the model):
  - Aider (`aider --message "..." --yes`) — open source, model-agnostic
  - OpenHands — full autonomous agent
  - Codex CLI — OpenAI's agent
  - Ralph (`ralph --prompt plan.md --timeout 45`) — autonomous loop with exit detection, circuit breaker, rate limiting. Consider if V1's single `claude -p` call proves unreliable for complex tasks.
  - Custom script — any script that takes a prompt and modifies files

## V7 — Security Levels

- [ ] **Per-plan allowed tools** — declare permissions in plan frontmatter, approved at run start
  ```yaml
  ---
  id: auth
  allowed_tools:
    - Read
    - Write
    - "Bash(npm test:*)"
    - "Bash(npm run:*)"
  needs_approval:
    - "Bash(npm install:*)"  # forge will pause and notify for approval
  ---
  ```
- [ ] **Install approval flow** — dangerous commands (npm install, pip install, etc.) trigger notification
  - Pause run, send Slack notification with package list
  - Wait for approval via Slack reaction or `forge approve <run-id>`
  - Resume from checkpoint after approval
  - Timeout → fail and notify
- [ ] **Per-step allowed tools** — different permissions for different pipeline steps
  ```yaml
  security:
    implement:
      allowed_tools: [Read, Write, "Bash(git:*)"]
      needs_approval: ["Bash(npm install:*)"]
    fix_cr:
      allowed_tools: [Read, Write, "Bash(npm test:*)"]
  ```
- [ ] **Sandbox mode** — run agent in Docker container with limited fs/network
- [ ] **Audit log** — log every command the agent runs, every file it changes
- [ ] **Allowlist/blocklist** — files the agent can/cannot touch

## Future / Maybe

### Jira Board & Sprint Management
- [ ] **Per-plan board override** — plan frontmatter `board_id` overrides global config
- [ ] **`forge boards`** — list available boards from Jira
- [ ] **`forge run --board <id>`** / `--no-board` — CLI flag override for board selection
- [ ] **Auto-detect board from project** — `GET /rest/agile/1.0/board?projectKeyOrId=CAURA`

- [ ] **Web dashboard** — view running tasks, logs, PR status (React + SSE)
- [ ] **Persistent state** — SQLite for task history, retry counts, cost tracking
- [ ] **Self-hosted models** — vLLM/SGLang serving MiniMax M2.5 on own GPU
- [ ] **PR template** — configurable PR body template with plan summary, cost, model used
- [ ] **Plan generator** — give forge a high-level goal, it generates the plan.md first
- [ ] **Multi-repo** — orchestrate across multiple repositories
- [ ] **Merge queue** — if multiple PRs ready, merge in dependency order
- [ ] **Cron trigger** — scheduled runs (e.g., nightly maintenance tasks)
- [ ] **Metrics** — Prometheus metrics for task duration, success rate, cost

---

## Reuse Decisions

| Component | Build or Reuse? | Decision |
|---|---|---|
| Agent runner | **Reuse `claude -p`** | Universal adapter — routes to any Anthropic-compat backend via env vars. Supports subscription, OpenRouter (320+ models), direct APIs, self-hosted. |
| Agent loop (if needed) | **Reuse Ralph** (deferred) | Exit detection, circuit breaker, rate limiting all solved. 6.3k★, MIT, calls `claude -p` internally so same model flexibility applies. Defer unless V1's single-call approach proves unreliable. |
| DAG scheduler | **Build** (~30 lines Go) | Airflow/Temporal are 100x overkill. We need toposort + channels. |
| Git operations | **Reuse `gh`/`glab` CLI** | Battle-tested, handles auth, pagination, edge cases. |
| Jira client | **Build** (net/http) | 2 endpoints. SDK is heavier than the code. |
| Slack notify | **Build** (net/http) | 1 webhook POST. |
| YAML config | **Reuse `gopkg.in/yaml.v3`** | Standard Go YAML lib. |
| CLI framework | **Reuse `cobra`** or just `flag` | V1 is one command, `flag` is fine. Cobra if we add subcommands. |
| Env var expansion | **Build** (~20 lines) | `os.ExpandEnv` + validation. |
| Template engine | **Reuse `text/template`** | For worktree cmd, PR body, etc. |
| Process manager | **Build** (`os/exec`) | Just spawn + wait. No need for a framework. |
| Model routing | **Reuse OpenRouter** (V3) | Anthropic-compat API, 320+ models, built-in fallback. No proxy/daemon — just env vars. |
