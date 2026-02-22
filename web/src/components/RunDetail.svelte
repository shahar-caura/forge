<script lang="ts">
  import { findRun } from "../lib/state/runs.svelte.js";
  import { navigate } from "../lib/router.svelte.js";
  import { relativeTime } from "../lib/utils/time.js";
  import StepProgress from "./StepProgress.svelte";
  import CRFeedback from "./CRFeedback.svelte";
  import LogViewer from "./LogViewer.svelte";

  interface Props {
    id: string;
  }

  let { id }: Props = $props();
  let run = $derived(findRun(id));
  let selectedStep = $state<number | null>(null);

  function planName(r: { plan_title?: string; plan_path: string }): string {
    if (r.plan_title) return r.plan_title;
    const parts = r.plan_path.split("/");
    return parts[parts.length - 1].replace(/\.md$/, "");
  }

  function handleSelectStep(step: number) {
    selectedStep = selectedStep === step ? null : step;
  }
</script>

<div class="run-detail">
  <button class="back" onclick={() => navigate("/")}>← Runs</button>

  {#if !run}
    <p class="empty">Run not found.</p>
  {:else}
    <div class="header">
      <h2>{planName(run)}</h2>
      <span class="badge badge-{run.status}">{run.status}</span>
    </div>

    <div class="meta">
      <div class="meta-row">
        <span class="label">ID</span>
        <span class="value mono">{run.id}</span>
      </div>
      {#if run.branch}
        <div class="meta-row">
          <span class="label">Branch</span>
          <span class="value mono">{run.branch}</span>
        </div>
      {/if}
      <div class="meta-row">
        <span class="label">Created</span>
        <span class="value">{relativeTime(run.created_at)}</span>
      </div>
      <div class="meta-row">
        <span class="label">Updated</span>
        <span class="value">{relativeTime(run.updated_at)}</span>
      </div>
    </div>

    <div class="links">
      {#if run.pr_url}
        <a href={run.pr_url} target="_blank" rel="noopener">PR #{run.pr_number}</a>
      {/if}
      {#if run.issue_url}
        <a href={run.issue_url} target="_blank" rel="noopener">{run.issue_key}</a>
      {/if}
    </div>

    <StepProgress steps={run.steps} onSelectStep={handleSelectStep} {selectedStep} />

    {#if selectedStep !== null}
      <LogViewer runId={run.id} step={selectedStep} live={run.status === "active"} />
    {/if}

    <CRFeedback crFeedback={run.cr_feedback} crFixSummary={run.cr_fix_summary} />
  {/if}
</div>

<style>
  .run-detail {
    max-width: 720px;
  }

  .back {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 0;
    font-size: 0.85rem;
    margin-bottom: 1rem;
  }

  .back:hover {
    color: var(--text-primary);
  }

  .header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 1rem;
  }

  h2 {
    margin: 0;
    font-size: 1.1rem;
    font-weight: 600;
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

  .meta {
    margin-bottom: 1.5rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }

  .meta-row {
    display: flex;
    padding: 0.45rem 0.75rem;
    border-bottom: 1px solid var(--border);
    font-size: 0.85rem;
  }

  .meta-row:last-child {
    border-bottom: none;
  }

  .label {
    width: 100px;
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .value {
    color: var(--text-primary);
  }

  .mono {
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.8rem;
  }

  .links {
    display: flex;
    gap: 1rem;
    margin-bottom: 1.5rem;
  }

  .links:empty {
    display: none;
  }

  a {
    color: var(--color-running);
    text-decoration: none;
    font-size: 0.85rem;
  }

  a:hover {
    text-decoration: underline;
  }

  .empty {
    color: var(--text-muted);
    text-align: center;
    padding: 3rem 0;
  }
</style>
