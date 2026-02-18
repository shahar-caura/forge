---
title: "Real V1: Frontmatter, Strict Branches, CR Feedback Loop"
---

# Real V1: Frontmatter, Strict Branches, CR Feedback Loop

## Context

Phases 1-2 built the core pipeline (8 steps: plan → issue → branch → worktree → agent → commit → PR → notify). The pipeline works end-to-end but branch names are `forge/<slug>`, there's no plan metadata, and the pipeline stops after PR creation. "Real V1" adds:

1. **Plan frontmatter** — YAML `title` field parsed from plan files
2. **Strict branch names** — `CAURA-288-deploy-server` matching `^[A-Z]+-[0-9]+(-[a-z0-9]+)+$`
3. **CR feedback loop** — poll for bot review, auto-fix, push, reply

Ralph (`frankbria/ralph-claude-code`) noted for future agent swap — deferred per `docs/APPROACH.md`.

---

## Phase 3: Plan Frontmatter + Strict Branch Names

### 3a. New package: `internal/plan/plan.go`

Simple frontmatter parser (~30 lines). No new deps — reuse `gopkg.in/yaml.v3`.

```go
type Plan struct {
    Title string `yaml:"title"`
    Body  string // everything after closing ---
}

func Parse(content string) (*Plan, error)
```

- Starts with `---\n`? Extract YAML between first two `---` delimiters, unmarshal into Plan, body = remainder.
- No frontmatter? `Plan{Title: "", Body: content}`.
- Unclosed frontmatter? Return error (fail fast).

**Tests** (`internal/plan/plan_test.go`): valid frontmatter, no frontmatter, unclosed, empty title, extra fields ignored, `---` in body after frontmatter.

### 3b. Integrate frontmatter into pipeline

**`internal/pipeline/run.go`**:

- Change `var plan string` to `var parsedPlan *plan.Plan` (and keep `var planBody string` for the prompt).
- Step 0: parse frontmatter after reading file. Use `parsedPlan.Body` as agent prompt, `parsedPlan.Title` for issue/PR titles.
- Step 1 (create issue): use `parsedPlan.Title` instead of `filepath.Base(...)`. Fallback to filename if title empty.
- Step 6 (create PR): use `parsedPlan.Title` for PR title instead of `filepath.Base(...)`.

### 3c. Strict branch naming

**`internal/pipeline/run.go`**:

Replace `BranchName(planPath string) string` with:
```go
func BranchName(issueKey, title string) string  // "CAURA-288" + "Deploy server" → "CAURA-288-deploy-server"
func SlugFromTitle(title string) string          // reusable kebab-case helper
func ValidateBranchName(branch string) error     // validates ^[A-Z]+-[0-9]+(-[a-z0-9]+)+$
```

- Step 2 (generate branch): `BranchName(rs.IssueKey, title)` + validate. When tracker is nil (no issue key), fall back to `forge/<slug>`.
- **`cmd/forge/main.go`**: Run ID generation currently calls `BranchName(planPath)`. Change to inline slug extraction from filename (the old logic without `forge/` prefix) since issue key isn't available yet.

**Tests**: Update `TestBranchName` for new signature. Add `TestValidateBranchName`, `TestSlugFromTitle`, `TestBranchName_NoIssueKey`.

---

## Phase 4: CR Feedback Loop

### 4a. Config: `internal/config/config.go`

Add to `Config`:
```go
type CRConfig struct {
    Enabled        bool     `yaml:"enabled"`          // default false
    PollTimeout    Duration `yaml:"poll_timeout"`      // default 5m
    PollInterval   Duration `yaml:"poll_interval"`     // default 15s
    CommentPattern string   `yaml:"comment_pattern"`   // regex, e.g. "Claude finished"
    FixStrategy    string   `yaml:"fix_strategy"`      // "amend" (default) or "new-commit"
}
```

Validation (only when `enabled: true`): `comment_pattern` required, `fix_strategy` must be `""`, `"amend"`, or `"new-commit"`.

### 4b. VCS interface: `internal/provider/types.go`

Add three methods (note: `Comment` type already exists):
```go
type VCS interface {
    CommitAndPush(ctx, dir, branch, message string) error
    CreatePR(ctx, branch, baseBranch, title, body string) (*PR, error)
    GetPRComments(ctx, prNumber int) ([]Comment, error)        // NEW
    PostPRComment(ctx, prNumber int, body string) error         // NEW
    AmendAndForcePush(ctx, dir, branch string) error            // NEW
}
```

### 4c. VCS implementation: `internal/provider/vcs/github.go`

- `GetPRComments` → `gh api repos/{repo}/issues/{number}/comments` → parse JSON into `[]Comment`
- `PostPRComment` → `gh pr comment {number} --repo {repo} --body {body}`
- `AmendAndForcePush` → `git add . && git commit --amend --no-edit && git push --force-with-lease`

### 4d. Pipeline: 3 new steps (8 → 11 total)

**`internal/state/state.go`** — update `StepNames`:
```go
var StepNames = []string{
    "read plan",        // 0
    "create issue",     // 1
    "generate branch",  // 2
    "create worktree",  // 3
    "run agent",        // 4
    "commit and push",  // 5
    "create pr",        // 6
    "poll cr",          // 7  ← NEW
    "fix cr",           // 8  ← NEW
    "push cr fix",      // 9  ← NEW
    "notify",           // 10
}
```

Add to `RunState`: `CRFeedback string`, `PlanTitle string`.

**`internal/pipeline/run.go`** — new steps:

**Step 7 — poll cr**: If `!cfg.CR.Enabled`, skip (log + return nil). Otherwise poll `GetPRComments` in a loop with `cfg.CR.PollInterval`, match `comment_pattern` regex against comment bodies. Timeout after `cfg.CR.PollTimeout`. Store matched comment body in `rs.CRFeedback`.

**Step 8 — fix cr**: Run agent again: `providers.Agent.Run(ctx, worktreePath, fixPrompt)` where `fixPrompt` includes the CR feedback and original plan body.

**Step 9 — push cr fix**: If `fix_strategy == "amend"` (default), call `AmendAndForcePush`. If `"new-commit"`, call `CommitAndPush`. Then best-effort `PostPRComment` with a summary.

### 4e. Tests

**Pipeline tests** (`run_test.go`): Extend `mockVCS` with 3 new methods. Add:
- `TestRun_CRLoop_HappyPath` — poll finds comment, agent fixes, amend push
- `TestRun_CRLoop_Disabled` — steps 7-9 skipped
- `TestRun_CRLoop_PollTimeout` — no comment found, pipeline fails
- `TestRun_CRLoop_NewCommitStrategy` — uses CommitAndPush instead
- `TestRun_CRLoop_Resume` — resume from step 8 after poll succeeded

**Config tests**: CR config parsing, defaults, validation errors.

**VCS tests** (`github_test.go`): GetPRComments, PostPRComment, AmendAndForcePush.

---

## Phase 5: Docs + Config Updates

- `forge.yaml` — add `cr:` section (disabled by default)
- `.env.example` — no new env vars needed
- `docs/STRUCTURE.md` — add `internal/plan/`
- `docs/BACKLOG.md` — mark CR feedback loop as done, keep Ralph as future
- `tests/TEST_PLAN.md` — annotate new test scenarios

---

## File Change Summary

| File | Change |
|------|--------|
| `internal/plan/plan.go` | **NEW** — frontmatter parser |
| `internal/plan/plan_test.go` | **NEW** — parser tests |
| `internal/config/config.go` | Add `CRConfig` struct + validation |
| `internal/config/config_test.go` | CR config test cases |
| `internal/provider/types.go` | 3 new VCS methods |
| `internal/provider/vcs/github.go` | Implement GetPRComments, PostPRComment, AmendAndForcePush |
| `internal/provider/vcs/github_test.go` | VCS method tests |
| `internal/state/state.go` | 11 steps, new RunState fields |
| `internal/state/state_test.go` | Update step count assertions |
| `internal/pipeline/run.go` | Frontmatter integration, new BranchName, 3 CR steps |
| `internal/pipeline/run_test.go` | Update all step indices, add CR + branch tests |
| `cmd/forge/main.go` | Fix run ID generation |
| `forge.yaml` | Add `cr:` section |
| `docs/STRUCTURE.md`, `docs/BACKLOG.md`, `tests/TEST_PLAN.md` | Doc updates |

## Verification

```bash
make test && go vet ./...
make build

# Test frontmatter parsing:
echo -e "---\ntitle: Hello World\n---\nCreate a hello world server" > tests/plans/hello-world.md
make run 'tests/plans/hello-world.md'
# → Branch should be CAURA-XXX-hello-world, issue title "Hello World"

# Test CR loop (with cr.enabled: true in forge.yaml):
# → After PR, watch logs for "polling for CR comment..."
# → Bot posts review → agent fixes → amend push → reply comment
```

## Implementation Order

1. `internal/plan/` (frontmatter) — independent, no deps
2. `internal/config/config.go` (CRConfig) — independent
3. Pipeline step 0 integration (uses plan package)
4. `BranchName` refactor + `cmd/forge/main.go` run ID fix
5. `internal/provider/types.go` + `vcs/github.go` (3 new VCS methods)
6. `internal/state/state.go` (11 steps, new fields)
7. Pipeline steps 7-9 (CR loop) — depends on 5, 6
8. Update all existing tests for 11-step layout
9. New tests for CR loop + frontmatter + branch naming
10. Docs and config
