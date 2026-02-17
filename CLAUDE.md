# Forge

Go CLI that executes a development plan file end-to-end, fire-and-forget.
`forge run plans/auth.md` creates a Jira issue, branches, runs Claude Code headless, opens a PR, handles CR feedback, and notifies via Slack.

See [docs/STRUCTURE.md](docs/STRUCTURE.md) for project layout.

## Principles

- **Fail fast** — validate everything upfront, exit 1 on any failure, no partial runs.
- **KISS** — no premature abstraction. V1 hardcodes the happy path.
- **DRY** — single source of truth. One config file, one loader, reuse providers across pipeline steps.
- **Test everything** — see [tests/TEST_PLAN.md](tests/TEST_PLAN.md) for scenarios. Annotate implemented tests to avoid duplication.
- **Research before implementing** — reuse existing CLIs/libraries (`gh`, `claude -p`, `curl`) over building from scratch.

## Tech Stack

- **Language:** Go (single binary, standard library first)
- **External CLIs:** `gh`, `git`, `claude -p`, `curl` — shell out over SDKs
- **Deps:** `gopkg.in/yaml.v3`, `text/template`, standard library
- **Avoid:** MCP, Python, heavy schedulers (Airflow/Temporal), SDKs with version churn (`go-github`)

## Conventions

- Each provider role (VCS, Tracker, Notifier, Agent, Worktree) has its own interface in `internal/provider/types.go` and its own subdirectory.
- All config lives in `forge.yaml`. Env vars resolved once at load time — nothing else reads env vars.
- No interactive prompts — if forge needs input, it fails and notifies.
- Shell out to CLIs over HTTP SDKs: `gh` over `go-github`, `git` over `go-git`.
- Errors: wrap with context (`fmt.Errorf("step %d: %w", ...)`), propagate up, notify via Slack on failure.
- Build with `make`. See [Makefile](Makefile) for targets.
