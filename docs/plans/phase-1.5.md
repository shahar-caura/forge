# Phase 1.5: Checkpointing & Resume

## Context

Phase 1 delivers a working pipeline but no persistence — if step 4 (agent) fails after 40 minutes, everything is lost. Phase 1.5 adds run state files so `forge resume <run-id>` continues from the last failed step. Also adds `forge runs` to list runs.

---

## Design Decisions

- **Run ID:** `YYYYMMDD-HHMMSS-<slug>` (human-readable, sortable, no UUID dep)
- **State location:** `.forge/runs/<run-id>.yaml` (outside config, gitignored)
- **Resume strategy:** Skip completed steps, re-run the failed step and everything after it
- **Worktree on failure:** Preserved (not cleaned up) so resume can reuse it
- **Agent resume:** Re-run in existing worktree — agent adapts to whatever state exists
- **Auto-cleanup:** Delete completed run state files older than N days (default 7, configurable via `state.retention` in forge.yaml)
- **CLI routing:** `switch os.Args[1]` — three commands don't warrant cobra

---

## Files

```
internal/state/state.go          # NEW — RunState struct, New/Load/Save/List/Cleanup
internal/state/state_test.go     # NEW — state persistence tests
internal/pipeline/run.go         # MODIFY — accept *RunState, runStep helper, skip logic
internal/pipeline/run_test.go    # MODIFY — update existing + add resume tests
cmd/forge/main.go                # MODIFY — add resume/runs subcommands
internal/config/config.go        # MODIFY — add StateConfig with retention
.gitignore                       # MODIFY — add .forge/
docs/plans/phase-1.5.md          # NEW — this plan
```

---

## Step 1: State Package

`internal/state/state.go` — RunState struct with:
- `New(id, planPath)` — 6 steps all pending
- `Load(id)` — read from `.forge/runs/<id>.yaml`
- `Save()` — atomic write (temp + rename)
- `List()` — all runs sorted by created_at desc
- `Cleanup(retention)` — delete completed runs older than retention

Step names: `"read plan"`, `"generate branch"`, `"create worktree"`, `"run agent"`, `"commit and push"`, `"create pr"`

## Step 2: Pipeline Changes

- Export `BranchName()` for CLI slug generation
- `Run()` accepts `*state.RunState`
- `runStep()` helper: skip completed, mark running → save → execute → mark completed/failed → save
- Preserve worktree on failure (skip `Remove()`)
- Validate worktree path on resume via `os.Stat`
- Re-read plan on resume (not stored in state)

## Step 3: CLI Changes

Three subcommands via `switch os.Args[1]`:
- `forge run <plan.md>` — generate run ID, create state, run pipeline
- `forge resume <run-id>` — load state, reset failed→pending, run pipeline
- `forge runs` — list all runs as table

Config addition: `state.retention` (default `168h`)

## Step 4: Cleanup

- `.forge/` added to `.gitignore`
- Auto-cleanup of completed runs older than retention after each run/resume
