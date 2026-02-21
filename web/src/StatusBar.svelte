<script lang="ts">
  let connected = $state(false);

  $effect(() => {
    const es = new EventSource("/api/events");

    es.onopen = () => {
      connected = true;
    };

    es.onerror = () => {
      connected = false;
    };

    return () => {
      es.close();
      connected = false;
    };
  });
</script>

<footer class="status-bar">
  <span class="indicator" class:connected></span>
  {connected ? "Connected" : "Disconnected"}
</footer>

<style>
  .status-bar {
    padding: 0.35rem 1rem;
    background: #1a1a1a;
    border-top: 1px solid #2a2a2a;
    font-size: 0.75rem;
    color: #888;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #ef4444;
  }

  .indicator.connected {
    background: #22c55e;
  }
</style>
