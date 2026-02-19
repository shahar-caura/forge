# Pre-commit Hooks
> Effort: S | Priority: 4 | Depends on: #02

## Context

Forge has no local git hooks. Developers can push code that fails CI lint and format checks, wasting a CI round-trip.

## Problem

- Feedback loop is slow — devs push, wait for CI, fix, push again
- Format drift accumulates — `gofumpt`/`goimports` changes pile up
- No local gate prevents committing code that will definitely fail CI

## Proposed Solution

Use [lefthook](https://github.com/evilmartians/lefthook) for git hooks. It's a single Go binary — no Python or Node dependencies. Hooks run in parallel where possible.

### Configuration

`lefthook.yml`:

```yaml
pre-commit:
  parallel: true
  commands:
    format-check:
      glob: "*.go"
      run: >
        gofumpt -l {staged_files} | grep . &&
        echo "Run 'make fmt' to fix formatting" && exit 1 ||
        true
    lint:
      glob: "*.go"
      run: golangci-lint run --new-from-rev=HEAD {staged_files}

pre-push:
  commands:
    test:
      run: make test
```

### Makefile Changes

Add a `setup` target that installs lefthook hooks:

```makefile
setup:
	lefthook install

.PHONY: setup
```

Update the `.PHONY` line at the top to include `setup` and `coverage` targets from #03.

### CI Integration

No CI changes — hooks are local-only. CI is the authoritative gate; hooks are a fast feedback shortcut.

## Acceptance Criteria

- [ ] `lefthook.yml` exists at repo root
- [ ] `make setup` installs hooks via `lefthook install`
- [ ] Pre-commit: `gofumpt` check runs on staged `.go` files
- [ ] Pre-commit: `golangci-lint` runs on staged `.go` files (incremental)
- [ ] Pre-push: `make test` runs
- [ ] Hooks run in parallel where possible
- [ ] Developer docs mention `make setup` as onboarding step
- [ ] Hooks do not block CI (CI does not depend on hooks being installed)

## Dependencies

| Issue | Why |
|-------|-----|
| #02 Lint & Format | Hooks run `gofumpt` and expanded golangci-lint config |

## References

- [lefthook](https://github.com/evilmartians/lefthook) — fast, parallel Git hooks manager
- [lefthook configuration](https://github.com/evilmartians/lefthook/blob/master/docs/configuration.md)
