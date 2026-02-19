# Lint & Format
> Effort: S | Priority: 2 | Depends on: #01

## Context

Forge's `.golangci.yml` (v2 format) enables 5 linters and 2 formatters:

```yaml
linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused

formatters:
  enable:
    - gofmt
    - goimports
```

The `Makefile` runs `goimports -w .` for formatting.

## Problem

- Missing linters that catch real bugs in Go CLI code: unchecked HTTP body closes, missing context propagation, incomplete exhaustive switches, dubious constructs
- `gofmt` allows stylistic variance that `gofumpt` would eliminate (empty lines inside blocks, redundant grouping)
- No linting of security-sensitive patterns (gosec)

## Proposed Solution

Expand the linter set from 5 to ~12 and replace `gofmt` with `gofumpt`.

### Configuration

`.golangci.yml`:

```yaml
version: "2"

linters:
  default: none
  enable:
    # existing
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    # new — bug detection
    - bodyclose       # unclosed HTTP response bodies
    - noctx           # HTTP requests without context
    - exhaustive      # incomplete switch/select on enums
    - nilerr          # returning nil when err != nil
    # new — style & correctness
    - gocritic        # opinionated meta-linter, catches real issues
    - revive          # drop-in golint replacement with configurable rules
    # new — security
    - gosec           # security-oriented static analysis

formatters:
  enable:
    - gofumpt
    - goimports

linters-settings:
  exhaustive:
    default-signifies-exhaustive: true
  gocritic:
    enabled-tags:
      - diagnostic
      - performance
    disabled-checks:
      - hugeParam  # not relevant for a CLI
  revive:
    rules:
      - name: blank-imports
      - name: exported
        arguments: [checkPrivateReceivers]
      - name: unreachable-code
      - name: unused-parameter

issues:
  max-issues-per-linter: 50
  max-same-issues: 5
```

### Makefile Changes

Replace `goimports` with `gofumpt` in the `fmt` target:

```makefile
fmt:
	gofumpt -w .
	goimports -w .
```

Both are needed: `gofumpt` handles formatting, `goimports` handles import ordering. Run `gofumpt` first since `goimports` is a no-op after `gofumpt` for everything except imports.

### CI Integration

No CI changes needed — `make fmt` and `make lint` are already in the CI workflow from #01. The expanded config takes effect automatically.

## Acceptance Criteria

- [ ] `.golangci.yml` enables all listed linters
- [ ] `gofumpt` replaces `gofmt` in formatters section
- [ ] `make fmt` runs `gofumpt -w . && goimports -w .`
- [ ] `make lint` passes on current codebase (fix any new warnings)
- [ ] No regressions — existing lint passes still hold
- [ ] `gofumpt` is documented as a dev dependency (README or CONTRIBUTING)

## Dependencies

| Issue | Why |
|-------|-----|
| #01 CI Pipeline | Lint runs in CI; must have CI first |

## References

- [golangci-lint v2 config](https://golangci-lint.run/usage/configuration/)
- [gofumpt](https://github.com/mvdan/gofumpt) — strict gofmt
- [gosec](https://github.com/securego/gosec) — Go security linter
- [gocritic](https://github.com/go-critic/go-critic) — meta-linter
- [revive](https://github.com/mgechev/revive) — golint replacement
