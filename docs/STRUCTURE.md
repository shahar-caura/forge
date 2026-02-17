# Forge — Project Structure

```
forge/
├── cmd/forge/main.go              # CLI entry point: run/resume/runs subcommands
├── internal/
│   ├── config/config.go           # Load forge.yaml, resolve env vars, validate
│   ├── pipeline/run.go            # 6-step pipeline with state tracking + resume
│   ├── state/state.go             # Run state persistence (New/Load/Save/List/Cleanup)
│   └── provider/
│       ├── types.go               # Provider interfaces + shared types (PR, Issue, Comment)
│       ├── vcs/github.go          # VCS       — gh CLI wrapper
│       ├── tracker/jira.go        # Tracker   — REST API via net/http
│       ├── notifier/slack.go      # Notifier  — webhook POST
│       ├── agent/claude.go        # Agent     — claude -p wrapper
│       └── worktree/script.go     # Worktree  — custom script wrapper
├── tests/
│   ├── TEST_PLAN.md               # Test scenarios and coverage tracking
│   └── plans/                     # Example plan files for testing
├── docs/
│   ├── PRD.md                     # V1 requirements and 10-step pipeline
│   ├── APPROACH.md                # Design philosophy and constraints
│   ├── BACKLOG.md                 # V2+ roadmap
│   └── STRUCTURE.md               # This file
├── forge.yaml                     # Example config (single source of truth)
├── Makefile                       # Build, test, lint, clean
├── CLAUDE.md                      # Instructions for AI agents working on this repo
├── go.mod
└── go.sum
```
