# Test Hardening
> Effort: M | Priority: 3 | Depends on: #01

## Context

Forge runs `go test -race ./...` via `make test`. There is no coverage tracking, no coverage threshold enforcement, and no coverage reporting in CI.

## Problem

- No visibility into test coverage — impossible to know which packages lack tests
- No ratchet mechanism — coverage can silently regress
- Race detection runs locally but there's no CI enforcement yet (addressed by #01, but coverage is not)

## Proposed Solution

Add coverage enforcement via `go-test-coverage` with a config-driven threshold, a `make coverage` target, and coverage artifact upload in CI.

### Configuration

`.testcoverage.yml`:

```yaml
# go-test-coverage configuration
# https://github.com/vladopajic/go-test-coverage

profile: coverage.out

threshold:
  # Start with measured baseline, ratchet up over time.
  # Run `go test -coverprofile=coverage.out ./...` to measure.
  total: 40

  # Per-package minimum (0 = no per-package gate yet).
  per-package: 0

# Exclude generated files and test helpers.
exclude:
  paths:
    - cmd/forge/main\.go  # CLI wiring, hard to unit-test
```

### Makefile Changes

```makefile
coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-check: coverage
	go-test-coverage --config .testcoverage.yml
```

Update `clean` to include coverage:

```makefile
clean:
	rm -rf $(BUILD_DIR) coverage.out
```

(Already there — no change needed for `clean`.)

### CI Integration

Add to `.github/workflows/ci.yml` after the Test step:

```yaml
      - name: Test with coverage
        run: go test -race -coverprofile=coverage.out ./...

      - name: Coverage check
        run: go-test-coverage --config .testcoverage.yml

      - name: Upload coverage artifact
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out
          retention-days: 14
```

Replace the existing `make test` step with the `Test with coverage` step above so tests only run once.

Install `go-test-coverage` in CI:

```yaml
      - name: Install go-test-coverage
        run: go install github.com/vladopajic/go-test-coverage/v2@latest
```

## Acceptance Criteria

- [ ] `.testcoverage.yml` exists with a measured baseline threshold
- [ ] `make coverage` generates `coverage.out` and prints per-function coverage
- [ ] `make coverage-check` fails if total coverage drops below threshold
- [ ] CI runs coverage check and uploads `coverage.out` as artifact
- [ ] Race detection is enabled in CI test step (`-race`)
- [ ] Baseline threshold is set to current measured coverage (rounded down to nearest 5%)
- [ ] Threshold can be ratcheted up by editing `.testcoverage.yml`

## Dependencies

| Issue | Why |
|-------|-----|
| #01 CI Pipeline | Coverage check runs as a CI step |

## References

- [go-test-coverage](https://github.com/vladopajic/go-test-coverage) — threshold enforcement
- [go test -coverprofile](https://pkg.go.dev/cmd/go#hdr-Testing_flags) — built-in coverage
- [actions/upload-artifact](https://github.com/actions/upload-artifact) — CI artifact storage
