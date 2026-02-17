# Forge — Approach

## We Value
- **Working software over comprehensive plans.** Ship V1 in a day.
- **Bash over SDKs.** Shell out to `gh`, `git`, `claude`, `curl`. Less code, fewer deps, easier to debug.
- **Narrow interfaces.** Every provider is one Go interface. Swap by implementing 3-5 methods.
- **Single source of truth.** One `forge.yaml` config. No scattered env vars. All validation in one place.
- **Explicit over magic.** No MCP, no plugins, no runtime discovery. If it's not in the config, it doesn't run.

## We Prefer
- Go for the orchestrator (single binary, goroutines for parallelism later).
- CLI tools over HTTP SDKs (`gh` > `go-github`, `glab` > `go-gitlab`).
- Claude Code subscription (not API) via `claude -p` headless mode.
- Configurable model per task via OpenRouter (later) for cost optimization.
- Reuse over rebuild — but only if the dependency is lighter than the code it replaces.

## We Avoid
- **MCP** — unreliable, hard to debug, unnecessary protocol layer.
- **Airflow / Temporal / heavy schedulers** — need infra, DBs, daemons. Our DAG is 30 lines of Go.
- **Python** — not for the orchestrator. Fine as a dep of tools we shell out to.
- **SDKs with version churn** — `go-github` breaks every quarter. `gh` CLI is stable.
- **Interactive prompts** — forge is fire-and-forget. If it needs input, it fails and notifies.
- **Premature abstraction** — V1 hardcodes the happy path. Interfaces exist for future swaps, not current flexibility.

## Constraints
- Claude Code subscription only (no Anthropic API key). All coding runs via `claude -p`.
- Repo uses `git-crypt` → custom worktree script instead of `git worktree add`.
- Must run while computer is locked → background process / remote server / tmux.
- Notifications only on error or success (no progress spam).

## Model Flexibility

The `claude` CLI is fully model-agnostic. It routes via env vars, not hardcoded endpoints:

```
Forge → claude -p → ANTHROPIC_BASE_URL env var → anywhere
```

This means the same `claude -p` binary supports all of these with zero code changes:

| Scenario | Env Overrides | Cost |
|---|---|---|
| **Claude Code subscription** | None (default) | Free within sub limits |
| **OpenRouter → MiniMax M2.5** | `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_MODEL` | ~$0.30/$1.20 per M tokens |
| **OpenRouter → any of 320+ models** | Same vars, different model slug | Varies |
| **MiniMax direct API** | `ANTHROPIC_BASE_URL=https://api.minimax.io/anthropic` | ~$0.30/$1.20 per M tokens |
| **Self-hosted vLLM/SGLang** | `ANTHROPIC_BASE_URL=http://localhost:8000` | GPU cost only |

OpenRouter speaks native Anthropic protocol via its "Anthropic Skin" — no proxy server, no router daemon, no SDK needed. Just env vars.

V1 uses the subscription (free). V3 adds per-plan model overrides via `agent.env` in forge.yaml, passed to `claude -p` as process environment. The plumbing is trivial: Forge already spawns `claude -p` via `os/exec` — adding env vars to the command is one line of Go.

**Key constraint:** `claude -p` is the universal adapter to models. If we ever want to swap the CLI itself (e.g., for aider or OpenHands), that's a different agent provider — handled by the Agent interface (V6), not by env vars.

## Ralph Consideration

Ralph (`frankbria/ralph-claude-code`, 6.3k★, MIT) solves the autonomous loop problem: exit detection via dual-condition gate, circuit breaker (3 loops no progress → stop), rate limiting, session continuity. It also calls `claude -p` under the hood, so the same model flexibility applies — Ralph doesn't lock us into anything.

However, Ralph adds complexity (bash, tmux, `.ralph/` directory, RALPH_STATUS protocol) that may not be worth it for V1. Our pipeline is already non-interactive and step-based — each `claude -p` call gets a focused prompt (implement plan / fix CR feedback), not an open-ended loop.

**Decision: defer.** If V1's single `claude -p` call per step proves unreliable (stuck loops, no exit, wasted tokens), revisit Ralph as the agent runner. The Agent interface makes this a clean swap.
