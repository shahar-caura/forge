<script lang="ts">
  interface Props {
    runId: string;
    step: number;
    live: boolean;
  }

  let { runId, step, live }: Props = $props();

  const MAX_LINES = 5000;
  let seq = 0;
  let lines = $state<{ id: number; text: string }[]>([]);
  let error = $state<string | null>(null);
  let container: HTMLElement | undefined = $state();
  let autoScroll = $state(true);
  let eventSource: EventSource | null = null;

  // Strip ANSI escape codes.
  function stripAnsi(text: string): string {
    return text.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, "");
  }

  function connect() {
    lines = [];
    error = null;

    const url = `/api/runs/${runId}/logs?step=${step}`;
    const es = new EventSource(url);
    eventSource = es;

    es.onmessage = (event) => {
      lines.push({ id: seq++, text: stripAnsi(event.data) });
      if (lines.length > MAX_LINES) {
        lines = lines.slice(lines.length - MAX_LINES);
      }
      if (autoScroll && container) {
        requestAnimationFrame(() => {
          container!.scrollTop = container!.scrollHeight;
        });
      }
    };

    es.onerror = () => {
      es.close();
      eventSource = null;
      if (lines.length === 0) {
        error = "No logs available for this step.";
      }
    };
  }

  function disconnect() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  function handleScroll() {
    if (!container) return;
    const atBottom =
      container.scrollHeight - container.scrollTop - container.clientHeight < 40;
    autoScroll = atBottom;
  }

  // Reconnect when props change.
  $effect(() => {
    // Access reactive deps.
    void runId;
    void step;
    connect();
    return disconnect;
  });
</script>

<div class="log-viewer">
  <div class="log-header">
    <span class="log-title">Agent Logs — Step {step}</span>
    {#if live}
      <span class="live-badge">LIVE</span>
    {/if}
  </div>
  <div class="log-content" bind:this={container} onscroll={handleScroll}>
    {#if error}
      <p class="log-error">{error}</p>
    {:else if lines.length === 0}
      <p class="log-empty">Waiting for logs...</p>
    {:else}
      {#each lines as line, i (line.id)}
        <div class="log-line"><span class="line-num">{i + 1}</span>{line.text}</div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .log-viewer {
    margin-top: 1rem;
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }

  .log-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border);
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .live-badge {
    font-size: 0.65rem;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    background: color-mix(in srgb, var(--color-running) 20%, transparent);
    color: var(--color-running);
    font-weight: 600;
  }

  .log-content {
    max-height: 400px;
    overflow-y: auto;
    background: var(--bg-base);
    padding: 0.5rem 0;
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.75rem;
    line-height: 1.5;
  }

  .log-line {
    padding: 0 0.75rem;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .log-line:hover {
    background: var(--bg-hover);
  }

  .line-num {
    display: inline-block;
    width: 3.5rem;
    color: var(--text-muted);
    text-align: right;
    margin-right: 0.75rem;
    user-select: none;
    opacity: 0.5;
  }

  .log-error,
  .log-empty {
    padding: 1.5rem;
    text-align: center;
    color: var(--text-muted);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    font-size: 0.85rem;
  }
</style>
