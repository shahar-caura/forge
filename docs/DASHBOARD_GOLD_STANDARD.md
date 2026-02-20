Forge Dashboard — Vision Brainstorm

  What You Already Have (Raw Material)

  - .forge/runs/<id>.yaml — full run state per step, branch, PR URL, Jira key, errors
  - .forge/runs/<id>-agent-step<N>.log — real-time agent output
  - Flat-file YAML, no daemon, no DB — the dashboard needs to bridge this gap

  The Core: forge serve

  A Go HTTP server embedded in the forge binary. Serves a React SPA + SSE endpoint. Zero extra deps to run — just forge serve and open a
  browser.

  ---
  Tier 1 — The Essentials

  1. Multi-Repo Run Aggregator
  Scan all repos on disk (configurable list or auto-discover from ~/code/*/.forge/runs/). One unified view across every project. Each run
  tagged with repo name, account identity, and which forge.yaml config was used.

  2. Live Pipeline Visualization
  Each run rendered as a horizontal pipeline of 11 steps. Steps light up green/yellow/red as they transition. Currently-running step
  pulses. Failed step shows the error inline. Click a step to see its log.

  3. Real-Time Log Streaming
  SSE-backed live tail of agent logs. Like forge logs -f but in the browser with syntax highlighting, collapsible tool calls, and
  auto-scroll. Stream .forge/runs/<id>-agent-step4.log as the agent writes to it.

  4. Account Awareness
  Detect which GitHub account is active per repo (gh auth status). Tag each run with the identity that executed it. Filter/group by
  account. Critical for consultants managing multiple orgs.

  ---
  Tier 2 — Intelligence Layer

  5. Run Timeline / Gantt View
  Right now you don't track per-step timestamps — add StartedAt/CompletedAt to each StepState. Then render a Gantt chart showing exactly
  where time was spent. "Agent ran for 12 minutes, CR poll waited 3 minutes, push took 2 seconds." This alone would be worth the
  dashboard.

  6. Cost & Token Tracking
  Parse Claude's JSON output for token counts (or intercept the --output-format json response). Track input/output tokens per run, per
  step, per repo. Show daily/weekly burn rate. Alert when a single run exceeds a threshold. "This month: 2.1M tokens across 47 runs,
  ~$14.30."

  7. PR Lifecycle Tracking
  Don't stop at "PR created." Poll gh pr view to show: PR open → review requested → changes requested → approved → merged. The dashboard
  becomes the single pane of glass for the full lifecycle, not just the forge pipeline.

  8. Dependency Graph Visualization
  When running --all-issues, render the issue dependency DAG visually. Show which issues are queued, running in parallel, blocked. Animate
   as issues complete and unlock downstream work. Like a CI pipeline view but for your entire feature plan.

  ---
  Tier 3 — Power Features

  9. Diff Preview per Run
  After the agent runs, capture git diff of the worktree. Show the actual code changes in the dashboard with syntax highlighting before
  the PR is even pushed. Review agent output without leaving the dashboard.

  10. Agent Replay / Debug Mode
  Record every tool call the agent made (Read, Write, Bash, etc.) with timestamps. Replay them step-by-step in the UI. "At 2:03 PM the
  agent read auth.go, then wrote 47 lines to middleware.go, then ran go test." Understand how the agent solved the problem, not just that
  it did.

  11. Run Comparison
  Compare two runs side-by-side. Same plan, different agent configs. Same issue, attempt 1 vs attempt 2. Show diffs in: time taken, tokens
   used, files changed, test results. Useful for tuning prompts and agent settings.

  12. Batch Orchestration Dashboard
  For --all-issues runs: a Kanban-style board. Columns = topo levels. Cards = issues. Cards move right as they complete. Shows parallelism
   factor ("3 issues running concurrently"). Estimated completion based on average step durations.

  ---
  Tier 4 — Delightful Extras

  13. Notification Center
  Aggregate Slack notifications, PR comments, CR feedback all in one feed. "FORGE-42: PR approved and merged" alongside "FORGE-43: CR
  feedback received — agent fixing." Replace Slack-hopping with one timeline.

  14. Health & Trends Dashboard
  - Success rate over time (% of runs that complete without intervention)
  - Average time-to-PR (from forge run to PR created)
  - Most common failure step (is it always CR? Always tests?)
  - Runs per day/week heatmap
  - Agent "hit rate" — how often does the first agent pass succeed vs needing CR fixes?

  15. Quick Actions from UI
  - Resume a failed run (button → forge resume <id>)
  - Open the worktree in your editor
  - Open the PR in GitHub
  - Open the Jira issue
  - Retry with a different agent (claude vs ralph)
  - Kill a stuck run

  16. Forge "Control Tower" Mode
  For teams: a shared forge serve instance that watches a directory of plan files. Drop a .md file into plans/, forge picks it up, runs
  it, dashboard shows progress. Like a CI server but for AI-driven development. Plans as the unit of work.

  17. Terminal Dashboard Alternative (TUI)
  Not everyone wants a browser. A forge dashboard TUI using bubbletea — split panes, live log tailing, keyboard navigation. Fits the
  "single binary" philosophy better than React. Could be the V1, with web as V2.

  18. Webhook/API for External Integration
  forge serve exposes a REST API. Other tools can query run status, trigger runs, subscribe to events. Integrate with Raycast, Alfred, or
  a custom Slack bot. "Hey Forge, what's running right now?"

  ---
  Architecture Suggestion

  forge serve [--port 8080] [--repos ~/code/project1,~/code/project2]

  ├── Go HTTP server (net/http, no framework)
  │   ├── GET  /api/runs          → list all runs across repos
  │   ├── GET  /api/runs/:id      → run detail + steps
  │   ├── GET  /api/runs/:id/logs → SSE stream of agent log
  │   ├── POST /api/runs/:id/resume → trigger resume
  │   ├── GET  /api/stats         → aggregate metrics
  │   └── GET  /                  → serve embedded React SPA
  │
  ├── State watcher (fsnotify on .forge/runs/*.yaml)
  │   └── pushes SSE events on state change
  │
  ├── React SPA (embedded via go:embed)
  │   ├── Dashboard (all runs, filters, search)
  │   ├── Run detail (pipeline viz + logs)
  │   ├── Batch view (dependency graph)
  │   └── Stats (charts, trends)
  │
  └── SQLite (optional, for cross-repo aggregation + history beyond retention)

  What I'd Build First

  1. Add per-step timestamps to StepState (tiny change, massive value)
  2. forge serve with a minimal Go HTTP server + SSE
  3. TUI dashboard (forge dashboard) using bubbletea — faster to ship, stays true to CLI philosophy
  4. Multi-repo scanner that finds all .forge/runs/ directories
  5. React SPA embedded via go:embed for the full web experience