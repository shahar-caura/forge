# Issue Gold Standard — Template & Rationale

Reference document for authoring forge plan-file issues. Every issue Forge executes should follow this template. Quality here directly determines success rate, parallelism, merge conflict frequency, and agent accuracy.

See also: [CLAUDE.md](../CLAUDE.md) § Issue Authoring — scope each issue to ≤3 files.

---

## Template

```markdown
---
title: "Add retry logic to Slack notifier"
effort: S                          # XS/S/M/L
depends_on: "#12, #14"             # topsort input
---

## Problem
Slack webhook calls fail silently on transient 5xx errors.
Users think notifications were sent when they weren't.

## Solution
Add exponential backoff retry (3 attempts, 1s/2s/4s) to
`internal/provider/notifier/slack.go:Send()`.

## File Manifest
| Action | File                                       |
|--------|--------------------------------------------|
| Modify | `internal/provider/notifier/slack.go`      |
| Create | `internal/provider/notifier/retry.go`      |
| Modify | `internal/provider/notifier/slack_test.go`  |

## Key Changes
- Wrap `http.Post` call in retry loop with backoff
- Only retry on 5xx and network errors, not 4xx
- Log each retry attempt

## Interface Changes
None. `Send(ctx, message)` signature unchanged.

## Acceptance Criteria
- [ ] 5xx retries 3 times with exponential backoff
- [ ] 4xx fails immediately, no retry
- [ ] Context cancellation respected mid-retry
- [ ] `make test` passes

## Verify
```bash
go test ./internal/provider/notifier/... -run TestSlackRetry -v
```

## Anti-Goals
- Do NOT add retry to other providers
- Do NOT add circuit breaker (that's #18)
- Do NOT change the notifier interface

## Context for Agent
Read `internal/provider/notifier/slack.go` first.
Follow existing error wrapping: `fmt.Errorf("slack: %w", err)`.
```

---

## Section Rationale

### Frontmatter (`title`, `effort`, `depends_on`)

- **title** — progressive detail: reviewers read titles, agents read everything.
- **effort** — XS/S/M/L t-shirt size. XS = single function change, L = new subsystem. Issues larger than M should be decomposed.
- **depends_on** — direct input to topological sort. Forge uses this to determine execution order and parallelism boundaries.

### Problem

One paragraph, max. States the *user-visible* consequence, not the technical gap. If you can't articulate the problem in two sentences, the issue is too vague.

### Solution

The approach, not the implementation. Says *what* changes, not every line of code. Enough for a reviewer to evaluate the direction without reading the diff.

### File Manifest

The single most powerful section for parallel execution. When every issue declares which files it creates/modifies/deletes:

- **Collision detection** — two issues can't both modify the same file unless one depends on the other.
- **Smarter topsort** — infer file-level deps beyond explicit `depends_on`.
- **File locking** — at batch start, each issue locks its declared files. Touching an undeclared file fails the run.
- **Visual dependency graphs** — generated from file overlap.

Actions: `Create`, `Modify`, `Delete`. Scope to ≤3 files per issue (see CLAUDE.md).

### Key Changes

Bullet list of the substantive changes. Not a diff, but enough that a reviewer knows the *shape* of the work. Three to five bullets max.

### Interface Changes

When issue #12 adds a method to an interface and #15 calls it, declaring the contract lets dependent issues code against it before the implementation exists. Like a header file for parallel work.

Use `None.` when the public API is unchanged — this is a positive signal, not filler.

### Acceptance Criteria

Checkboxes the agent (and reviewer) can verify mechanically. Each criterion is binary pass/fail. Avoid subjective criteria like "code is clean."

### Verify

A concrete command the agent runs after implementation. Not "make sure it works" but `go test ./... -run TestSpecificThing`. The agent self-validates, and PR creation can be gated on this passing.

> **Open question:** Should Forge validate that issues conform to this template programmatically (e.g. check required headings, parse the file manifest table)? Or is rigid enforcement counterproductive when working with AI — where the agent can interpret intent from loosely structured input? Needs further thinking. See also [#59](https://github.com/shahar-caura/forge/issues/59).

### Anti-Goals

The most impactful section for LLM agents. Without explicit boundaries, agents drift. Telling the agent what *not* to do prevents the majority of scope creep. Common anti-goals:

- Don't refactor surrounding code
- Don't add features to adjacent systems
- Don't change interfaces unless the issue says to

### Context for Agent

Orientation, not instructions. "Read X first", "Follow the pattern in Y." One real code snippet beats three paragraphs of description.

---

## Creative Ideas (Future)

### Checkpoint Assertions

Break work into verifiable steps:

1. After adding `retry.go` → `go build ./...` passes
2. After modifying `slack.go` → `go test ./internal/provider/notifier/...` passes
3. After all changes → `make lint` passes

If the agent fails at step 2, it knows exactly where things went wrong. TDD for the issue itself.

### Conflict Resolution Rules

```markdown
## On Conflict
If this overlaps with #14, this issue yields (rebase on top of #14's branch).
```

Pre-declare who wins when parallel issues touch the same file. The pipeline auto-resolves instead of failing.

### Blast Radius Score (auto-calculated)

Computed from: files touched × dependents × import depth. High blast radius = human review required. Low = auto-merge candidate.

### Knowledge Prerequisites

```markdown
## Read First
- `internal/provider/notifier/slack.go`
- `internal/provider/types.go:Notifier`
```

Tell the agent what to read before coding. Reduces hallucination. The agent prompt can inject these as pre-reads.

### Escape Hatch

```markdown
## If Stuck
- Tests won't pass after 3 attempts → commit with `[WIP]`, open draft PR
- Interface change needed → STOP, comment on issue, don't proceed
```

For fire-and-forget, the failure mode matters as much as the happy path.

### Diff Sketch

```markdown
## Rough Shape
// In slack.go:Send()
- resp, err := http.Post(...)
+ resp, err := retryWithBackoff(ctx, 3, func() (*http.Response, error) {
+     return http.Post(...)
+ })
```

Not copy-paste prescriptive, but enough to know the *shape* of the change.

### Issue-as-Test

Acceptance criteria become actual test cases generated *before* implementation. Forge creates the test file first, then the agent implements until tests pass.

### Template Inheritance

`type: bug` auto-includes "Reproduction Steps" and "Root Cause". `type: feature` auto-includes "API Design". Less boilerplate, more structure.

### Semantic Merge Order

Beyond execution deps, declare merge order. Two issues might execute in parallel but merge sequentially to avoid conflicts.

### Runtime Context

```markdown
## Environment
Uses: SLACK_WEBHOOK_URL (already in forge.yaml)
New env vars: none
New dependencies: none
```

Validate external deps before the agent starts.

### Success Metrics (beyond tests pass)

- No new linter warnings
- Coverage doesn't decrease
- `go vet` clean
- Performance benchmarks hold

### Confidence Score

After finishing, the agent self-rates confidence (1–5). Low confidence = human review. High = auto-merge candidate.

### Issue Decomposition Hints

```markdown
## If This Is Too Big
Split into:
  a) Add retry.go utility (S, 1 file)
  b) Wire retry into slack.go (S, 2 files)
```

Pre-declare the split if the issue turns out too large. Or Forge auto-splits issues exceeding the file limit.
