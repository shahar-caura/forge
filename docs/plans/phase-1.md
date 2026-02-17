# Phase 1 — Core Loop

Minimal working pipeline: config → worktree → agent → commit → PR.

## Deliverables

```
forge/
├── cmd/forge/main.go
├── internal/
│   ├── config/config.go
│   ├── pipeline/run.go
│   └── provider/
│       ├── types.go
│       ├── vcs/github.go
│       ├── agent/claude.go
│       └── worktree/script.go
├── scripts/
│   └── git-worktree-add.sh      # user-provided
├── forge.yaml
├── Makefile
└── go.mod
```

## Steps (in order)

### 1. Bootstrap
- [ ] `go mod init github.com/user/forge`
- [ ] Create `Makefile` with `build`, `test`, `clean` targets
- [ ] Create `forge.yaml` example config

### 2. Config Loader (`internal/config/config.go`)
- [ ] Struct matching forge.yaml schema (root_branch, worktree, github, agent)
- [ ] Load YAML via `gopkg.in/yaml.v3`
- [ ] Expand `${ENV_VAR}` references via `os.ExpandEnv`
- [ ] Validate: non-empty required fields, `gh auth status`, `claude --version`, worktree script exists

### 3. Provider Types (`internal/provider/types.go`)
- [ ] `type PR struct { URL, Number, Branch string }`
- [ ] Interfaces (keep minimal):
  ```go
  type VCS interface {
      CreatePR(ctx, branch, title, body string) (*PR, error)
  }
  type Agent interface {
      Run(ctx, workdir, prompt string) error
  }
  type Worktree interface {
      Create(ctx, branch, baseBranch string) (path string, err error)
      Remove(ctx, path string) error
  }
  ```

### 4. Worktree Provider (`internal/provider/worktree/script.go`)
- [ ] Shell out to `create_cmd` template with `{{.Branch}}` `{{.BaseBranch}}`
- [ ] Capture stdout as worktree path
- [ ] Shell out to `remove_cmd` for cleanup

### 5. Agent Provider (`internal/provider/agent/claude.go`)
- [ ] Build command: `claude -p <prompt> --allowedTools Read,Write,Bash`
- [ ] Set working directory to worktree path
- [ ] Execute with configurable timeout (default 45m)
- [ ] Stream stdout/stderr to log

### 6. VCS Provider (`internal/provider/vcs/github.go`)
- [ ] `CreatePR`: shell out to `gh pr create --title --body --head --base`
- [ ] Parse PR URL from stdout
- [ ] `CommitAndPush`: shell out to `git add -A && git commit -m && git push -u origin`

### 7. Pipeline (`internal/pipeline/run.go`)
- [ ] `func Run(ctx, cfg *Config, planPath string) error`
- [ ] Steps:
  1. Read plan file content
  2. Generate branch name: `feature/<slug>` from plan filename
  3. Create worktree
  4. Run agent with plan as prompt
  5. Commit and push
  6. Create PR
  7. Cleanup worktree (defer)
- [ ] Wrap errors with step context: `fmt.Errorf("step %d (%s): %w", ...)`

### 8. CLI Entry (`cmd/forge/main.go`)
- [ ] Parse args: `forge run <plan.md>`
- [ ] Load config from `./forge.yaml`
- [ ] Call `pipeline.Run()`
- [ ] Exit 0 on success, 1 on error

## Out of Scope (Phase 2+)
- Jira integration
- CR feedback loop (steps 6-9 of full pipeline)
- Slack notifications
- Retry logic

## Verification

```bash
# Build
make build

# Dry run (with mock plan)
echo "# Test Plan\nAdd a hello.txt file" > plans/test.md
./forge run plans/test.md

# Expected:
# 1. Worktree created at /tmp/forge-xxx or configured path
# 2. Claude runs in worktree
# 3. Changes committed to feature/test branch
# 4. PR created (URL printed)
# 5. Worktree cleaned up
```

## Dependencies

- Go 1.21+
- `gh` CLI authenticated (`gh auth status`)
- `claude` CLI logged in (`claude --version`)
- `scripts/git-worktree-add.sh` in place
