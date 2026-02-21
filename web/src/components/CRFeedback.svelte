<script lang="ts">
  import { renderMarkdown } from "../lib/utils/markdown.js";

  interface Props {
    crFeedback?: string;
    crFixSummary?: string;
  }

  let { crFeedback, crFixSummary }: Props = $props();

  let feedbackOpen = $state(true);
  let fixOpen = $state(true);

  let hasFeedback = $derived(!!crFeedback);
  let hasFix = $derived(!!crFixSummary);
</script>

{#if hasFeedback || hasFix}
  <div class="cr-section">
    <h3>Code Review</h3>

    {#if hasFeedback}
      <details bind:open={feedbackOpen}>
        <summary>Review Feedback</summary>
        <div class="markdown">
          <!-- eslint-disable-next-line svelte/no-at-html-tags -- trusted server markdown -->
          {@html renderMarkdown(crFeedback!)}
        </div>
      </details>
    {/if}

    {#if hasFix}
      <details bind:open={fixOpen}>
        <summary>Fix Summary</summary>
        <div class="markdown">
          <!-- eslint-disable-next-line svelte/no-at-html-tags -- trusted server markdown -->
          {@html renderMarkdown(crFixSummary!)}
        </div>
      </details>
    {/if}
  </div>
{/if}

<style>
  h3 {
    margin: 0 0 0.75rem;
    font-size: 0.85rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
  }

  .cr-section {
    margin-top: 1.5rem;
  }

  details {
    margin-bottom: 0.75rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }

  summary {
    padding: 0.6rem 0.75rem;
    background: var(--bg-surface);
    cursor: pointer;
    font-size: 0.85rem;
    font-weight: 500;
    color: var(--text-primary);
  }

  summary:hover {
    background: var(--bg-hover);
  }

  .markdown {
    padding: 0.75rem;
    font-size: 0.85rem;
    line-height: 1.6;
    color: var(--text-primary);
  }

  .markdown :global(p) {
    margin: 0 0 0.5rem;
  }

  .markdown :global(code) {
    background: var(--bg-active);
    padding: 0.15rem 0.35rem;
    border-radius: 3px;
    font-size: 0.8rem;
  }

  .markdown :global(pre) {
    background: var(--bg-surface);
    padding: 0.75rem;
    border-radius: 4px;
    overflow-x: auto;
  }

  .markdown :global(pre code) {
    background: none;
    padding: 0;
  }

  .markdown :global(ul),
  .markdown :global(ol) {
    padding-left: 1.5rem;
    margin: 0 0 0.5rem;
  }

  .markdown :global(a) {
    color: var(--color-running);
  }
</style>
