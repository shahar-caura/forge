<script lang="ts">
  import {
    getFilteredRuns,
    getStatusFilter,
    setStatusFilter,
    isLoading,
    getError,
  } from "../lib/state/runs.svelte.js";
  import { navigate } from "../lib/router.svelte.js";
  import { relativeTime } from "../lib/utils/time.js";
  import type { RunStatus } from "../lib/api/client.js";

  let runs = $derived(getFilteredRuns());
  let filter = $derived(getStatusFilter());
  let loading = $derived(isLoading());
  let error = $derived(getError());

  const filters: Array<{ label: string; value: RunStatus | "all" }> = [
    { label: "All", value: "all" },
    { label: "Active", value: "active" },
    { label: "Completed", value: "completed" },
    { label: "Failed", value: "failed" },
  ];

  function stepProgress(run: { steps: Array<{ status: string }> }): string {
    const done = run.steps.filter((s) => s.status === "completed").length;
    return `${done}/${run.steps.length}`;
  }

  function planName(run: { plan_title?: string; plan_path: string }): string {
    if (run.plan_title) return run.plan_title;
    const parts = run.plan_path.split("/");
    return parts[parts.length - 1].replace(/\.md$/, "");
  }
</script>

<div class="runs-list">
  <div class="toolbar">
    <h2>Pipeline Runs</h2>
    <div class="filters">
      {#each filters as f (f.value)}
        <button class:active={filter === f.value} onclick={() => setStatusFilter(f.value)}>
          {f.label}
        </button>
      {/each}
    </div>
  </div>

  {#if loading}
    <p class="empty">Loading runs…</p>
  {:else if error}
    <p class="empty error">{error}</p>
  {:else if runs.length === 0}
    <p class="empty">No runs found.</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th>Plan</th>
          <th>Status</th>
          <th>Steps</th>
          <th>Updated</th>
          <th>PR</th>
        </tr>
      </thead>
      <tbody>
        {#each runs as run (run.id)}
          <tr class="clickable" onclick={() => navigate(`/runs/${run.id}`)}>
            <td class="plan-name">{planName(run)}</td>
            <td>
              <span class="badge badge-{run.status}">{run.status}</span>
            </td>
            <td class="mono">{stepProgress(run)}</td>
            <td class="muted">{relativeTime(run.updated_at)}</td>
            <td>
              {#if run.pr_url}
                <a
                  href={run.pr_url}
                  target="_blank"
                  rel="noopener"
                  onclick={(e) => e.stopPropagation()}
                >
                  #{run.pr_number}
                </a>
              {:else}
                <span class="muted">—</span>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .runs-list {
    max-width: 960px;
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
  }

  h2 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
  }

  .filters {
    display: flex;
    gap: 0.25rem;
  }

  .filters button {
    background: none;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    padding: 0.25rem 0.75rem;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
  }

  .filters button:hover {
    background: var(--bg-hover);
  }

  .filters button.active {
    background: var(--bg-active);
    color: var(--text-bright);
    border-color: var(--text-secondary);
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  th {
    text-align: left;
    padding: 0.5rem 0.75rem;
    border-bottom: 1px solid var(--border);
    color: var(--text-muted);
    font-size: 0.75rem;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  td {
    padding: 0.6rem 0.75rem;
    border-bottom: 1px solid var(--border);
    font-size: 0.85rem;
  }

  tr.clickable {
    cursor: pointer;
  }

  tr.clickable:hover td {
    background: var(--bg-hover);
  }

  .plan-name {
    font-weight: 500;
  }

  .mono {
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.8rem;
  }

  .muted {
    color: var(--text-muted);
  }

  .badge {
    display: inline-block;
    padding: 0.15rem 0.5rem;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 500;
  }

  .badge-active {
    background: color-mix(in srgb, var(--color-running) 20%, transparent);
    color: var(--color-running);
  }

  .badge-completed {
    background: color-mix(in srgb, var(--color-success) 20%, transparent);
    color: var(--color-success);
  }

  .badge-failed {
    background: color-mix(in srgb, var(--color-error) 20%, transparent);
    color: var(--color-error);
  }

  .empty {
    color: var(--text-muted);
    text-align: center;
    padding: 3rem 0;
  }

  .error {
    color: var(--color-error);
  }

  a {
    color: var(--color-running);
    text-decoration: none;
  }

  a:hover {
    text-decoration: underline;
  }
</style>
