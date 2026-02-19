# Forge — Project Structure

```
forge/
├── cmd/forge/                     # CLI entry point (one file per subcommand)
│   ├── main.go                    # main(), newRootCmd(), version vars
│   ├── version.go                 # newVersionCmd()
│   ├── cmd_run.go                 # newRunCmd(), cmdRun()
│   ├── cmd_push.go                # newPushCmd(), cmdPush(), wirePushProviders()
│   ├── cmd_resume.go              # newResumeCmd(), cmdResume()
│   ├── cmd_runs.go                # newRunsCmd(), cmdRuns()
│   ├── cmd_status.go              # newStatusCmd(), cmdStatus()
│   ├── cmd_logs.go                # newLogsCmd(), cmdLogs()
│   ├── cmd_steps.go               # newStepsCmd(), cmdSteps()
│   ├── cmd_edit.go                # newEditCmd(), cmdEdit(), editPush()
│   ├── cmd_init.go                # newInitCmd(), cmdInit(), generateEnvFiles(), templates
│   └── helpers.go                 # completeRunIDs(), wireProviders(), git helpers
├── internal/
│   ├── config/config.go           # Load forge.yaml, resolve env vars, validate
│   ├── plan/plan.go               # Frontmatter parser (title from YAML between --- delimiters)
│   ├── pipeline/run.go            # 11-step pipeline with state tracking + resume + CR loop
│   ├── state/state.go             # Run state persistence (New/Load/Save/List/Cleanup)
│   └── provider/
│       ├── types.go               # Provider interfaces + shared types (PR, Issue, Comment)
│       ├── vcs/github.go          # VCS       — gh CLI wrapper
│       ├── tracker/jira.go        # Tracker   — REST API via net/http
│       ├── notifier/slack.go      # Notifier  — webhook POST
│       ├── agent/claude.go        # Agent     — claude -p wrapper
│       └── worktree/git.go        # Worktree  — template command wrapper (tilde expansion)
├── tests/
│   ├── TEST_PLAN.md               # Test scenarios and coverage tracking
│   └── plans/                     # Example plan files for testing
├── docs/
│   ├── PRD.md                     # V1 requirements and 10-step pipeline
│   ├── APPROACH.md                # Design philosophy and constraints
│   ├── BACKLOG.md                 # V2+ roadmap
│   ├── STRUCTURE.md               # This file
│   └── research/                  # DX tooling proposals (issue-ready docs)
│       ├── 00-overview.md         # Index, dependency graph, execution order
│       ├── 01-ci-pipeline.md      # GitHub Actions CI
│       ├── 02-lint-and-format.md  # golangci-lint v2 expansion + gofumpt
│       ├── 03-test-hardening.md   # Coverage enforcement + race in CI
│       ├── 04-pre-commit-hooks.md # lefthook setup
│       ├── 05-security-scanning.md # govulncheck + deadcode
│       ├── 06-reproducible-builds.md # GoReleaser + version injection
│       └── 07-branch-protection.md   # GitHub rules + conventional commits
├── .github/
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.md          # Bug report template
│   │   └── feature_request.md     # Feature request template
│   ├── pull_request_template.md   # PR template
│   └── workflows/
│       ├── ci.yml                 # CI: fmt, vet, lint, test
│       └── release.yml            # GoReleaser on v* tags
├── .goreleaser.yml                # Cross-platform release config
├── install.sh                     # curl | bash installer
├── forge.yaml                     # Example config (single source of truth)
├── Makefile                       # Build, test, lint, clean, release-dry
├── README.md                      # Install, quick start, commands
├── CONTRIBUTING.md                # Dev setup, PR guidelines
├── CLAUDE.md                      # Instructions for AI agents working on this repo
├── LICENSE                        # MIT
├── go.mod
└── go.sum
```
