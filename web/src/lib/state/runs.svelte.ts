import { api, type Run, type RunStatus } from "../api/client.js";

let runs = $state<Run[]>([]);
let loading = $state(false);
let error = $state<string | null>(null);
let statusFilter = $state<RunStatus | "all">("all");

export function getRuns(): Run[] {
  return runs;
}

export function getFilteredRuns(): Run[] {
  if (statusFilter === "all") return runs;
  return runs.filter((r) => r.status === statusFilter);
}

export function getStatusFilter(): RunStatus | "all" {
  return statusFilter;
}

export function setStatusFilter(s: RunStatus | "all") {
  statusFilter = s;
}

export function isLoading(): boolean {
  return loading;
}

export function getError(): string | null {
  return error;
}

export function findRun(id: string): Run | undefined {
  return runs.find((r) => r.id === id);
}

export function upsert(run: Run) {
  const idx = runs.findIndex((r) => r.id === run.id);
  if (idx >= 0) {
    runs[idx] = run;
  } else {
    runs.unshift(run);
  }
}

export async function fetchRuns() {
  loading = true;
  error = null;
  const { data, error: err } = await api.GET("/runs");
  if (err) {
    error = "Failed to fetch runs";
    loading = false;
    return;
  }
  if (data) {
    runs = data.runs;
  }
  loading = false;
}
