# CI Pipeline
> Effort: S | Priority: 1 | Depends on: —

## Context

Forge has a `Makefile` with `lint`, `test`, `fmt`, and `vet` targets but no CI pipeline. All quality checks run only when a developer remembers to invoke them locally.

## Problem

- No automated gate on PRs — broken code can be merged
- No shared baseline — "works on my machine" failures
- Blocks all other automation (#02–#07) since they need CI to run in

## Proposed Solution

A single GitHub Actions workflow (`.github/workflows/ci.yml`) that runs on every push to `master` and on all PRs. Steps mirror the existing Makefile targets.

### Configuration

`.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [master]
  pull_request:

permissions:
  contents: read

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Format check
        run: |
          make fmt
          git diff --exit-code || (echo "::error::goimports produced changes; run 'make fmt' locally" && exit 1)

      - name: Vet
        run: make vet

      - name: Lint
        run: make lint

      - name: Test
        run: make test
```

### Makefile Changes

None — the existing targets are sufficient. The `fmt` check uses `git diff --exit-code` to detect uncommitted formatting changes.

### CI Integration

This *is* the CI integration. Later issues add steps to this workflow.

## Acceptance Criteria

- [ ] `.github/workflows/ci.yml` exists and is valid
- [ ] Workflow triggers on push to `master` and on all PRs
- [ ] `make fmt` runs and fails CI if files would change
- [ ] `make vet` runs
- [ ] `make lint` runs (golangci-lint)
- [ ] `make test` runs (with `-race` — already in Makefile)
- [ ] Go module cache is enabled (`setup-go` cache)
- [ ] Workflow uses `go-version-file: go.mod` (no hardcoded version)
- [ ] Green on current `master`

## Dependencies

None. This is the foundation issue.

## References

- [actions/setup-go](https://github.com/actions/setup-go) — Go module caching built-in
- [GitHub Actions workflow syntax](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions)
