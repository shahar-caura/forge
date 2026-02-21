<script lang="ts">
  import { isConnected } from "./lib/state/sse.svelte.js";

  let connected = $derived(isConnected());
</script>

<footer class="status-bar">
  <span class="indicator" class:connected></span>
  {connected ? "Connected" : "Disconnected"}
</footer>

<style>
  .status-bar {
    padding: 0.35rem 1rem;
    background: var(--bg-surface);
    border-top: 1px solid var(--border);
    font-size: 0.75rem;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--color-error);
  }

  .indicator.connected {
    background: var(--color-success);
  }
</style>
