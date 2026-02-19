# Security Scanning
> Effort: S | Priority: 5 | Depends on: #01

## Context

Forge shells out to external CLIs (`gh`, `git`, `claude`, `curl`) and makes HTTP requests to Jira and Slack. It handles API tokens and webhook URLs. There is no automated vulnerability or dead code scanning.

## Problem

- No visibility into known vulnerabilities in dependencies
- No detection of unused/dead code that increases attack surface and maintenance burden
- Security issues are only caught by manual review

## Proposed Solution

Add two official Go team tools: `govulncheck` for dependency vulnerability scanning and `deadcode` for unreachable code detection. Both are low-noise, high-signal tools.

### Configuration

No config files needed — both tools work out of the box.

### Makefile Changes

```makefile
vuln:
	govulncheck ./...

deadcode:
	deadcode ./...
```

### CI Integration

Add to `.github/workflows/ci.yml` after the test step:

```yaml
      - name: Install security tools
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          go install golang.org/x/tools/cmd/deadcode@latest

      - name: Vulnerability check
        run: make vuln

      - name: Dead code check
        run: make deadcode
```

**Rollout strategy:** Start with `continue-on-error: true` for both steps. Once the baseline is clean (fix any findings), remove `continue-on-error` to make them blocking.

```yaml
      - name: Vulnerability check
        run: make vuln
        continue-on-error: true  # Remove once baseline is clean

      - name: Dead code check
        run: make deadcode
        continue-on-error: true  # Remove once baseline is clean
```

## Acceptance Criteria

- [ ] `make vuln` runs `govulncheck ./...`
- [ ] `make deadcode` runs `deadcode ./...`
- [ ] CI runs both checks after tests
- [ ] Initial run: both pass or findings are triaged
- [ ] Once clean, `continue-on-error` is removed (checks become blocking)
- [ ] Any govulncheck findings are addressed (upgrade dep or document accepted risk)
- [ ] Any deadcode findings are addressed (remove dead code or document reason)

## Dependencies

| Issue | Why |
|-------|-----|
| #01 CI Pipeline | Security checks run as CI steps |

## References

- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) — official Go vulnerability scanner
- [deadcode](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) — official Go dead code finder
- [Go vulnerability database](https://vuln.go.dev/)
