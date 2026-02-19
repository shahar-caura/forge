# Reproducible Builds
> Effort: M | Priority: 6 | Depends on: #01

## Context

Forge is built with `go build -o bin/forge ./cmd/forge`. There is no version injection, no cross-compilation, and no release automation. The binary has no way to report its version.

## Problem

- `forge --version` doesn't exist — no way to identify which build is running
- No reproducible release process — builds depend on whoever runs `go build`
- No cross-platform binaries (linux/darwin, amd64/arm64)
- No checksums or provenance for distributed binaries

## Proposed Solution

Use [GoReleaser](https://goreleaser.com/) for release automation, triggered by pushing a git tag. Inject version info via `-ldflags` at build time.

### Configuration

**Version variables in `cmd/forge/main.go`:**

```go
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)
```

Add a `version` subcommand to the root cobra command:

```go
root.AddCommand(&cobra.Command{
    Use:   "version",
    Short: "Print version information",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("forge %s (commit: %s, built: %s)\n", version, commit, date)
    },
})
```

**`.goreleaser.yml`:**

```yaml
version: 2

builds:
  - main: ./cmd/forge
    binary: forge
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.CommitDate}}
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    mod_timestamp: "{{ .CommitTimestamp }}"

archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

release:
  github:
    owner: shahar-caura
    name: forge
```

**`.github/workflows/release.yml`:**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Makefile Changes

Update the `build` target to inject version from git:

```makefile
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/forge
```

Add a release dry-run target:

```makefile
release-dry:
	goreleaser release --snapshot --clean
```

### CI Integration

The release workflow is a separate file (`.github/workflows/release.yml`) triggered only on tag pushes. CI workflow from #01 continues to run on PRs and master pushes.

## Acceptance Criteria

- [ ] `forge version` prints version, commit hash, and build date
- [ ] `make build` injects version info via `-ldflags`
- [ ] `.goreleaser.yml` exists and produces binaries for linux/darwin x amd64/arm64
- [ ] `make release-dry` runs a local snapshot release (no publish)
- [ ] `.github/workflows/release.yml` triggers on `v*` tags
- [ ] Release creates GitHub release with binaries and checksums
- [ ] Builds use `-trimpath` and `CGO_ENABLED=0` for reproducibility
- [ ] `go build` without ldflags still works (defaults to "dev")

## Dependencies

| Issue | Why |
|-------|-----|
| #01 CI Pipeline | Release workflow builds on CI patterns; Go setup is shared |

## References

- [GoReleaser](https://goreleaser.com/) — Go release automation
- [goreleaser-action](https://github.com/goreleaser/goreleaser-action) — GitHub Actions integration
- [Go ldflags](https://pkg.go.dev/cmd/link) — version injection at build time
