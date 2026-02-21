<script lang="ts">
  import StatusBar from "./StatusBar.svelte";

  let currentView = $state("runs");
</script>

<div class="layout">
  <header>
    <h1>Forge Dashboard</h1>
  </header>

  <div class="body">
    <nav>
      <button class:active={currentView === "runs"} onclick={() => (currentView = "runs")}>
        Runs
      </button>
      <button class:active={currentView === "stats"} onclick={() => (currentView = "stats")}>
        Stats
      </button>
    </nav>

    <main>
      {#if currentView === "runs"}
        <p>Runs list coming soon.</p>
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
    background: #0f0f0f;
    color: #e0e0e0;
  }

  .layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  header {
    padding: 0.75rem 1.5rem;
    background: #1a1a1a;
    border-bottom: 1px solid #2a2a2a;
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
    background: #141414;
    border-right: 1px solid #2a2a2a;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  nav button {
    background: none;
    border: none;
    color: #999;
    padding: 0.5rem 0.75rem;
    text-align: left;
    border-radius: 6px;
    cursor: pointer;
    font-size: 0.9rem;
  }

  nav button:hover {
    background: #1f1f1f;
    color: #e0e0e0;
  }

  nav button.active {
    background: #252525;
    color: #fff;
  }

  main {
    flex: 1;
    padding: 1.5rem;
    overflow-y: auto;
  }
</style>
