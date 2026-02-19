# Forge

Go CLI that executes a development plan file end-to-end, fire-and-forget.

```
forge run plans/auth.md
```

One command: creates a Jira issue, branches, runs Claude Code headless, opens a PR, handles code review feedback, and notifies via Slack.

## Install

### Homebrew (macOS/Linux)

```sh
brew tap shahar-caura/tap
brew install forge
```

### Binary (macOS/Linux)

```sh
curl -fsSL https://raw.githubusercontent.com/shahar-caura/forge/master/install.sh | bash
```

### From source

```sh
go install github.com/shahar-caura/forge/cmd/forge@latest
```

### GitHub Release

```sh
gh release download --repo shahar-caura/forge --pattern '*darwin_arm64*'
tar xzf forge_*_darwin_arm64.tar.gz
sudo mv forge /usr/local/bin/
```

## Prerequisites

- **git** and **gh** (GitHub CLI) — authenticated via `gh auth login`
- **claude** (Claude Code CLI) — requires an Anthropic subscription

> **Note:** `claude` is a paid subscription tool. The test suite mocks it, so contributors can develop without it.

## Quick Start

```sh
# Initialize config
forge init

# Write a plan
cat > plans/my-feature.md <<'EOF'
---
title: Add health check endpoint
---
Add a /healthz endpoint that returns 200 OK with a JSON body.
EOF

# Execute it
forge run plans/my-feature.md

# Check status
forge runs
forge status <run-id>
```

## Commands

| Command | Description |
|---------|-------------|
| `forge init` | Interactive wizard to generate `forge.yaml` |
| `forge run <plan.md>` | Execute a plan file end-to-end |
| `forge push` | Push current branch as a PR |
| `forge resume <run-id>` | Resume a previous run |
| `forge runs` | List all runs |
| `forge status <run-id>` | Show status of a run |
| `forge logs <run-id>` | Show logs for a run |
| `forge steps` | List pipeline steps |
| `forge edit <run-id>` | Open a worktree for manual editing |
| `forge version` | Print version information |

## Configuration

All config lives in `forge.yaml`. Run `forge init` to generate it interactively.

Secrets go in `.forge.env` (project-local) and `~/.config/forge/env` (global). See the [project structure](docs/STRUCTURE.md) for details.

## Development

```sh
git clone https://github.com/shahar-caura/forge.git
cd forge
make build    # Build binary
make test     # Run tests
make lint     # Run linter
make fmt      # Format code
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for more details.

## License

[MIT](LICENSE)
