import { upsert } from "./runs.svelte.js";
import type { Run } from "../api/client.js";

let connected = $state(false);
let eventSource: EventSource | null = null;

export function isConnected(): boolean {
  return connected;
}

export function connect() {
  if (eventSource) return;

  const es = new EventSource("/api/events");
  eventSource = es;

  es.onopen = () => {
    connected = true;
  };

  es.onmessage = (event) => {
    try {
      const run: Run = JSON.parse(event.data);
      upsert(run);
    } catch {
      // ignore malformed messages
    }
  };

  es.onerror = () => {
    connected = false;
    es.close();
    eventSource = null;
    // reconnect after 3s
    setTimeout(connect, 3000);
  };
}

export function disconnect() {
  if (eventSource) {
    eventSource.close();
    eventSource = null;
    connected = false;
  }
}
