# Forge — Test Plan

Test scenarios for forge. When a test is implemented, annotate with `<!-- tested: TestFuncName -->` to track coverage and avoid duplication.

---

## Config (`internal/config`)

- [ ] Load valid `forge.yaml` with all fields populated
- [ ] Resolve `${ENV_VAR}` references from environment
- [ ] Fail on missing required fields
- [ ] Fail on unresolved env var (empty or unset)
- [ ] Fail on unreachable Jira base_url
- [ ] Fail on bad `gh auth status`
- [ ] Fail on missing `claude` binary

## VCS (`internal/provider/vcs`)

- [ ] Create PR via `gh pr create` and return URL
- [ ] Fetch PR review comments filtered by bot usernames
- [ ] Post comment on PR

## Tracker (`internal/provider/tracker`)

- [ ] Create issue and return key
- [ ] Handle auth failure (bad token/email)

## Notifier (`internal/provider/notifier`)

- [ ] Send DM notification via webhook
- [ ] Handle webhook failure (bad URL, non-200)

## Agent (`internal/provider/agent`)

- [ ] Run `claude -p` with plan as prompt in given directory
- [ ] Timeout after configured duration
- [ ] Capture stdout/stderr

## Worktree (`internal/provider/worktree`)

- [ ] Run create_cmd and capture worktree path from stdout
- [ ] Run remove_cmd for cleanup
- [ ] Fail if create_cmd exits non-zero

## Pipeline (`internal/pipeline`)

- [ ] Full happy path: plan file -> Jira -> branch -> agent -> PR -> CR wait -> fix -> push -> notify
- [ ] Fail at step N -> skip remaining steps, notify via Slack with error context
- [ ] Fail at config validation -> exit 1 before any side effects

## E2E Flows

- [ ] `forge run tests/plans/hello-world.md` — creates Jira issue, branch, PR, sends Slack notification
- [ ] `forge run` with missing config -> prints validation errors, exits 1
- [ ] `forge run` with unreachable Jira -> fails fast, notifies Slack
