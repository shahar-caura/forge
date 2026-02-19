# Contributing to Forge

## Prerequisites

- **Go 1.22+** (see `go.mod` for exact version)
- **golangci-lint** — `brew install golangci-lint`
- **goimports** — `go install golang.org/x/tools/cmd/goimports@latest`
- **git** and **gh** (GitHub CLI)

Optional (for running pipelines, not required for development):
- **claude** (Claude Code CLI) — requires a paid Anthropic subscription. The test suite mocks it.

## Getting Started

```sh
# Fork and clone
gh repo fork shahar-caura/forge --clone
cd forge

# Build
make build

# Run tests
make test

# Run linter
make lint

# Format code
make fmt
```

## Project Structure

See [docs/STRUCTURE.md](docs/STRUCTURE.md) for a full layout.

Key directories:
- `cmd/forge/` — CLI entry point, one file per subcommand
- `internal/config/` — Config loading and validation
- `internal/pipeline/` — Pipeline execution engine
- `internal/provider/` — Provider interfaces and implementations (VCS, Tracker, Notifier, Agent, Worktree)
- `internal/state/` — Run state persistence

## Making Changes

1. Create a feature branch from `master`
2. Make your changes
3. Run `make fmt && make vet && make lint && make test`
4. Commit with a clear message describing the change
5. Open a PR against `master`

## PR Guidelines

- Keep PRs focused — one feature or fix per PR
- Include tests for new functionality
- Update docs if you change CLI behavior or config format
- Reference the GitHub issue number if applicable

## Code Style

- Standard library first — avoid unnecessary dependencies
- Shell out to CLIs (`gh`, `git`, `claude`) over HTTP SDKs
- Wrap errors with context: `fmt.Errorf("step %d: %w", i, err)`
- Fail fast — validate upfront, exit on first error

## Running the Full Pipeline

To run forge end-to-end you need:
1. A `forge.yaml` config (run `forge init`)
2. A plan file (Markdown with YAML frontmatter)
3. `gh auth login` authenticated
4. `claude` CLI installed and authenticated

For development, `make test` covers all provider behavior via mocks.
