<script lang="ts">
  import type { StepState } from "../lib/api/client.js";

  interface Props {
    steps: StepState[];
  }

  let { steps }: Props = $props();
</script>

<div class="step-progress">
  <h3>Steps</h3>
  <ol>
    {#each steps as step, i (i)}
      <li class="step step-{step.status}">
        <span class="icon">
          {#if step.status === "completed"}
            <span class="check">✓</span>
          {:else if step.status === "running"}
            <span class="spinner"></span>
          {:else if step.status === "failed"}
            <span class="cross">✗</span>
          {:else}
            <span class="circle"></span>
          {/if}
        </span>
        <span class="name">{step.name}</span>
        {#if step.error}
          <span class="error-msg">{step.error}</span>
        {/if}
      </li>
    {/each}
  </ol>
</div>

<style>
  h3 {
    margin: 0 0 0.75rem;
    font-size: 0.85rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
  }

  ol {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .step {
    display: flex;
    align-items: flex-start;
    gap: 0.6rem;
    padding: 0.45rem 0;
    border-left: 2px solid var(--border);
    margin-left: 0.5rem;
    padding-left: 1rem;
    position: relative;
  }

  .step:last-child {
    border-left-color: transparent;
  }

  .icon {
    flex-shrink: 0;
    width: 1.2rem;
    height: 1.2rem;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8rem;
    margin-left: -1.65rem;
    background: var(--bg-base);
  }

  .check {
    color: var(--color-success);
    font-weight: bold;
  }

  .cross {
    color: var(--color-error);
    font-weight: bold;
  }

  .circle {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-pending);
  }

  .spinner {
    width: 10px;
    height: 10px;
    border: 2px solid var(--color-running);
    border-top-color: transparent;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .name {
    font-size: 0.85rem;
    color: var(--text-primary);
  }

  .step-pending .name {
    color: var(--text-muted);
  }

  .step-failed .name {
    color: var(--color-error);
  }

  .error-msg {
    display: block;
    font-size: 0.75rem;
    color: var(--color-error);
    margin-top: 0.15rem;
    opacity: 0.85;
  }
</style>
