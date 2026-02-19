# Branch Protection
> Effort: S | Priority: 7 | Depends on: #02, #03, #05

## Context

Forge's `master` branch has no protection rules. Anyone with write access can push directly, force-push, or merge without CI passing.

## Problem

- Direct pushes to `master` bypass CI entirely
- Force pushes can rewrite history and lose work
- PRs can be merged with failing checks
- No commit message conventions — changelog generation (from #06) won't be useful without structured messages

## Proposed Solution

Configure GitHub branch protection rules on `master` using the `gh` CLI (consistent with Forge's "shell out to CLIs" principle). Add conventional commit enforcement via lefthook.

### Configuration

**Branch protection rules (applied via `gh` CLI):**

```bash
gh api repos/{owner}/{repo}/rulesets \
  --method POST \
  --input - <<'EOF'
{
  "name": "Protect master",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": {
      "include": ["refs/heads/master"],
      "exclude": []
    }
  },
  "rules": [
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 1,
        "dismiss_stale_reviews_on_push": true,
        "require_last_push_approval": false
      }
    },
    {
      "type": "required_status_checks",
      "parameters": {
        "strict_status_checks_policy": true,
        "required_status_checks": [
          { "context": "ci" }
        ]
      }
    },
    {
      "type": "non_fast_forward"
    },
    {
      "type": "required_linear_history"
    }
  ]
}
EOF
```

**Conventional commits — add to `lefthook.yml` (extends #04):**

```yaml
commit-msg:
  commands:
    commitlint:
      run: >
        echo "$(head -1 {1})" | grep -qE '^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?: .{1,72}$' ||
        (echo "Commit message must follow conventional commits: type(scope): description" && exit 1)
```

This uses a simple grep pattern instead of a separate tool — no extra dependency, and it covers the 90% case. The pattern enforces:
- A valid type prefix (feat, fix, docs, etc.)
- Optional scope in parentheses
- A colon + space separator
- A description (1–72 chars on first line)

### Makefile Changes

Add a target to apply branch protection (one-time setup):

```makefile
protect:
	@echo "Applying branch protection rules to master..."
	gh api repos/$(shell gh repo view --json nameWithOwner -q .nameWithOwner)/rulesets \
		--method POST --input .github/ruleset.json
```

Store the ruleset JSON in `.github/ruleset.json` for version control.

### CI Integration

No CI changes — branch protection is a GitHub-side configuration. The `ci` job name from #01 is referenced as a required status check.

## Acceptance Criteria

- [ ] `master` branch requires PR reviews (at least 1 approval)
- [ ] `master` branch requires CI status checks to pass
- [ ] Force pushes to `master` are blocked
- [ ] Linear history is required (squash or rebase merge only)
- [ ] Stale reviews are dismissed on new pushes
- [ ] Commit-msg hook validates conventional commit format
- [ ] Ruleset config is checked into `.github/ruleset.json`
- [ ] `make protect` applies rules via `gh` CLI
- [ ] Rules can be updated by editing the JSON and re-running `make protect`

## Dependencies

| Issue | Why |
|-------|-----|
| #02 Lint & Format | `lint` check must exist as a required status |
| #03 Test Hardening | `test` check must exist as a required status |
| #05 Security Scanning | `vuln` check should exist before gating on it |

## References

- [GitHub rulesets API](https://docs.github.com/en/rest/repos/rules)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [gh api](https://cli.github.com/manual/gh_api) — GitHub CLI API access
