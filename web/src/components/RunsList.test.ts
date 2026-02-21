import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/svelte";
import RunsList from "./RunsList.svelte";
import type { Run } from "../lib/api/client.js";

// Stub fetch for the runs module
vi.stubGlobal("fetch", vi.fn());

function makeRun(overrides: Partial<Run> = {}): Run {
  return {
    id: "test-1",
    plan_path: "plans/auth.md",
    status: "active",
    created_at: "2026-02-20T10:00:00Z",
    updated_at: "2026-02-20T10:05:00Z",
    steps: [
      { name: "read plan", status: "completed" },
      { name: "create issue", status: "running" },
      { name: "generate branch", status: "pending" },
    ],
    ...overrides,
  };
}

describe("RunsList", () => {
  beforeEach(async () => {
    // Reset module state
    const { getRuns, setStatusFilter } = await import("../lib/state/runs.svelte.js");
    getRuns().length = 0;
    setStatusFilter("all");
  });

  it("shows empty state when no runs", () => {
    render(RunsList);
    expect(screen.getByText("No runs found.")).toBeInTheDocument();
  });

  it("renders runs table when runs exist", async () => {
    const { upsert } = await import("../lib/state/runs.svelte.js");
    upsert(makeRun({ id: "r-1", plan_title: "Auth Feature" }));

    render(RunsList);
    expect(screen.getByText("Auth Feature")).toBeInTheDocument();
  });

  it("shows plan filename when no title", async () => {
    const { upsert } = await import("../lib/state/runs.svelte.js");
    upsert(makeRun({ id: "r-2", plan_path: "plans/deploy.md" }));

    render(RunsList);
    expect(screen.getByText("deploy")).toBeInTheDocument();
  });

  it("shows step progress count", async () => {
    const { upsert } = await import("../lib/state/runs.svelte.js");
    upsert(makeRun({ id: "r-3" }));

    render(RunsList);
    expect(screen.getByText("1/3")).toBeInTheDocument();
  });

  it("shows PR link when present", async () => {
    const { upsert } = await import("../lib/state/runs.svelte.js");
    upsert(makeRun({ id: "r-4", pr_url: "https://github.com/org/repo/pull/42", pr_number: 42 }));

    render(RunsList);
    const link = screen.getByText("#42");
    expect(link).toBeInTheDocument();
    expect(link.closest("a")).toHaveAttribute("href", "https://github.com/org/repo/pull/42");
  });

  it("renders filter buttons", () => {
    render(RunsList);
    expect(screen.getByText("All")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeInTheDocument();
  });
});
