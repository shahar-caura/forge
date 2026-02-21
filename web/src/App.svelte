<script lang="ts">
  import StatusBar from "./StatusBar.svelte";
  import RunsList from "./components/RunsList.svelte";
  import RunDetail from "./components/RunDetail.svelte";
  import { match, navigate } from "./lib/router.svelte.js";
  import { connect, disconnect } from "./lib/state/sse.svelte.js";
  import { fetchRuns } from "./lib/state/runs.svelte.js";

  let route = $derived(match());

  $effect(() => {
    fetchRuns();
    connect();
    return disconnect;
  });
</script>

<div class="layout">
  <header>
    <h1>Forge Dashboard</h1>
  </header>

  <div class="body">
    <nav>
      <button class:active={route.route === "runs"} onclick={() => navigate("/")}>Runs</button>
      <button class:active={route.route === "stats"} onclick={() => navigate("/stats")}>
        Stats
      </button>
    </nav>

    <main>
      {#if route.route === "runs"}
        <RunsList />
      {:else if route.route === "run-detail"}
        <RunDetail id={route.params.id} />
      {:else}
        <p>Stats coming soon.</p>
      {/if}
    </main>
  </div>

  <StatusBar />
</div>

<style>
  :global(body) {
    margin: 0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: var(--bg-base);
    color: var(--text-primary);
  }

  .layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  header {
    padding: 0.75rem 1.5rem;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border);
  }

  header h1 {
    margin: 0;
    font-size: 1.1rem;
    font-weight: 600;
  }

  .body {
    display: flex;
    flex: 1;
    overflow: hidden;
  }

  nav {
    width: 180px;
    padding: 1rem 0.75rem;
    background: var(--bg-sidebar);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  nav button {
    background: none;
    border: none;
    color: var(--text-secondary);
    padding: 0.5rem 0.75rem;
    text-align: left;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  nav button:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  nav button.active {
    background: var(--bg-active);
    color: var(--text-bright);
  }

  main {
    flex: 1;
    padding: 1.5rem;
    overflow-y: auto;
  }
</style>
